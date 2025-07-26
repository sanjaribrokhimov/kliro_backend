package services

import (
	"fmt"
	"kliro/models"
	"net/http"
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

	// Извлекаем название банка из URL
	bankName := mp.extractBankName(url)

	// Парсим данные с помощью регулярных выражений
	credit := &models.Microcredit{
		BankName:   bankName,
		URL:        url,
		MaxAmount:  0, // будет установлено позже
		TermMonths: 0, // будет установлено позже
		RateMin:    0, // будет установлено позже
		RateMax:    0, // будет установлено позже
		CreatedAt:  time.Now(),
	}

	// Устанавливаем значения, если они найдены
	if maxAmount := mp.extractMaxAmount(text); maxAmount != nil {
		credit.MaxAmount = *maxAmount
	}
	if termMonths := mp.extractTermMonths(text); termMonths != nil {
		credit.TermMonths = *termMonths
	}
	if rateMin := mp.extractRateMin(text); rateMin != nil {
		credit.RateMin = *rateMin
	}
	if rateMax := mp.extractRateMax(text); rateMax != nil {
		credit.RateMax = *rateMax
	}

	return credit, nil
}

func (mp *MicrocreditParser) extractBankName(url string) string {
	// Извлекаем название банка из URL
	re := regexp.MustCompile(`https?://(?:www\.)?([^.]+)\.`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return "Unknown Bank"
}

func (mp *MicrocreditParser) extractMaxAmount(text string) *float64 {
	// Ищем суммы в тексте
	re := regexp.MustCompile(`(\d+(?:\s*\d+)*)\s*(?:so'm|сум|сумма|миллион|млн)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		// Убираем пробелы и конвертируем в число
		amountStr := strings.ReplaceAll(matches[1], " ", "")
		if amount, err := mp.parseAmount(amountStr); err == nil {
			return &amount
		}
	}
	return nil
}

func (mp *MicrocreditParser) extractTermMonths(text string) *int {
	// Ищем сроки в месяцах
	re := regexp.MustCompile(`(\d+)\s*(?:oy|мес|месяц|месяцев)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		if months, err := mp.parseInt(matches[1]); err == nil {
			return &months
		}
	}
	return nil
}

func (mp *MicrocreditParser) extractRateMin(text string) *float64 {
	// Ищем минимальную ставку
	re := regexp.MustCompile(`(?:от|dan)\s*(\d+(?:\.\d+)?)\s*(?:%|foiz)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		if rate, err := mp.parseFloat(matches[1]); err == nil {
			return &rate
		}
	}
	return nil
}

func (mp *MicrocreditParser) extractRateMax(text string) *float64 {
	// Ищем максимальную ставку
	re := regexp.MustCompile(`(?:до|gacha)\s*(\d+(?:\.\d+)?)\s*(?:%|foiz)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		if rate, err := mp.parseFloat(matches[1]); err == nil {
			return &rate
		}
	}
	return nil
}

func (mp *MicrocreditParser) parseAmount(amountStr string) (float64, error) {
	// Простой парсер для сумм
	var amount float64
	_, err := fmt.Sscanf(amountStr, "%f", &amount)
	return amount, err
}

func (mp *MicrocreditParser) parseInt(intStr string) (int, error) {
	var result int
	_, err := fmt.Sscanf(intStr, "%d", &result)
	return result, err
}

func (mp *MicrocreditParser) parseFloat(floatStr string) (float64, error) {
	var result float64
	_, err := fmt.Sscanf(floatStr, "%f", &result)
	return result, err
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
