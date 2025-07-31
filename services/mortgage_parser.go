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

type MortgageParser struct{}

func NewMortgageParser() *MortgageParser {
	return &MortgageParser{}
}

func (mp *MortgageParser) ParseURL(url string) (*models.Mortgage, error) {
	log.Printf("[MORTGAGE PARSER] 🚀 Начинаем парсинг URL: %s", url)

	// Получаем HTML страницы
	log.Printf("[MORTGAGE PARSER] 🌐 Загружаем страницу...")
	httpClient := &http.Client{
		Timeout: 15 * time.Second,
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		log.Printf("[MORTGAGE PARSER] ❌ Ошибка получения страницы: %v", err)
		return nil, fmt.Errorf("ошибка получения страницы: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("[MORTGAGE PARSER] 📡 Статус страницы: %d", resp.StatusCode)

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("[MORTGAGE PARSER] ❌ Ошибка парсинга HTML: %v", err)
		return nil, fmt.Errorf("ошибка парсинга HTML: %v", err)
	}

	// Берем весь текст со страницы
	text := doc.Find("body").Text()
	log.Printf("[MORTGAGE PARSER] 📄 Исходный текст: %d символов", len(text))

	// Очищаем текст от лишнего
	text = mp.cleanText(text)

	log.Printf("[MORTGAGE PARSER] Очищенный текст для %s (первые 5000 символов):", url)
	log.Printf(text[:min(len(text), 5000)])
	log.Printf("[MORTGAGE PARSER] 📏 Общая длина текста: %d символов", len(text))

	// Промпт для DeepSeek
	prompt := fmt.Sprintf(`Извлеки информацию об ипотечном кредите из текста и верни JSON объект.

Объект должен содержать:
bank_name: название банка
rate_max: максимальная процентная ставка (число, например 15.5)
rate_min: минимальная процентная ставка (число, например 12.0)
term_years: срок кредита в годах (число, например 20)
max_amount: максимальная сумма кредита (число в миллионах, например 500)
initial_payment: первоначальный взнос в процентах (число, например 20)

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
		log.Printf("[MORTGAGE PARSER] ❌ DEEPSEEK_API_KEY не установлен")
		return nil, fmt.Errorf("DeepSeek API key not set")
	}
	log.Printf("[MORTGAGE PARSER] ✅ DEEPSEEK_API_KEY найден (длина: %d)", len(apiKey))

	req.Header.Set("Authorization", "Bearer "+apiKey)

	log.Printf("[MORTGAGE PARSER] 🌐 Отправляем запрос к DeepSeek API...")
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	dsResp, err := client.Do(req)
	if err != nil {
		log.Printf("[MORTGAGE PARSER] ❌ Ошибка HTTP запроса к DeepSeek: %v", err)
		return nil, fmt.Errorf("ошибка вызова DeepSeek API: %v", err)
	}
	defer dsResp.Body.Close()

	log.Printf("[MORTGAGE PARSER] 📡 Статус ответа DeepSeek: %d", dsResp.StatusCode)

	body, _ := ioutil.ReadAll(dsResp.Body)
	log.Printf("[MORTGAGE PARSER] 📄 Размер ответа DeepSeek: %d байт", len(body))

	if dsResp.StatusCode != 200 {
		log.Printf("[MORTGAGE PARSER] ❌ Ошибка DeepSeek API (статус %d): %s", dsResp.StatusCode, string(body))
		return nil, fmt.Errorf("DeepSeek API error (status %d): %s", dsResp.StatusCode, string(body))
	}

	var deepSeekResponse DeepSeekResponse
	if err := json.Unmarshal(body, &deepSeekResponse); err != nil {
		log.Printf("[MORTGAGE PARSER] ❌ Ошибка парсинга JSON ответа DeepSeek: %v", err)
		log.Printf("[MORTGAGE PARSER] 📄 Сырой ответ: %s", string(body))
		return nil, fmt.Errorf("ошибка парсинга ответа DeepSeek: %v", err)
	}

	log.Printf("[MORTGAGE PARSER] 📊 Количество choices в ответе: %d", len(deepSeekResponse.Choices))

	if len(deepSeekResponse.Choices) > 0 {
		raw := deepSeekResponse.Choices[0].Message.Content
		raw = strings.TrimSpace(raw)
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)

		log.Printf("[MORTGAGE PARSER] DeepSeek ответ для %s: %s", url, raw)
		log.Printf("[MORTGAGE PARSER] 📄 Длина ответа DeepSeek: %d символов", len(raw))

		var mortgage models.Mortgage
		if err := json.Unmarshal([]byte(raw), &mortgage); err != nil {
			log.Printf("[MORTGAGE PARSER ERROR] Ошибка парсинга JSON для %s: %v, raw: %s", url, err, raw)
			return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
		}

		// Устанавливаем время создания и URL
		mortgage.CreatedAt = time.Now()
		mortgage.URL = url

		// Улучшаем данные
		mp.improveMortgageData(&mortgage)

		log.Printf("[MORTGAGE PARSER] ✅ Успешно спарсили ипотеку: %s (ставка: %.1f%%-%.1f%%)", mortgage.BankName, mortgage.RateMin, mortgage.RateMax)

		return &mortgage, nil
	}

	return nil, fmt.Errorf("нет ответа от DeepSeek")
}

func (mp *MortgageParser) cleanText(raw string) string {
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

func (mp *MortgageParser) improveMortgageData(mortgage *models.Mortgage) {
	// Улучшаем название банка
	mortgage.BankName = mp.cleanBankName(mortgage.BankName)

	// Проверяем и корректируем ставки
	if mortgage.RateMin > mortgage.RateMax {
		mortgage.RateMin, mortgage.RateMax = mortgage.RateMax, mortgage.RateMin
	}

	// Проверяем разумные значения
	if mortgage.RateMin < 0 {
		mortgage.RateMin = 0
	}
	if mortgage.RateMax > 50 {
		mortgage.RateMax = 50
	}
	if mortgage.TermYears < 1 {
		mortgage.TermYears = 1
	}
	if mortgage.TermYears > 30 {
		mortgage.TermYears = 30
	}
	if mortgage.InitialPayment < 0 {
		mortgage.InitialPayment = 0
	}
	if mortgage.InitialPayment > 100 {
		mortgage.InitialPayment = 100
	}
}

func (mp *MortgageParser) cleanBankName(name string) string {
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
