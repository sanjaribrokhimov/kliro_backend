package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"kliro/services"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-gonic/gin"
)

const DEEPSEEK_API_URL = "https://api.deepseek.com/v1/chat/completions"

// https://tengebank.uz/credit/mikrozajm-onlajn
// https://ru.ipakyulibank.uz/physical/kredity/mikrozaymy
// https://tbcbank.uz/ru/product/kredity/
// https://aloqabank.uz/uz/private/crediting/onlayn-mikroqarz/
// https://mkbank.uz/uz/private/crediting/microloan/
// https://xb.uz/page/onlayn-mikroqarz
// https://turonbank.uz/ru/private/crediting/mikrokredit-dlya-samozanyatykh-lits/
// https://hamkorbank.uz/physical/credits/microloan-online/
// https://sqb.uz/uz/individuals/credits/mikrozaym-ru/
// https://www.ipotekabank.uz/private/crediting/micro_new/

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

type ParserController struct {
	currencyService *services.CurrencyService
}

func NewParserController(currencyService *services.CurrencyService) *ParserController {
	return &ParserController{
		currencyService: currencyService,
	}
}

func (pc *ParserController) ParsePage(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	resp, err := http.Get(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch url: %v", err)})
		return
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse HTML: %v", err)})
		return
	}

	// Удаляем навигацию, футер и прочие неинформативные блоки
	doc.Find("nav, header, footer, .navbar, .menu, .sidebar, .breadcrumbs, .topbar, .language, .lang-switcher, .mobile-menu, .contact-info").Remove()

	// Удаляем скрипты и стили
	doc.Find("script, style").Remove()

	// Пытаемся вытащить только релевантные блоки с ключевыми словами
	var relevantText []string
	doc.Find("section, div").Each(func(i int, s *goquery.Selection) {
		txt := strings.ToLower(s.Text())
		if strings.Contains(txt, "микро") || strings.Contains(txt, "oy") || strings.Contains(txt, "foiz") || strings.Contains(txt, "so'm") || strings.Contains(txt, "%") {
			relevantText = append(relevantText, s.Text())
		}
	})

	var text string
	if len(relevantText) > 0 {
		text = strings.Join(relevantText, " ")
	} else {
		text = doc.Find("body").Text()
	}

	text = cleanText(text)

	fmt.Println("[PARSE] Очищенный текст для DeepSeek (первые 5000 символов):")
	fmt.Println(text)

	prompt := fmt.Sprintf(`Извлеки информацию о микрокредите из текста и верни JSON-объект со следующими полями:

bank_name: название банка, извлеки из URL (например, если URL — "https://www.ipoteka.uz/credits", то bank_name = "ipoteka"; если невозможно определить — null)
url: оригинальный URL
max_amount: максимальная сумма кредита (только число, если не указана — null)
term_months: срок кредита в месяцах (только число, если не указано — null). Если срок указан в годах (например, «до 3 лет»), обязательно переведи в месяцы (например, 3 года = 36 месяцев). Если указано "до N месяцев", "срок до N месяцев" или "до N мес.", обязательно извлеки это как максимальный срок и запиши как число. Например, "до 36 месяцев" → term_months: 36
rate_min: минимальная процентная ставка (только число, если указано, иначе — null). Если указано от X", то rate_min = X. Если указан только процент без от или до (например, 24), то rate_min = X, rate_max = null
rate_max: максимальная процентная ставка (только число, если указано, иначе — null). Если указано до Y", то rate_max = Y

Важно: извлекай данные как с русскоязычных, так и с узбекоязычных сайтов.
Учитывай следующие слова и их значение:
foiz — процентная ставка
dan — от (для rate_min)
gacha — до (для rate_max, max_amount, term_months)
oy, oygacha, oy muddati — срок в месяцах (например: 60 oygacha → term_months: 60)
so'm, so'mgacha, miqdori — сумма кредита
mikroqarz, onlayn kredit — микрокредит
kredit muddati — срок кредита
kredit miqdori — сумма кредита

Если сумма или срок указаны диапазоном (например: 12-60 oy), выдели только максимальное значение.

Правила для процентных ставок:
- Если указано "от X до Y" → rate_min = X, rate_max = Y
- Если указано только "от X" → rate_min = X, rate_max = null
- Если указано только "до Y" → rate_min = null, rate_max = Y
- Если указан только процент без "от" или "до" (например, "24", "24 годовых") → rate_min = 24, rate_max = null

Обязательно:
Если на странице указано несколько видов кредитов, извлекай только микрокредит.
Если не указано слово "микрокредит" или "онлайн-кредит", всё равно извлекай данные только по одному (любому) кредиту.

Текст: "%s"
URL: "%s"
Верни только JSON. Без пояснений. Если какое-то значение не найдено — укажи null.`, text, url)

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
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "DeepSeek API key not set"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	dsResp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to call DeepSeek API: %v", err)})
		return
	}
	defer dsResp.Body.Close()

	body, _ := ioutil.ReadAll(dsResp.Body)
	var deepSeekResponse DeepSeekResponse
	if err := json.Unmarshal(body, &deepSeekResponse); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse DeepSeek response"})
		return
	}

	if len(deepSeekResponse.Choices) > 0 {
		raw := deepSeekResponse.Choices[0].Message.Content
		raw = strings.TrimSpace(raw)
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse final JSON", "raw": raw})
			return
		}
		c.JSON(http.StatusOK, gin.H{"result": result, "success": true})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "No response from DeepSeek"})
	}
}

// Новый парсер для валюты
func (pc *ParserController) ParseCurrencyPage(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	resp, err := http.Get(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch url: %v", err)})
		return
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse HTML: %v", err)})
		return
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

	text = cleanText(text)

	fmt.Println("[PARSE CURRENCY] Очищенный текст для DeepSeek (первые 5000 символов):")
	fmt.Println(text)

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
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "DeepSeek API key not set"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	dsResp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to call DeepSeek API: %v", err)})
		return
	}
	defer dsResp.Body.Close()

	body, _ := ioutil.ReadAll(dsResp.Body)
	var deepSeekResponse DeepSeekResponse
	if err := json.Unmarshal(body, &deepSeekResponse); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse DeepSeek response"})
		return
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
			fmt.Printf("[PARSE CURRENCY ERROR] Failed to parse JSON: %v\n", err)
			fmt.Printf("[PARSE CURRENCY ERROR] Raw response: %s\n", raw)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse final JSON", "raw": raw})
			return
		}

		// Группируем по валютам
		usdRates := []map[string]interface{}{}
		rubRates := []map[string]interface{}{}
		kztRates := []map[string]interface{}{}
		eurRates := []map[string]interface{}{}

		for _, rate := range ratesArray {
			currency, ok := rate["currency"].(string)
			if !ok {
				continue
			}

			switch currency {
			case "USD":
				usdRates = append(usdRates, rate)
			case "RUB":
				rubRates = append(rubRates, rate)
			case "KZT":
				kztRates = append(kztRates, rate)
			case "EUR":
				eurRates = append(eurRates, rate)
			}
		}

		// Сохраняем курсы в БД
		if err := pc.currencyService.SaveCurrencyRates(ratesArray); err != nil {
			log.Printf("[PARSE CURRENCY ERROR] Ошибка сохранения курсов: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save currency rates"})
			return
		}

		result := map[string]interface{}{
			"USD": usdRates,
			"RUB": rubRates,
			"KZT": kztRates,
			"EUR": eurRates,
		}
		c.JSON(http.StatusOK, gin.H{"result": result, "success": true})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "No response from DeepSeek"})
	}
}

// Очистка текста от ссылок, HTML и мусора
func cleanText(raw string) string {
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

func (pc *ParserController) ParseAutocreditPage(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	// Используем autocredit parser
	parser := services.NewAutocreditParser()
	credit, err := parser.ParseURL(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse autocredit: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": credit, "success": true})
}
