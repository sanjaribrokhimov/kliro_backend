package services

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type CurrencyParser struct {
	currencyService *CurrencyService
}

func NewCurrencyParser(currencyService *CurrencyService) *CurrencyParser {
	return &CurrencyParser{
		currencyService: currencyService,
	}
}

func (cp *CurrencyParser) ParseAndSaveCurrencyRates() error {
	log.Printf("[CURRENCY PARSER] Начинаем парсинг курсов валют...")

	url := "https://bank.uz/uz/currency"

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[CURRENCY PARSER ERROR] Ошибка создания запроса: %v", err)
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[CURRENCY PARSER ERROR] Ошибка получения страницы: %v", err)
		return err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("[CURRENCY PARSER ERROR] Ошибка парсинга HTML: %v", err)
		return err
	}

	rates := cp.ParseCurrencyRatesWithGoquery(doc)

	if err := cp.currencyService.SaveCurrencyRates(rates); err != nil {
		log.Printf("[CURRENCY PARSER ERROR] Ошибка сохранения курсов: %v", err)
		return err
	}

	log.Printf("[CURRENCY PARSER] Успешно обработано %d курсов валют", len(rates))
	return nil
}

func (cp *CurrencyParser) ParseCurrencyRates(html string) ([]map[string]interface{}, error) {
	// Создаем документ из HTML строки
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга HTML: %v", err)
	}

	return cp.ParseCurrencyRatesWithGoquery(doc), nil
}

func (cp *CurrencyParser) ParseCurrencyRatesWithGoquery(doc *goquery.Document) []map[string]interface{} {
	var rates []map[string]interface{}

	currencies := []string{"USD", "EUR", "RUB", "KZT"}

	for _, currency := range currencies {
		// Находим блок для конкретной валюты
		currencySelector := fmt.Sprintf("#best_%s", currency)
		currencyBlock := doc.Find(currencySelector)

		if currencyBlock.Length() == 0 {
			log.Printf("[CURRENCY PARSER] ❌ Не найден блок для валюты %s", currency)
			continue
		}

		log.Printf("[CURRENCY PARSER] ✅ Найден блок для валюты %s", currency)

		// Извлекаем курсы покупки (левый блок)
		buyRates := cp.extractBuyRatesFromBlock(currencyBlock)
		log.Printf("[CURRENCY PARSER] Найдено %d курсов покупки для %s", len(buyRates), currency)

		// Извлекаем курсы продажи (правый блок)
		sellRates := cp.extractSellRatesFromBlock(currencyBlock)
		log.Printf("[CURRENCY PARSER] Найдено %d курсов продажи для %s", len(sellRates), currency)

		// Создаем map для быстрого поиска курсов продажи по банку
		sellMap := make(map[string]float64)
		for _, sellRate := range sellRates {
			sellMap[sellRate.bankName] = sellRate.rate
		}

		// Объединяем данные
		for _, buyRate := range buyRates {
			sellRate, exists := sellMap[buyRate.bankName]
			if exists {
				rate := map[string]interface{}{
					"currency": currency,
					"bank":     buyRate.bankName,
					"buy":      buyRate.rate,
					"sell":     sellRate,
				}
				rates = append(rates, rate)
			}
		}
	}

	return rates
}

type RateData struct {
	bankName string
	rate     float64
}

func (cp *CurrencyParser) extractBuyRatesFromBlock(block *goquery.Selection) []RateData {
	var rates []RateData

	// Находим левый блок с курсами покупки
	leftBlock := block.Find(".bc-inner-blocks-left")
	if leftBlock.Length() == 0 {
		log.Printf("[CURRENCY PARSER] Не найден левый блок покупки")
		return rates
	}

	// Извлекаем курсы покупки
	leftBlock.Find(".bc-inner-block-left-texts").Each(func(i int, s *goquery.Selection) {
		// Название банка
		bankName := s.Find(".bc-inner-block-left-text .medium-text").Text()
		bankName = strings.TrimSpace(bankName)

		// Курс покупки
		rateText := s.Find(".medium-text.green-date").Text()
		rateText = strings.TrimSpace(rateText)
		rateText = strings.ReplaceAll(rateText, " ", "")
		rateText = strings.ReplaceAll(rateText, "so'm", "")

		if bankName != "" && rateText != "" {
			rate, err := strconv.ParseFloat(rateText, 64)
			if err == nil {
				log.Printf("[CURRENCY PARSER] Курс покупки: %s = %.2f", bankName, rate)
				rates = append(rates, RateData{
					bankName: bankName,
					rate:     rate,
				})
			} else {
				log.Printf("[CURRENCY PARSER] Ошибка парсинга курса покупки: %s -> %s (ошибка: %v)", bankName, rateText, err)
			}
		}
	})

	return rates
}

func (cp *CurrencyParser) extractSellRatesFromBlock(block *goquery.Selection) []RateData {
	var rates []RateData

	// Находим правый блок с курсами продажи
	rightBlock := block.Find(".bc-inner-blocks-right")
	if rightBlock.Length() == 0 {
		log.Printf("[CURRENCY PARSER] Не найден правый блок продажи")
		return rates
	}

	// Извлекаем курсы продажи
	rightBlock.Find(".bc-inner-block-left-texts").Each(func(i int, s *goquery.Selection) {
		// Название банка
		bankName := s.Find(".bc-inner-block-left-text .medium-text").Text()
		bankName = strings.TrimSpace(bankName)

		// Курс продажи
		rateText := s.Find(".medium-text.green-date").Text()
		rateText = strings.TrimSpace(rateText)
		rateText = strings.ReplaceAll(rateText, " ", "")
		rateText = strings.ReplaceAll(rateText, "so'm", "")

		if bankName != "" && rateText != "" {
			rate, err := strconv.ParseFloat(rateText, 64)
			if err == nil {
				log.Printf("[CURRENCY PARSER] Курс продажи: %s = %.2f", bankName, rate)
				rates = append(rates, RateData{
					bankName: bankName,
					rate:     rate,
				})
			} else {
				log.Printf("[CURRENCY PARSER] Ошибка парсинга курса продажи: %s -> %s (ошибка: %v)", bankName, rateText, err)
			}
		}
	})

	return rates
}
