package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"kliro/models"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type MicrocreditParser struct{}

func NewMicrocreditParser() *MicrocreditParser {
	return &MicrocreditParser{}
}

func (mp *MicrocreditParser) ParseURL(url string) (*models.Microcredit, error) {
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

	text = mp.cleanText(text)

	// Используем DeepSeek для парсинга
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

		var parsedCredit models.Microcredit
		if err := json.Unmarshal([]byte(raw), &parsedCredit); err != nil {
			return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
		}

		// Устанавливаем время создания
		parsedCredit.CreatedAt = time.Now()

		return &parsedCredit, nil
	}

	return nil, fmt.Errorf("нет ответа от DeepSeek")
}

func (mp *MicrocreditParser) cleanText(raw string) string {
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
