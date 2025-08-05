package services

import (
	"fmt"
	"kliro/models"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type MortgageParser struct{}

func NewMortgageParser() *MortgageParser {
	return &MortgageParser{}
}

func (mp *MortgageParser) ParseURL(url string) ([]*models.Mortgage, error) {
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

	var mortgages []*models.Mortgage

	// Ищем все карточки ипотечных кредитов
	doc.Find(".table-card-offers-block").Each(func(i int, s *goquery.Selection) {
		mortgage := &models.Mortgage{}

		// Название банка (блок 1)
		bankName := s.Find(".table-card-offers-block1-text").Text()
		mortgage.BankName = strings.TrimSpace(bankName)

		// Описание (название ипотечного кредита)
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

	return mortgages, nil
}
