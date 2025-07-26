package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type DeepSeekRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type CurrencyParser struct {
	currencyService *CurrencyService
}

func NewCurrencyParser(currencyService *CurrencyService) *CurrencyParser {
	return &CurrencyParser{
		currencyService: currencyService,
	}
}

// ParseAndSaveCurrencyRates парсит и сохраняет курсы валют
func (cp *CurrencyParser) ParseAndSaveCurrencyRates() error {
	log.Printf("[CURRENCY PARSER] Начинаем парсинг курсов валют...")

	url := "https://bank.uz/uz/currency"

	// Получаем HTML страницы
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[CURRENCY PARSER ERROR] Ошибка получения страницы: %v", err)
		return err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("[CURRENCY PARSER ERROR] Ошибка парсинга HTML: %v", err)
		return err
	}

	// Удаляем навигацию, футер и прочие неинформативные блоки
	doc.Find("nav, header, footer, .navbar, .menu, .sidebar, .breadcrumbs, .topbar, .language, .lang-switcher, .mobile-menu, .contact-info").Remove()

	// Удаляем скрипты и стили
	doc.Find("script, style").Remove()

	// Пытаемся вытащить только релевантные блоки с ключевыми словами для валюты
	var relevantText []string
	doc.Find("section, div, table, tbody, tr, td").Each(func(i int, s *goquery.Selection) {
		txt := strings.ToLower(s.Text())
		if strings.Contains(txt, "валют") || strings.Contains(txt, "usd") || strings.Contains(txt, "eur") || strings.Contains(txt, "rub") || strings.Contains(txt, "курс") || strings.Contains(txt, "exchange") {
			relevantText = append(relevantText, s.Text())
		}
	})

	var text string
	if len(relevantText) > 0 {
		text = strings.Join(relevantText, " ")
	} else {
		text = doc.Find("body").Text()
	}

	text = cp.cleanText(text)

	log.Printf("[CURRENCY PARSER] Очищенный текст для DeepSeek (первые 7000 символов):")
	log.Print(text)

	prompt := fmt.Sprintf(`Найди таблицу курсов валют после заголовка "Valyuta almashtirish shahobchalaridagi eng yahshi kurslar".
Извлеки только курсы USD, RUB, KZT, EUR. Игнорируй всё после таблицы курсов.

Верни ТОЧНО такой JSON массив:
[
  {
    "bank": "название банка",
    "currency": "USD",
    "buy": 12560,
    "sell": 12655
  },
  {
    "bank": "название банка", 
    "currency": "RUB",
    "buy": 140,
    "sell": 165
  }
]

Правила:
- currency может быть только: USD, RUB, KZT, EUR
- buy и sell должны быть числами (не строками)
- если sell не указан, используй null
- если банк не найден, используй "Unknown Bank"

Текст: %s
Только JSON массив.`, text)

	reqBody := DeepSeekRequest{
		Model:       "deepseek-chat",
		Messages:    []Message{{Role: "user", Content: prompt}},
		MaxTokens:   4096,
		Temperature: 0.0,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", DEEPSEEK_API_URL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Printf("[CURRENCY PARSER ERROR] DeepSeek API key not set")
		return fmt.Errorf("DeepSeek API key not set")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	dsResp, err := client.Do(req)
	if err != nil {
		log.Printf("[CURRENCY PARSER ERROR] Ошибка вызова DeepSeek API: %v", err)
		return err
	}
	defer dsResp.Body.Close()

	body, _ := ioutil.ReadAll(dsResp.Body)
	var deepSeekResponse DeepSeekResponse
	if err := json.Unmarshal(body, &deepSeekResponse); err != nil {
		log.Printf("[CURRENCY PARSER ERROR] Ошибка парсинга ответа DeepSeek: %v", err)
		return err
	}

	if len(deepSeekResponse.Choices) > 0 {
		raw := deepSeekResponse.Choices[0].Message.Content
		raw = strings.TrimSpace(raw)
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)

		var ratesArray []map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &ratesArray); err != nil {
			log.Printf("[CURRENCY PARSER ERROR] Ошибка парсинга JSON: %v", err)
			log.Printf("[CURRENCY PARSER ERROR] Raw response: %s", raw)
			return err
		}

		// Сохраняем курсы в БД
		if err := cp.currencyService.SaveCurrencyRates(ratesArray); err != nil {
			log.Printf("[CURRENCY PARSER ERROR] Ошибка сохранения курсов: %v", err)
			return err
		}

		log.Printf("[CURRENCY PARSER] Успешно обработано %d курсов валют", len(ratesArray))
	} else {
		log.Printf("[CURRENCY PARSER ERROR] Нет ответа от DeepSeek")
		return fmt.Errorf("no response from DeepSeek")
	}

	return nil
}

// Очистка текста от ссылок, HTML и мусора
func (cp *CurrencyParser) cleanText(raw string) string {
	// Удаляем скрипты и стили
	reScript := regexp.MustCompile(`<script[^>]*>.*?</script>`)
	reStyle := regexp.MustCompile(`<style[^>]*>.*?</style>`)
	reLink := regexp.MustCompile(`https?://\S+|ftp://\S+|mailto:\S+`)
	reTag := regexp.MustCompile(`<[^>]+>`)
	reSpaces := regexp.MustCompile(`\s+`)
	reJS := regexp.MustCompile(`javascript:`)
	reConsole := regexp.MustCompile(`console\.(log|error|warn|info)\([^)]*\)`)
	reFunction := regexp.MustCompile(`function\s+\w+\s*\([^)]*\)\s*\{[^}]*\}`)

	// Удаляем скрипты и стили
	clean := reScript.ReplaceAllString(raw, "")
	clean = reStyle.ReplaceAllString(clean, "")

	// Удаляем ссылки
	clean = reLink.ReplaceAllString(clean, "")

	// Удаляем HTML теги
	clean = reTag.ReplaceAllString(clean, "")

	// Удаляем JavaScript код
	clean = reJS.ReplaceAllString(clean, "")
	clean = reConsole.ReplaceAllString(clean, "")
	clean = reFunction.ReplaceAllString(clean, "")

	lines := strings.Split(clean, "\n")
	var compact []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && len(line) > 2 {
			compact = append(compact, line)
		}
	}

	clean = strings.Join(compact, " ")
	clean = reSpaces.ReplaceAllString(clean, " ")
	clean = strings.TrimSpace(clean)

	if len(clean) > 7000 {
		clean = clean[:7000]
	}
	return clean
}
