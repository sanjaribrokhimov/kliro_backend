package services

import (
	"fmt"
	"kliro/models"
	"kliro/utils"
	"net/http"
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
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения страницы: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга HTML: %v", err)
	}

	return tp.ParseTransfersWithGoquery(doc), nil
}

func (tp *TransferParser) ParseTransfersWithGoquery(doc *goquery.Document) []*models.Transfer {
	var transfers []*models.Transfer

	// Для сайта bank.uz/uz/perevodi - ищем карточки переводов
	doc.Find(".banki-p2p__item").Each(func(i int, s *goquery.Selection) {
		transfer := &models.Transfer{
			CreatedAt: time.Now(),
		}

		// Название приложения - нормализуем
		appName := s.Find(".banki-p2p__name a").First().Text()
		normalizer := utils.GetBankNormalizer()
		transfer.AppName = normalizer.NormalizeBankName(strings.TrimSpace(appName))

		// Комиссия
		commissionText := s.Find(".banki-p2p__percent span").Last().Text()
		transfer.Commission = strings.TrimSpace(commissionText)

		// Лимиты
		limitText := s.Find(".banki-p2p__desc span").Last().Text()
		if limitText != "" {
			transfer.LimitUZ = &limitText
		}

		// Добавляем перевод если есть название приложения
		if transfer.AppName != "" {
			transfers = append(transfers, transfer)
		}
	})

	fmt.Printf("Найдено переводов: %d\n", len(transfers))
	return transfers
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
	if transfer.LimitUZ == nil || *transfer.LimitUZ == "Не указано" || *transfer.LimitUZ == "" {
		limitUZ := "Информация о лимитах не указана"
		transfer.LimitUZ = &limitUZ
	}

	if transfer.LimitUZ == nil || *transfer.LimitUZ == "Не указано" || *transfer.LimitUZ == "" {
		limitUZ := "Limit haqida ma'lumot ko'rsatilmagan"
		transfer.LimitUZ = &limitUZ
	}

}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
