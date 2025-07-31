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
	log.Printf("[TRANSFER PARSER] 🚀 Начинаем парсинг URL: %s", url)

	// Получаем HTML страницы
	log.Printf("[TRANSFER PARSER] 🌐 Загружаем страницу...")
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[TRANSFER PARSER] ❌ Ошибка получения страницы: %v", err)
		return nil, fmt.Errorf("ошибка получения страницы: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("[TRANSFER PARSER] 📡 Статус страницы: %d", resp.StatusCode)

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("[TRANSFER PARSER] ❌ Ошибка парсинга HTML: %v", err)
		return nil, fmt.Errorf("ошибка парсинга HTML: %v", err)
	}

	// Берем ВЕСЬ текст со страницы
	text := doc.Find("body").Text()
	log.Printf("[TRANSFER PARSER] 📄 Исходный текст: %d символов", len(text))

	// Очищаем текст от лишнего
	text = tp.cleanText(text)

	log.Printf("[TRANSFER PARSER] Очищенный текст для %s (первые 5000 символов):", url)
	log.Printf(text[:min(len(text), 5000)])
	log.Printf("[TRANSFER PARSER] 📏 Общая длина текста: %d символов", len(text))

	// Простой промпт для DeepSeek
	prompt := fmt.Sprintf(`Извлеки информацию о ВСЕХ приложениях для переводов из текста и верни JSON-массив объектов.

Каждый объект должен содержать:
app_name: название приложения
commission: комиссия за переводы (например, "0%%", "0.5%%", "1%%")
limit_ru: информация о лимитах на русском языке (null если не найдено)
limit_uz: информация о лимитах на узбекском языке нужно взять из руского и перевести на узбекский (null если не найдено )

Если лимиты не указаны в тексте, используй null вместо пустой строки.

Найди ВСЕ приложения для переводов в тексте. Не пропускай ничего.

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
		log.Printf("[TRANSFER PARSER] ❌ DEEPSEEK_API_KEY не установлен")
		return nil, fmt.Errorf("DeepSeek API key not set")
	}
	log.Printf("[TRANSFER PARSER] ✅ DEEPSEEK_API_KEY найден (длина: %d)", len(apiKey))

	req.Header.Set("Authorization", "Bearer "+apiKey)

	log.Printf("[TRANSFER PARSER] 🌐 Отправляем запрос к DeepSeek API...")
	client := &http.Client{}
	dsResp, err := client.Do(req)
	if err != nil {
		log.Printf("[TRANSFER PARSER] ❌ Ошибка HTTP запроса к DeepSeek: %v", err)
		return nil, fmt.Errorf("ошибка вызова DeepSeek API: %v", err)
	}
	defer dsResp.Body.Close()

	log.Printf("[TRANSFER PARSER] 📡 Статус ответа DeepSeek: %d", dsResp.StatusCode)

	body, _ := ioutil.ReadAll(dsResp.Body)
	log.Printf("[TRANSFER PARSER] 📄 Размер ответа DeepSeek: %d байт", len(body))

	if dsResp.StatusCode != 200 {
		log.Printf("[TRANSFER PARSER] ❌ Ошибка DeepSeek API (статус %d): %s", dsResp.StatusCode, string(body))
		return nil, fmt.Errorf("DeepSeek API error (status %d): %s", dsResp.StatusCode, string(body))
	}

	var deepSeekResponse DeepSeekResponse
	if err := json.Unmarshal(body, &deepSeekResponse); err != nil {
		log.Printf("[TRANSFER PARSER] ❌ Ошибка парсинга JSON ответа DeepSeek: %v", err)
		log.Printf("[TRANSFER PARSER] 📄 Сырой ответ: %s", string(body))
		return nil, fmt.Errorf("ошибка парсинга ответа DeepSeek: %v", err)
	}

	log.Printf("[TRANSFER PARSER] 📊 Количество choices в ответе: %d", len(deepSeekResponse.Choices))

	if len(deepSeekResponse.Choices) > 0 {
		raw := deepSeekResponse.Choices[0].Message.Content
		raw = strings.TrimSpace(raw)
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)

		log.Printf("[TRANSFER PARSER] DeepSeek ответ для %s: %s", url, raw)
		log.Printf("[TRANSFER PARSER] 📄 Длина ответа DeepSeek: %d символов", len(raw))

		var parsedTransfers []*models.Transfer
		if err := json.Unmarshal([]byte(raw), &parsedTransfers); err != nil {
			log.Printf("[TRANSFER PARSER ERROR] Ошибка парсинга JSON для %s: %v, raw: %s", url, err, raw)
			return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
		}

		// Устанавливаем время создания для всех записей
		for i, transfer := range parsedTransfers {
			transfer.CreatedAt = time.Now()
			log.Printf("[TRANSFER PARSER] 📝 Приложение %d: %s (комиссия: %s)", i+1, transfer.AppName, transfer.Commission)
		}

		// Удаляем дубликаты и улучшаем данные
		uniqueTransfers := tp.removeDuplicatesAndImprove(parsedTransfers)

		log.Printf("[TRANSFER PARSER] ✅ Успешно спарсили %d приложений, после удаления дубликатов: %d", len(parsedTransfers), len(uniqueTransfers))

		return uniqueTransfers, nil
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
	if len(clean) > 20000 {
		clean = clean[:20000]
	}

	return clean
}

// removeDuplicatesAndImprove удаляет дубликаты и улучшает данные
func (tp *TransferParser) removeDuplicatesAndImprove(transfers []*models.Transfer) []*models.Transfer {
	seen := make(map[string]bool)
	var uniqueTransfers []*models.Transfer

	for _, transfer := range transfers {
		// Нормализуем название приложения
		normalizedName := tp.normalizeAppName(transfer.AppName)

		// Пропускаем дубликаты
		if seen[normalizedName] {
			log.Printf("[TRANSFER PARSER] 🔄 Пропускаем дубликат: %s", transfer.AppName)
			continue
		}

		// Улучшаем данные
		tp.improveTransferData(transfer)

		// Добавляем в результат
		uniqueTransfers = append(uniqueTransfers, transfer)
		seen[normalizedName] = true
	}

	return uniqueTransfers
}

// normalizeAppName нормализует название приложения для сравнения
func (tp *TransferParser) normalizeAppName(name string) string {
	// Приводим к нижнему регистру
	normalized := strings.ToLower(strings.TrimSpace(name))

	// Удаляем лишние пробелы
	normalized = strings.Join(strings.Fields(normalized), " ")

	// Убираем общие суффиксы
	normalized = strings.TrimSuffix(normalized, " mobile")
	normalized = strings.TrimSuffix(normalized, " bank")
	normalized = strings.TrimSuffix(normalized, " pay")

	return normalized
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
