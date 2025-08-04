package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"kliro/models"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const DEEPSEEK_API_URL = "https://api.deepseek.com/v1/chat/completions"

type AutocreditParser struct{}

func NewAutocreditParser() *AutocreditParser {
	return &AutocreditParser{}
}

func (ap *AutocreditParser) ParseURL(url string) (*models.Autocredit, error) {
	// Получаем HTML страницы
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения страницы: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга HTML: %v", err)
	}

	// Удаляем навигацию, футер и прочие неинформативные блоки
	doc.Find("nav, header, footer, .navbar, .menu, .sidebar, .breadcrumbs, .topbar, .language, .lang-switcher, .mobile-menu, .contact-info").Remove()

	// Удаляем скрипты и стили
	doc.Find("script, style").Remove()

	// Пытаемся вытащить только релевантные блоки с ключевыми словами
	var relevantText []string
	doc.Find("section, div, p, span, li, td").Each(func(i int, s *goquery.Selection) {
		txt := strings.ToLower(s.Text())
		if strings.Contains(txt, "авто") || strings.Contains(txt, "машина") || strings.Contains(txt, "автомобиль") || strings.Contains(txt, "oy") || strings.Contains(txt, "foiz") || strings.Contains(txt, "so'm") || strings.Contains(txt, "%") || strings.Contains(txt, "первоначальный") || strings.Contains(txt, "взнос") || strings.Contains(txt, "кредит") || strings.Contains(txt, "ставка") || strings.Contains(txt, "срок") {
			relevantText = append(relevantText, s.Text())
		}
	})

	var text string
	if len(relevantText) > 0 {
		text = strings.Join(relevantText, " ")
	} else {
		text = doc.Find("body").Text()
	}

	text = ap.cleanText(text)

	// Логируем очищенный текст для отладки
	if len(text) > 2000 {
		log.Printf("[AUTOCREDIT PARSER] Очищенный текст для %s (первые 2000 символов): %s", url, text[:2000])
	} else {
		log.Printf("[AUTOCREDIT PARSER] Очищенный текст для %s: %s", url, text)
	}

	prompt := fmt.Sprintf(`
	Ты профессиональный парсер данных автокредитов. Извлеки из текста данные и верни строго один JSON следующего формата:
	
	{
	  "bank_name": string|null,               // Название банка, извлеки из URL (например, "hamkorbank.uz" → "hamkorbank", "xb.uz" → "xb"). Если невозможно — null
	  "url": string,                          // Исходный URL
	  "rate": number,                         // Минимальная процентная ставка (в процентах). Примеры: "от 24.9" → 24.9, "до 28" → 28, "24" → 24. Если не найдено — 0
	  "initial_payment": number,             // Первоначальный взнос в процентах. Примеры: "от 25" → 25, "25" → 25. Если не найдено — 0
	  "term_months": number,                 // Максимальный срок в месяцах. Примеры: "до 5 лет" → 60, "48 месяцев" → 48, "4 года" → 48. Если не найдено — 0
	  "max_amount": string|number|null       // Максимальная сумма кредита:
											 // - Если указано "до 600 млн сум" → 600000000
											 // - Если указано "до 2000 БРВ" → "2000 BRV"
											 // - Если указано "3x минимальной зарплаты", "МРЗП", "минимальная зарплата", "minimal wage" → "3x минимальная зарплата" и т.д.
											 // - Если "не ограничено", "VIP", "чекланмаган" → "VIP"
											 // - Если не указано — null
	}
	
	📌 Учитывай обе языковые версии: русский и узбекский.
	📌 Извлекай только автокредит — ищи слова: автокредит, автомобильный кредит, avtokredit, avtomobil, mashina, bosh to'lov, foiz, oy muddati и т.д.
	
	Примеры фраз для max_amount:
	- “до 2000 БРВ” → "2000 BRV"
	- “до 3 минимальных зарплат” → "3x минимальная зарплата"
	- “до 600 млн сум” → 600000000
	- “без ограничений”, “не ограничено”, “VIP”, “чекланмаган” → "VIP"
	
	Примеры для bank_name:
	- "https://www.infinbank.com/ru/private/credits/avto_credit/" → "infinbank"
	- "https://xb.uz/page/tezkor-avtokredit" → "xb"
	
	TEXT: """%s"""
	URL: "%s"
	
	Верни только JSON. Без пояснений, без текста до и после.
	`, text, url)

	// Вызываем DeepSeek API
	reqBody := DeepSeekRequest{
		Model:       "deepseek-chat",
		Messages:    []Message{{Role: "user", Content: prompt}},
		MaxTokens:   256,
		Temperature: 0.0,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", DEEPSEEK_API_URL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("DeepSeek API key not set")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	dsResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка вызова DeepSeek API: %v", err)
	}
	defer dsResp.Body.Close()

	body, _ := ioutil.ReadAll(dsResp.Body)
	var deepSeekResponse DeepSeekResponse
	if err := json.Unmarshal(body, &deepSeekResponse); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа DeepSeek: %v", err)
	}

	if len(deepSeekResponse.Choices) > 0 {
		raw := deepSeekResponse.Choices[0].Message.Content
		raw = strings.TrimSpace(raw)
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)

		log.Printf("[AUTOCREDIT PARSER] DeepSeek ответ для %s: %s", url, raw)

		// Сначала парсим в промежуточную структуру с правильными типами
		var tempResponse struct {
			BankName       string      `json:"bank_name"`
			URL            string      `json:"url"`
			Rate           float64     `json:"rate"`
			InitialPayment float64     `json:"initial_payment"`
			TermMonths     int         `json:"term_months"`
			MaxAmount      interface{} `json:"max_amount"`
		}

		if err := json.Unmarshal([]byte(raw), &tempResponse); err != nil {
			log.Printf("[AUTOCREDIT PARSER ERROR] Ошибка парсинга JSON для %s: %v, raw: %s", url, err, raw)
			return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
		}

		// Конвертируем max_amount в строку
		var maxAmountStr string
		switch v := tempResponse.MaxAmount.(type) {
		case string:
			maxAmountStr = v
		case float64:
			if v == 0 {
				maxAmountStr = "VIP"
			} else {
				maxAmountStr = fmt.Sprintf("%.0f", v)
			}
		case int:
			if v == 0 {
				maxAmountStr = "VIP"
			} else {
				maxAmountStr = fmt.Sprintf("%d", v)
			}
		default:
			maxAmountStr = "VIP"
		}

		// Создаем финальную структуру
		parsedCredit := models.Autocredit{
			BankName:       tempResponse.BankName,
			URL:            tempResponse.URL,
			Rate:           tempResponse.Rate,
			InitialPayment: tempResponse.InitialPayment,
			TermMonths:     tempResponse.TermMonths,
			MaxAmount:      maxAmountStr,
			CreatedAt:      time.Now(),
		}

		log.Printf("[AUTOCREDIT PARSER] Успешно спарсили для %s: bank=%s, rate=%.2f, initial_payment=%.2f, term=%d, max_amount=%s",
			url, parsedCredit.BankName, parsedCredit.Rate, parsedCredit.InitialPayment, parsedCredit.TermMonths, parsedCredit.MaxAmount)

		return &parsedCredit, nil
	}

	return nil, fmt.Errorf("нет ответа от DeepSeek")
}

func (ap *AutocreditParser) cleanText(raw string) string {
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

	if len(clean) > 5000 {
		clean = clean[:5000]
	}
	return clean
}
