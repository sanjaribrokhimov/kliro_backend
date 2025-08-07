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

type DepositParser struct{}

func NewDepositParser() *DepositParser {
	return &DepositParser{}
}

func (dp *DepositParser) ParseURL(url string) ([]*models.Deposit, error) {
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

	return dp.ParseDepositsWithGoquery(doc), nil
}

func (dp *DepositParser) ParseDepositsWithGoquery(doc *goquery.Document) []*models.Deposit {
	var deposits []*models.Deposit

	doc.Find(".table-card-offers-bottom").Each(func(i int, s *goquery.Selection) {
		deposit := &models.Deposit{
			CreatedAt: time.Now(),
		}

		// Название банка - нормализуем
		bankName := s.Find(".table-card-offers-block1-text > span.medium-text").First().Text()
		normalizer := utils.GetBankNormalizer()
		deposit.BankName = normalizer.NormalizeBankName(strings.TrimSpace(bankName))

		// Название депозита - берем как есть
		depositName := s.Find(".table-card-offers-block1-text a").First().Text()
		deposit.Title = strings.TrimSpace(depositName)

		// Ставка - берем как есть, только убираем пробелы
		rateText := s.Find(".table-card-offers-block2 > span.medium-text").First().Text()
		deposit.Rate = strings.TrimSpace(rateText)

		// Срок - берем как есть, только убираем пробелы
		termText := s.Find(".table-card-offers-block3 > span.medium-text").First().Text()
		deposit.TermYears = strings.TrimSpace(termText)

		// Минимальная сумма - берем как есть, только убираем пробелы
		minAmountText := s.Find(".table-card-offers-block4 > span.medium-text").First().Text()
		deposit.MinAmount = strings.TrimSpace(minAmountText)

		// Добавляем вклад если есть название банка
		if deposit.BankName != "" {
			deposits = append(deposits, deposit)
		}
	})

	fmt.Printf("Найдено вкладов: %d\n", len(deposits))
	return deposits
}
