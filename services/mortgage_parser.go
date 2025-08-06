package services

import (
	"fmt"
	"kliro/models"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type MortgageParser struct{}

func NewMortgageParser() *MortgageParser {
	return &MortgageParser{}
}

func (mp *MortgageParser) ParseURL(url string) ([]*models.Mortgage, error) {
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

	return mp.ParseMortgagesWithGoquery(doc), nil
}

func (mp *MortgageParser) ParseMortgagesWithGoquery(doc *goquery.Document) []*models.Mortgage {
	var mortgages []*models.Mortgage

	// Для сайта bank.uz/uz/ipoteka - ищем карточки ипотек
	doc.Find(".table-card-offers-bottom").Each(func(i int, s *goquery.Selection) {
		mortgage := &models.Mortgage{
			CreatedAt: time.Now(),
		}

		// Название банка
		bankName := s.Find(".table-card-offers-block1-text > span.medium-text").First().Text()
		mortgage.BankName = strings.TrimSpace(bankName)

		// Описание (название ипотеки)
		description := s.Find(".table-card-offers-block1-text a").First().Text()
		mortgage.Description = strings.TrimSpace(description)

		// Процентная ставка (блок 2) - сохраняем как есть
		rateText := s.Find(".table-card-offers-block2 > span.medium-text").First().Text()
		mortgage.Rate = strings.TrimSpace(rateText)

		// Срок (блок 3) - сохраняем как есть
		termText := s.Find(".table-card-offers-block3 > span.medium-text").First().Text()
		mortgage.Term = strings.TrimSpace(termText)

		// Сумма (блок 4) - сохраняем как есть
		amountText := s.Find(".table-card-offers-block4 > span.medium-text").First().Text()
		mortgage.Amount = strings.TrimSpace(amountText)

		// Канал (банк/онлайн) - сохраняем как есть
		channelText := s.Find(".table-card-offers-block5 .medium-text").Text()
		mortgage.Channel = strings.TrimSpace(channelText)

		// Добавляем ипотеку если есть название банка
		if mortgage.BankName != "" {
			mortgages = append(mortgages, mortgage)
		}
	})

	fmt.Printf("[MORTGAGE PARSER] Всего найдено ипотек: %d\n", len(mortgages))
	return mortgages
}
