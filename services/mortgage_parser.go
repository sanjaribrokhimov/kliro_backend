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

type MortgageParser struct{}

func NewMortgageParser() *MortgageParser {
	return &MortgageParser{}
}

func (mp *MortgageParser) ParseURL(url string) (*models.Mortgage, error) {
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
		if strings.Contains(txt, "ипотек") || strings.Contains(txt, "ipotek") || strings.Contains(txt, "yil") || strings.Contains(txt, "foiz") || strings.Contains(txt, "so'm") || strings.Contains(txt, "%") || strings.Contains(txt, "год") {
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
	prompt := fmt.Sprintf(`Извлеки информацию об ипотечном кредите из текста и верни JSON-объект со следующими полями:

bank_name: название банка, извлеки из URL (например, если URL — "https://www.ipoteka.uz/credits", то bank_name = "ipoteka"; если невозможно определить — null)
url: оригинальный URL
max_amount: максимальная сумма кредита в миллионах сум (только число, если не указана — null). Если указано в миллиардах, переведи в миллионы (например: 2 млрд → 2000)
term_years: срок кредита в годах (только число, если не указано — null). Если срок указан в месяцах, переведи в годы и округли вверх (например: 60 месяцев → 5 лет)
rate: процентная ставка (только число, если указано, иначе — null). ИЩИ ТОЛЬКО РЕАЛЬНЫЕ ПРОЦЕНТЫ. Если указан диапазон "от X до Y", берем минимальную ставку X. Если указан только один процент, берем его. Если процент не найден — null
initial_payment: первоначальный взнос в процентах (только число, если не указано — null). ИЩИ ТОЛЬКО РЕАЛЬНЫЕ ПРОЦЕНТЫ ВЗНОСА. Если не найден — null

ВАЖНО: ИЩИ ТОЛЬКО РЕАЛЬНЫЕ ДАННЫЕ. Если что-то не указано на сайте — ставь null.

Важно: извлекай данные как с русскоязычных, так и с узбекоязычных сайтов.
Учитывай следующие слова и их значение:
foiz — процентная ставка
dan — от (для rate)
gacha — до (для max_amount, term_years)
yil, yilgacha, yil muddati — срок в годах (например: 20 yilgacha → term_years: 20)
so'm, so'mgacha, miqdori — сумма кредита
ipoteka, ipoteka krediti — ипотечный кредит
kredit muddati — срок кредита
kredit miqdori — сумма кредита
boshlang'ich to'lov — первоначальный взнос

Если сумма или срок указаны диапазоном (например: 10-20 yil), выдели только максимальное значение.

Правила для процентных ставок:
- Если указано "от X до Y" → rate = X (минимальная)
- Если указано только "от X" → rate = X
- Если указано только "до Y" → rate = Y
- Если указан только процент без "от" или "до" (например, "15", "15 годовых") → rate = 15
- Если процент НЕ НАЙДЕН на сайте → rate = null

Правила для первоначального взноса:
- Если указано "от X" → initial_payment = X
- Если указано "X" → initial_payment = X
- Если указано "X процентов" → initial_payment = X
- Если взнос НЕ НАЙДЕН на сайте → initial_payment = null

Обязательно:
Если на странице указано несколько видов кредитов, извлекай только ипотечный кредит.
Если не указано слово "ипотека" или "ipoteka", всё равно извлекай данные только по одному (любому) кредиту.
НЕ ПРИДУМЫВАЙ ДАННЫЕ. Если что-то не указано — ставь null.

Текст: "%s"
URL: "%s"
Верни только JSON. Без пояснений. Если какое-то значение не найдено — укажи null.`, text, url)

	// Вызываем DeepSeek API
	reqBody := DeepSeekRequest{
		Model:       "deepseek-chat",
		Messages:    []Message{{Role: "user", Content: prompt}},
		MaxTokens:   512,
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

		var parsedCredit models.Mortgage
		if err := json.Unmarshal([]byte(raw), &parsedCredit); err != nil {
			return nil, fmt.Errorf("ошибка парсинга JSON: %v", err)
		}

		// Устанавливаем время создания
		parsedCredit.CreatedAt = time.Now()

		return &parsedCredit, nil
	}

	return nil, fmt.Errorf("нет ответа от DeepSeek")
}

func (mp *MortgageParser) cleanText(raw string) string {
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
