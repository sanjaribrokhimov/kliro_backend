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
		if strings.Contains(txt, "комиссия") || strings.Contains(txt, "лимит") || strings.Contains(txt, "перевод") || strings.Contains(txt, "%") || strings.Contains(txt, "млн") || strings.Contains(txt, "сум") {
			relevantText = append(relevantText, s.Text())
		}
	})

	var text string
	if len(relevantText) > 0 {
		text = strings.Join(relevantText, " ")
	} else {
		text = doc.Find("body").Text()
	}

	text = tp.cleanText(text)

	log.Printf("[TRANSFER PARSER] Очищенный текст для %s (первые 3000 символов):", url)
	log.Printf(text[:min(len(text), 3000)])
	log.Printf("[TRANSFER PARSER] 📏 Общая длина текста: %d символов", len(text))

	// Используем DeepSeek для парсинга ВСЕХ приложений
	prompt := fmt.Sprintf(`Извлеки информацию о ВСЕХ приложениях для переводов из текста и верни JSON-массив объектов.

КРИТИЧЕСКИ ВАЖНО: На странице должно быть 30-50 приложений для переводов. Ты должен найти ВСЕ из них!

Каждый объект должен содержать:
app_name: название приложения (например, "Davr Mobile", "Paynet", "xazna", "Mavrid", "Milliy", "SQB Mobile", "Anorbank", "Smartbank", "Oq", "Hamkor", "Humans", "My Uztelecom", "Uzum Bank", "AVO", "TBC UZ", "Payme", "Click Up", "Paylov", "A-Pay", "Limon Pay", "Uzum", "TBC", "Humo", "UzCard", "Visa", "Mastercard", "Click", "Payme", "Uzum Bank", "TBC Bank", "Anor Bank", "Hamkor Bank", "SQB Bank", "Milliy Bank", "Ipoteka Bank", "Turon Bank", "Aloqa Bank", "Xalq Bank", "Agro Bank", "Asaka Bank", "NBU", "CBU" и т.д.)
commission: комиссия за переводы (например, "0%%", "0.5%%", "1%%", "0.7%%" и т.д.)
limit_ru: информация о лимитах и условиях НА РУССКОМ ЯЗЫКЕ (например, "Ежемесячно за переводы до 5 млн сум комиссия 0%%, затем 0.5%%", "Комиссия за переводы составляет 0%% в пределах лимита 5 млн в месяц, далее 0.5%%" и т.д.)
limit_uz: информация о лимитах и условиях НА УЗБЕКСКОМ ЯЗЫКЕ (например, "Har oy 5 mln so'mgacha o'tkazmalar uchun komissiya 0%%, keyin 0.5%%", "O'tkazmalar uchun komissiya oylik 5 mln so'm limit doirasida 0%%, keyin 0.5%%" и т.д.)

ИНСТРУКЦИИ:
1. Найди ВСЕ приложения для переводов на странице (должно быть 30-50 приложений)
2. Ищи названия банков, платежных систем, мобильных приложений
3. Извлекай данные как с русскоязычных, так и с узбекоязычных сайтов
4. Если какое-то значение не найдено — укажи "Не указано"
5. Каждое приложение должно быть отдельным объектом в массиве
6. НЕ ПРОПУСКАЙ НИ ОДНОГО ПРИЛОЖЕНИЯ!
7. ВАЖНО: limit_ru должен содержать информацию на русском языке, limit_uz на узбекском языке

Ключевые слова для поиска:
комиссия — commission
лимит — limit
перевод — transfer
млн — million
сум — sum
месяц — month
год — year

Текст: "%s"
Верни только JSON-массив. Без пояснений.`, text)

	// Вызываем DeepSeek API
	reqBody := DeepSeekRequest{
		Model:       "deepseek-chat",
		Messages:    []Message{{Role: "user", Content: prompt}},
		MaxTokens:   8192,
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

		log.Printf("[TRANSFER PARSER] ✅ Успешно спарсили %d приложений для %s", len(parsedTransfers), url)

		return parsedTransfers, nil
	}

	return nil, fmt.Errorf("нет ответа от DeepSeek")
}

func (tp *TransferParser) cleanText(raw string) string {
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

	if len(clean) > 10000 {
		clean = clean[:10000]
	}
	return clean
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
