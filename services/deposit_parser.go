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

type DepositParser struct{}

func NewDepositParser() *DepositParser {
	return &DepositParser{}
}

func (dp *DepositParser) ParseURL(url string) (*models.Deposit, error) {
	log.Printf("[DEPOSIT PARSER] 🚀 Начинаем парсинг URL: %s", url)

	// Получаем HTML страницы
	log.Printf("[DEPOSIT PARSER] 🌐 Загружаем страницу...")
	httpClient := &http.Client{
		Timeout: 15 * time.Second,
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		log.Printf("[DEPOSIT PARSER] ❌ Ошибка получения страницы: %v", err)
		return nil, fmt.Errorf("ошибка получения страницы: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("[DEPOSIT PARSER] 📡 Статус страницы: %d", resp.StatusCode)

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("[DEPOSIT PARSER] ❌ Ошибка парсинга HTML: %v", err)
		return nil, fmt.Errorf("ошибка парсинга HTML: %v", err)
	}

	// Берем весь текст со страницы
	text := doc.Find("body").Text()
	log.Printf("[DEPOSIT PARSER] 📄 Исходный текст: %d символов", len(text))

	// Очищаем текст от лишнего
	text = dp.cleanText(text)

	log.Printf("[DEPOSIT PARSER] Очищенный текст для %s (первые 5000 символов):", url)
	log.Printf(text[:min(len(text), 5000)])
	log.Printf("[DEPOSIT PARSER] 📏 Общая длина текста: %d символов", len(text))

	// Промпт для DeepSeek
	prompt := fmt.Sprintf(`Извлеки информацию о банковском вкладе из текста и верни JSON объект.

Объект должен содержать:
bank_name: название банка
rate: процентная ставка (число, например 15.5)
term_months: срок вклада в месяцах (число, например 12)
min_amount: минимальная сумма вклада (число в миллионах, например 1)

Если какое-то значение не найдено, используй null.

Текст: "%s"
Верни только JSON объект.`, text)

	// Вызываем DeepSeek API
	reqBody := DeepSeekRequest{
		Model:       "deepseek-chat",
		Messages:    []Message{{Role: "user", Content: prompt}},
		MaxTokens:   4096,
		Temperature: 0.0,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Printf("[DEPOSIT PARSER] ❌ DEEPSEEK_API_KEY не установлен")
		return nil, fmt.Errorf("DeepSeek API key not set")
	}
	log.Printf("[DEPOSIT PARSER] ✅ DEEPSEEK_API_KEY найден (длина: %d)", len(apiKey))

	req.Header.Set("Authorization", "Bearer "+apiKey)

	log.Printf("[DEPOSIT PARSER] 🌐 Отправляем запрос к DeepSeek API...")
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	dsResp, err := client.Do(req)
	if err != nil {
		log.Printf("[DEPOSIT PARSER] ❌ Ошибка HTTP запроса к DeepSeek: %v", err)
		return nil, fmt.Errorf("ошибка вызова DeepSeek API: %v", err)
	}
	defer dsResp.Body.Close()

	log.Printf("[DEPOSIT PARSER] 📡 Статус ответа DeepSeek: %d", dsResp.StatusCode)

	body, _ := ioutil.ReadAll(dsResp.Body)
	log.Printf("[DEPOSIT PARSER] 📄 Размер ответа DeepSeek: %d байт", len(body))

	if dsResp.StatusCode != 200 {
		log.Printf("[DEPOSIT PARSER] ❌ Ошибка DeepSeek API (статус %d): %s", dsResp.StatusCode, string(body))
		return nil, fmt.Errorf("DeepSeek API error (status %d): %s", dsResp.StatusCode, string(body))
	}

	var deepSeekResponse DeepSeekResponse
	if err := json.Unmarshal(body, &deepSeekResponse); err != nil {
		log.Printf("[DEPOSIT PARSER] ❌ Ошибка парсинга JSON ответа DeepSeek: %v", err)
		log.Printf("[DEPOSIT PARSER] 📄 Сырой ответ: %s", string(body))
		return nil, fmt.Errorf("ошибка парсинга ответа DeepSeek: %v", err)
	}

	log.Printf("[DEPOSIT PARSER] 📊 Количество choices в ответе: %d", len(deepSeekResponse.Choices))

	if len(deepSeekResponse.Choices) > 0 {
		raw := deepSeekResponse.Choices[0].Message.Content
		raw = strings.TrimSpace(raw)
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)

		log.Printf("[DEPOSIT PARSER] DeepSeek ответ для %s: %s", url, raw)
		log.Printf("[DEPOSIT PARSER] 📄 Длина ответа DeepSeek: %d символов", len(raw))

		var deposit models.Deposit
		if err := json.Unmarshal([]byte(raw), &deposit); err != nil {
			log.Printf("[DEPOSIT PARSER ERROR] Ошибка парсинга JSON для %s: %v, raw: %s", url, err, raw)
			return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
		}

		// Устанавливаем время создания и URL
		deposit.CreatedAt = time.Now()
		deposit.URL = url

		// Улучшаем данные
		dp.improveDepositData(&deposit)

		log.Printf("[DEPOSIT PARSER] ✅ Успешно спарсили вклад: %s (ставка: %.1f%%)", deposit.BankName, deposit.Rate)

		return &deposit, nil
	}

	return nil, fmt.Errorf("нет ответа от DeepSeek")
}

func (dp *DepositParser) cleanText(raw string) string {
	// Удаляем HTML теги
	reTag := regexp.MustCompile(`<[^>]+>`)
	clean := reTag.ReplaceAllString(raw, "")

	// Удаляем скрипты и стили
	reScript := regexp.MustCompile(`<script[^>]*>.*?</script>`)
	reStyle := regexp.MustCompile(`<style[^>]*>.*?</style>`)
	clean = reScript.ReplaceAllString(clean, "")
	clean = reStyle.ReplaceAllString(clean, "")

	// Удаляем лишние пробелы и переносы строк
	reSpaces := regexp.MustCompile(`\s+`)
	clean = reSpaces.ReplaceAllString(clean, " ")
	clean = strings.TrimSpace(clean)

	// Ограничиваем длину
	if len(clean) > 8000 {
		clean = clean[:8000]
	}

	return clean
}

func (dp *DepositParser) improveDepositData(deposit *models.Deposit) {
	// Улучшаем название банка
	deposit.BankName = dp.cleanBankName(deposit.BankName)

	// Проверяем разумные значения
	if deposit.Rate < 0 {
		deposit.Rate = 0
	}
	if deposit.Rate > 50 {
		deposit.Rate = 50
	}
	if deposit.TermMonths < 1 {
		deposit.TermMonths = 1
	}
	if deposit.TermMonths > 120 {
		deposit.TermMonths = 120
	}
	if deposit.MinAmount < 0 {
		deposit.MinAmount = 0
	}
}

func (dp *DepositParser) cleanBankName(name string) string {
	// Убираем лишние пробелы
	cleaned := strings.TrimSpace(name)

	// Исправляем известные названия
	nameMap := map[string]string{
		"Anorbank":    "Anor Bank",
		"Asakabank":   "Asaka Bank",
		"Hamkorbank":  "Hamkor Bank",
		"Ipotekabank": "Ipoteka Bank",
		"Milliybank":  "Milliy Bank",
		"Sqbbank":     "SQB Bank",
		"Turonbank":   "Turon Bank",
		"Xalqbank":    "Xalq Bank",
		"Agrobank":    "Agro Bank",
		"Aloqabank":   "Aloqa Bank",
	}

	if corrected, exists := nameMap[cleaned]; exists {
		return corrected
	}

	return cleaned
}
