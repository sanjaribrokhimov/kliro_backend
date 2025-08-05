package services

import (
	"fmt"
	"kliro/models"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type AutocreditParser struct{}

func NewAutocreditParser() *AutocreditParser {
	return &AutocreditParser{}
}

func (ap *AutocreditParser) ParseURL(url string) ([]*models.Autocredit, error) {
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

	var autocredits []*models.Autocredit

	// Ищем все карточки автокредитов
	fmt.Printf("[AUTOCREDIT PARSER] Ищем элементы с классом .table-card-offers-bottom\n")
	doc.Find(".table-card-offers-bottom").Each(func(i int, s *goquery.Selection) {
		autocredit := &models.Autocredit{}

		// Название банка (блок 1)
		bankName := s.Find(".table-card-offers-block1-text span.medium-text").First().Text()
		autocredit.BankName = strings.TrimSpace(bankName)

		// Описание (название автокредита)
		description := s.Find(".table-card-offers-block1-text a").First().Text()
		autocredit.Description = strings.TrimSpace(description)

		// Процентная ставка (блок 2) - сохраняем как есть
		rateText := s.Find(".table-card-offers-block2 span.medium-text").First().Text()
		autocredit.Rate = strings.TrimSpace(rateText)

		// Срок (блок 3) - сохраняем как есть
		termText := s.Find(".table-card-offers-block3 span.medium-text").First().Text()
		autocredit.Term = strings.TrimSpace(termText)

		// Сумма (блок 4) - сохраняем как есть
		amountText := s.Find(".table-card-offers-block4 span.medium-text").First().Text()
		autocredit.Amount = strings.TrimSpace(amountText)

		// Канал (банк/онлайн) - сохраняем как есть
		channelText := s.Find(".table-card-offers-block5 span.medium-text").Last().Text()
		autocredit.Channel = strings.TrimSpace(channelText)

		// Добавляем автокредит если есть название банка
		if autocredit.BankName != "" {
			autocredits = append(autocredits, autocredit)
			fmt.Printf("[AUTOCREDIT PARSER] Найден автокредит: %s - %s (ставка: %s, срок: %s, сумма: %s, канал: %s)\n",
				autocredit.BankName, autocredit.Description, autocredit.Rate, autocredit.Term, autocredit.Amount, autocredit.Channel)
		}
	})

	fmt.Printf("[AUTOCREDIT PARSER] Всего найдено автокредитов: %d\n", len(autocredits))
	return autocredits, nil
}
