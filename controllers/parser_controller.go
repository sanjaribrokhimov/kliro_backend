package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"kliro/models"
	"kliro/services"
	"kliro/utils"
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

	// Читаем HTML
	bodyBytes := make([]byte, 0)
	buffer := make([]byte, 1024)

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			bodyBytes = append(bodyBytes, buffer[:n]...)
		}
		if err != nil {
			break
		}
	}

	html := string(bodyBytes)

	// Создаем парсер и парсим курсы
	parser := services.NewCurrencyParser(pc.currencyService)
	rates, err := parser.ParseCurrencyRates(html)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse currency rates: %v", err)})
		return
	}

	// Группируем по валютам
	usdRates := []map[string]interface{}{}
	rubRates := []map[string]interface{}{}
	kztRates := []map[string]interface{}{}
	eurRates := []map[string]interface{}{}

	for _, rate := range rates {
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
	if err := pc.currencyService.SaveCurrencyRates(rates); err != nil {
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

func (pc *ParserController) ParseTransferPage(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	log.Printf("[PARSER CONTROLLER] 🚀 Начинаем парсинг переводов для URL: %s", url)

	// Используем transfer parser
	parser := services.NewTransferParser()
	transfers, err := parser.ParseURL(url)
	if err != nil {
		log.Printf("[PARSER CONTROLLER] ❌ Ошибка парсинга: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse transfer: %v", err)})
		return
	}

	log.Printf("[PARSER CONTROLLER] ✅ Парсинг завершен. Получено %d переводов", len(transfers))
	for i, transfer := range transfers {
		log.Printf("[PARSER CONTROLLER] 📋 %d. %s - %s", i+1, transfer.AppName, transfer.Commission)
	}

	// Сохраняем в базу данных
	db := utils.GetDB()
	savedCount := 0
	for _, transfer := range transfers {
		if err := db.Table("new_transfer").Create(transfer).Error; err != nil {
			log.Printf("[PARSER CONTROLLER] ❌ Ошибка сохранения %s: %v", transfer.AppName, err)
		} else {
			log.Printf("[PARSER CONTROLLER] ✅ Сохранен: %s", transfer.AppName)
			savedCount++
		}
	}
	log.Printf("[PARSER CONTROLLER] 📊 Сохранено %d/%d переводов", savedCount, len(transfers))

	c.JSON(http.StatusOK, gin.H{"result": transfers, "success": true, "saved": savedCount})
}

// ParseTransferAndUpdateDatabase парсит переводы и обновляет базу данных
func (pc *ParserController) ParseTransferAndUpdateDatabase(c *gin.Context) {
	log.Printf("[PARSER CONTROLLER] 🚀 Начинаем полный парсинг и обновление переводов")

	// Очищаем старые данные
	db := utils.GetDB()
	if err := db.Where("1 = 1").Delete(&models.Transfer{}).Error; err != nil {
		log.Printf("[PARSER CONTROLLER] ❌ Ошибка очистки базы: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear database"})
		return
	}

	// Парсим переводы
	parser := services.NewTransferParser()
	transfers, err := parser.ParseURL("https://bank.uz/perevodi")
	if err != nil {
		log.Printf("[PARSER CONTROLLER] ❌ Ошибка парсинга: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse transfers: %v", err)})
		return
	}

	// Сохраняем новые данные
	savedCount := 0
	for _, transfer := range transfers {
		if err := db.Table("new_transfer").Create(map[string]interface{}{
			"app_name":   transfer.AppName,
			"commission": transfer.Commission,
			"limit_ru":   transfer.LimitRU,
			"limit_uz":   transfer.LimitUZ,
			"created_at": transfer.CreatedAt,
		}).Error; err != nil {
			log.Printf("[PARSER CONTROLLER] ❌ Ошибка сохранения %s: %v", transfer.AppName, err)
		} else {
			log.Printf("[PARSER CONTROLLER] ✅ Сохранен: %s", transfer.AppName)
			savedCount++
		}
	}

	log.Printf("[PARSER CONTROLLER] 📊 Обновление завершено: %d/%d переводов", savedCount, len(transfers))

	c.JSON(http.StatusOK, gin.H{
		"result":  transfers,
		"success": true,
		"saved":   savedCount,
		"message": "База данных успешно обновлена",
	})
}

func (pc *ParserController) ParseMortgagePage(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	log.Printf("[PARSER CONTROLLER] 🏠 Начинаем парсинг ипотеки для URL: %s", url)

	// Используем mortgage parser
	parser := services.NewMortgageParser()
	mortgage, err := parser.ParseURL(url)
	if err != nil {
		log.Printf("[PARSER CONTROLLER] ❌ Ошибка парсинга ипотеки: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse mortgage: %v", err)})
		return
	}

	log.Printf("[PARSER CONTROLLER] ✅ Ипотека спарсена: %s (%.1f%%)", mortgage.BankName, mortgage.Rate)

	c.JSON(http.StatusOK, gin.H{
		"result":  mortgage,
		"success": true,
	})
}

func (pc *ParserController) ParseDepositPage(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	log.Printf("[PARSER CONTROLLER] 💰 Начинаем парсинг вкладов для URL: %s", url)

	// Используем deposit parser
	parser := services.NewDepositParser()
	deposits, err := parser.ParseURL(url)
	if err != nil {
		log.Printf("[PARSER CONTROLLER] ❌ Ошибка парсинга вкладов: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse deposits: %v", err)})
		return
	}

	log.Printf("[PARSER CONTROLLER] ✅ Спарсено вкладов: %d", len(deposits))

	c.JSON(http.StatusOK, gin.H{
		"result":  deposits,
		"success": true,
	})
}

func (pc *ParserController) ParseCardPage(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	log.Printf("[PARSER CONTROLLER] 💳 Начинаем парсинг карт для URL: %s", url)

	// Используем card parser
	parser := services.NewCardParser()
	cards, err := parser.ParseURL(url)
	if err != nil {
		log.Printf("[PARSER CONTROLLER] ❌ Ошибка парсинга карт: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse cards: %v", err)})
		return
	}

	log.Printf("[PARSER CONTROLLER] ✅ Спарсено карт: %d", len(cards))

	c.JSON(http.StatusOK, gin.H{
		"result":  cards,
		"success": true,
	})
}

func (pc *ParserController) ParseMicrocreditPage(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter is required"})
		return
	}

	log.Printf("[PARSER CONTROLLER] 💰 Начинаем парсинг микрокредитов для URL: %s", url)

	// Используем microcredit parser
	parser := services.NewMicrocreditParser()
	microcredits, err := parser.ParseURL(url)
	if err != nil {
		log.Printf("[PARSER CONTROLLER] ❌ Ошибка парсинга микрокредитов: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse microcredits: %v", err)})
		return
	}

	log.Printf("[PARSER CONTROLLER] ✅ Спарсено микрокредитов: %d", len(microcredits))

	c.JSON(http.StatusOK, gin.H{
		"result":  microcredits,
		"success": true,
	})
}
