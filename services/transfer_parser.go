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

type TransferParser struct{}

func NewTransferParser() *TransferParser {
	return &TransferParser{}
}

func (tp *TransferParser) ParseURL(url string) ([]*models.Transfer, error) {
	log.Printf("[TRANSFER PARSER] Начало парсинга переводов")

	// Получаем HTML страницы
	httpClient := &http.Client{
		Timeout: 15 * time.Second,
	}
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения страницы: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга HTML: %v", err)
	}

	// Берем весь текст со страницы
	text := doc.Find("body").Text()

	// Очищаем текст от лишнего
	text = tp.cleanText(text)

	// Простой промпт для DeepSeek
	prompt := fmt.Sprintf(`Извлеки информацию о ВСЕХ приложениях для P2P переводов из текста и верни JSON-массив объектов.

Каждый объект должен содержать:
app_name: название приложения (например, "Davr Mobile 2.0", "Paynet", "xazna", "Mavrid", "Milliy", "SQB Mobile", "Anorbank", "Uzum Bank", "AVO", "TBC UZ", "Payme", "Click Up", "Paylov", "A-Pay", "Limon Pay")
commission: комиссия за переводы (например, "0%%", "0.5%%", "1%%", "0.6%%", "0.7%%")
limit_ru: информация о лимитах на русском языке (null если не найдено)
limit_uz: информация о лимитах на узбекском языке (null если не найдено)

Найди ВСЕ приложения для переводов в тексте. Обрати внимание на раздел "P2P переводы с карты на карту".

Текст: "%s"
Верни только JSON-массив.`, text)

	// Вызываем DeepSeek API
	reqBody := DeepSeekRequest{
		Model:       "deepseek-chat",
		Messages:    []Message{{Role: "user", Content: prompt}},
		MaxTokens:   8192,
		Temperature: 0.0,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("DeepSeek API key not set")
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	dsResp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка вызова DeepSeek API: %v", err)
	}
	defer dsResp.Body.Close()

	body, _ := ioutil.ReadAll(dsResp.Body)

	if dsResp.StatusCode != 200 {
		return nil, fmt.Errorf("DeepSeek API error (status %d): %s", dsResp.StatusCode, string(body))
	}

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

		var transfers []*models.Transfer
		if err := json.Unmarshal([]byte(raw), &transfers); err != nil {
			return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
		}

		// Устанавливаем время создания для каждого перевода
		for _, transfer := range transfers {
			transfer.CreatedAt = time.Now()
			tp.improveTransferData(transfer)
		}

		return transfers, nil
	}

	return nil, fmt.Errorf("нет ответа от DeepSeek")
}

func (tp *TransferParser) cleanText(raw string) string {
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

// improveTransferData улучшает данные перевода
func (tp *TransferParser) improveTransferData(transfer *models.Transfer) {
	// Улучшаем комиссию
	if transfer.Commission == "Не указано" || transfer.Commission == "" {
		transfer.Commission = "0%"
	}

	// Улучшаем лимиты
	if transfer.LimitRU == nil || *transfer.LimitRU == "Не указано" || *transfer.LimitRU == "" {
		limitRU := "Информация о лимитах не указана"
		transfer.LimitRU = &limitRU
	}

	if transfer.LimitUZ == nil || *transfer.LimitUZ == "Не указано" || *transfer.LimitUZ == "" {
		limitUZ := "Limit haqida ma'lumot ko'rsatilmagan"
		transfer.LimitUZ = &limitUZ
	}

	// Улучшаем название приложения
	transfer.AppName = tp.cleanAppName(transfer.AppName)
}

// cleanAppName очищает название приложения
func (tp *TransferParser) cleanAppName(name string) string {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
