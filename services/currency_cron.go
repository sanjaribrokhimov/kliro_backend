package services

import (
	"kliro/models"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// Функция для парсинга валют
func parseCurrencyData(logger *log.Logger) []*models.Currency {
	// Парсим напрямую, без обращения к API
	parser := NewCurrencyParser(nil) // Временно передаем nil, так как нам нужен только парсер
	
	// Получаем курсы валют напрямую с сайта
	url := "https://bank.uz/uz/currency"
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Printf("Ошибка создания запроса: %v", err)
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("Ошибка получения страницы: %v", err)
		return nil
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		logger.Printf("Ошибка парсинга HTML: %v", err)
		return nil
	}
	
	// Получаем курсы валют
	rates := parser.ParseCurrencyRatesWithGoquery(doc)
	
	// Конвертируем в модель Currency
	var currencies []*models.Currency
	for _, rate := range rates {
		currency, ok := rate["currency"].(string)
		if !ok {
			continue
		}
		
		bank, ok := rate["bank"].(string)
		if !ok {
			continue
		}
		
		buyRate, ok := rate["buy"].(float64)
		if !ok {
			continue
		}
		
		sellRate, ok := rate["sell"].(float64)
		if !ok {
			sellRate = 0
		}
		
		currencies = append(currencies, &models.Currency{
			BankName:  bank,
			Currency:  currency,
			BuyRate:   buyRate,
			SellRate:  &sellRate,
			CreatedAt: time.Now(),
		})
	}
	
	logger.Printf("Парсинг валют завершен - найдено %d курсов", len(currencies))
	return currencies
}

// Инициализация данных (первый запуск)
func InitializeCurrencyData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	logger.Printf("Инициализация данных currency - очищаем базу и парсим заново...")

	// Очищаем обе таблицы
	db.Exec("TRUNCATE new_currency")
	db.Exec("TRUNCATE old_currency")

	// Парсим валюты и сохраняем в обе таблицы
	if currencies := parseCurrencyData(logger); currencies != nil {
		for _, currency := range currencies {
			db.Table("new_currency").Create(currency)
			db.Table("old_currency").Create(currency)
		}
		logger.Printf("Инициализация завершена - заполнены таблицы new_currency и old_currency")
	} else {
		logger.Printf("Ошибка при инициализации данных currency")
	}
}

func StartCurrencyCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeCurrencyData(db)

	c := cron.New()
	c.AddFunc("0 0 */3 * * *", func() { // Каждые 3 часа
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало парсинга currency (каждые 3 часа)...")

		// Копируем new_currency в old_currency
		db.Exec("TRUNCATE old_currency")
		db.Exec("INSERT INTO old_currency SELECT * FROM new_currency")
		db.Exec("TRUNCATE new_currency")

		// Парсим валюты заново
		if currencies := parseCurrencyData(logger); currencies != nil {
			for _, currency := range currencies {
				db.Table("new_currency").Create(currency)
			}
			logger.Printf("Парсинг currency завершен - обновлено %d записей", len(currencies))
		} else {
			logger.Printf("Ошибка при парсинге currency")
		}
	})
	c.Start()
	log.Printf("[CURRENCY CRON] Планировщик запущен. Парсинг валют будет выполняться каждые 3 часа")
}
