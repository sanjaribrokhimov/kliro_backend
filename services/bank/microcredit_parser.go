package services

import (
	"fmt"
	"kliro/models"
	"kliro/utils"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type MicrocreditParser struct{}

func NewMicrocreditParser() *MicrocreditParser {
	return &MicrocreditParser{}
}

func (mp *MicrocreditParser) ParseURL(url string) ([]*models.Microcredit, error) {
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

	return mp.ParseMicrocreditsWithGoquery(doc), nil
}

func (mp *MicrocreditParser) ParseMicrocreditsWithGoquery(doc *goquery.Document) []*models.Microcredit {
	var microcredits []*models.Microcredit

	doc.Find(".table-card-offers-bottom").Each(func(i int, s *goquery.Selection) {
		microcredit := &models.Microcredit{
			CreatedAt: utils.UzbekTime(),
		}

		// Название банка - нормализуем
		bankName := s.Find(".table-card-offers-block1-text > span.medium-text").First().Text()
		normalizer := utils.GetBankNormalizer()
		microcredit.BankName = normalizer.NormalizeBankName(strings.TrimSpace(bankName))

		// Описание (название микрокредита)
		description := s.Find(".table-card-offers-block1-text a").First().Text()
		microcredit.Description = strings.TrimSpace(description)

		// URL ссылки
		if link := s.Find(".table-card-offers-block1-text a").First(); link.Length() > 0 {
			if href, exists := link.Attr("href"); exists {
				microcredit.URL = strings.TrimSpace(href)
			}
		}

		// Процентная ставка (блок 2) - сохраняем как есть
		rateText := s.Find(".table-card-offers-block2 > span.medium-text").First().Text()
		microcredit.Rate = strings.TrimSpace(rateText)

		// Срок (блок 3) - сохраняем как есть
		termText := s.Find(".table-card-offers-block3 > span.medium-text").First().Text()
		microcredit.Term = strings.TrimSpace(termText)

		// Сумма (блок 4) - сохраняем как есть
		amountText := s.Find(".table-card-offers-block4 > span.medium-text").First().Text()
		microcredit.Amount = strings.TrimSpace(amountText)

		// Канал (банк/онлайн) - сохраняем как есть
		channelText := s.Find(".table-card-offers-block5 .medium-text").Text()
		microcredit.Channel = strings.TrimSpace(channelText)

		// Добавляем микрокредит если есть название банка
		if microcredit.BankName != "" {
			microcredits = append(microcredits, microcredit)
		}
	})

	fmt.Printf("Найдено микрокредитов: %d\n", len(microcredits))
	return microcredits
}
