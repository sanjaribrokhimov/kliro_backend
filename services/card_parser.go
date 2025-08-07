package services

import (
	"fmt"
	"kliro/models"
	"kliro/utils"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type CardParser struct{}

func NewCardParser() *CardParser {
	return &CardParser{}
}

func (cp *CardParser) ParseURL(url string) ([]*models.Card, error) {
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

	return cp.ParseCardsWithGoquery(doc), nil
}

func (cp *CardParser) ParseCardsWithGoquery(doc *goquery.Document) []*models.Card {
	var cards []*models.Card

	doc.Find(".table-card-offers-bottom").Each(func(i int, s *goquery.Selection) {
		card := &models.Card{
			CreatedAt: time.Now(),
		}

		// Название банка - нормализуем
		bankName := s.Find(".table-card-offers-block1-text > span.medium-text").First().Text()
		normalizer := utils.GetBankNormalizer()
		card.BankName = normalizer.NormalizeBankName(strings.TrimSpace(bankName))

		// Название карты - берем как есть
		cardTitle := s.Find(".table-card-offers-block1-text a").First().Text()
		card.Title = strings.TrimSpace(cardTitle)

		// Описание (второй span в block1-text) - может быть пустым, но не сохраняем в модель
		_ = s.Find(".table-card-offers-block1-text span.medium-text").Last().Text()

		// Валюта - берем как есть
		currencyText := s.Find(".table-card-offers-block2 > span.medium-text").First().Text()
		card.Currency = strings.TrimSpace(currencyText)

		// Система (MasterCard, Visa и т.д.) - берем как есть
		systemText := s.Find(".table-card-offers-block3 > span.medium-text").First().Text()
		card.System = strings.TrimSpace(systemText)

		// Тип открытия (онлайн, в банке) - берем как есть
		openingText := s.Find(".table-card-offers-block4 > span.medium-text").First().Text()
		card.OpeningType = strings.TrimSpace(openingText)

		// Добавляем карту если есть название банка
		if card.BankName != "" {
			cards = append(cards, card)
		}
	})

	fmt.Printf("Найдено карт: %d\n", len(cards))
	return cards
}
