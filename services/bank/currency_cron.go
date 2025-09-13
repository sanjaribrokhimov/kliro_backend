package services

import (
	"errors"
	"kliro/models"
	"kliro/utils"
	"log"
	"net/http"
	"os"

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
	currentTime := utils.UzbekTime() // Получаем время Узбекистана один раз

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
			CreatedAt: utils.UzbekTime(),
			UpdatedAt: utils.UzbekTime(),
		})
	}

	logger.Printf("Парсинг валют завершен - найдено %d курсов (время Узбекистана: %s)", len(currencies), currentTime.Format("2006-01-02 15:04:05"))
	return currencies
}

// Функция для обновления курсов валют в БД
func updateCurrencyRates(db *gorm.DB, currencies []*models.Currency, logger *log.Logger) {
	if len(currencies) == 0 {
		logger.Printf("Нет данных для обновления")
		return
	}

	updatedCount := 0
	newCount := 0
	currentTime := utils.UzbekTime() // Получаем время Узбекистана один раз

	for _, currency := range currencies {
		// Проверяем, есть ли уже запись для этого банка и валюты
		var existingCurrency models.Currency
		err := db.Where("bank_name = ? AND currency = ?", currency.BankName, currency.Currency).First(&existingCurrency).Error

		if err == nil {
			// Запись существует - обновляем значения с новым timestamp (время Узбекистана)
			updates := map[string]interface{}{
				"buy_rate":   currency.BuyRate,
				"sell_rate":  currency.SellRate,
				"updated_at": currentTime,
			}

			if err := db.Model(&existingCurrency).Updates(updates).Error; err != nil {
				logger.Printf("Ошибка обновления записи для %s %s: %v", currency.BankName, currency.Currency, err)
				continue
			}

			updatedCount++
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Записи нет - добавляем новую с временем Узбекистана
			currency.CreatedAt = utils.UzbekTime()
			currency.UpdatedAt = utils.UzbekTime()

			if err := db.Create(currency).Error; err != nil {
				logger.Printf("Ошибка создания записи для %s %s: %v", currency.BankName, currency.Currency, err)
				continue
			}
			newCount++
		} else {
			// Произошла ошибка при поиске
			logger.Printf("Ошибка поиска существующей записи: %v", err)
			continue
		}
	}

	logger.Printf("Обновление валют завершено: %d новых, %d обновлений (время Узбекистана: %s)", newCount, updatedCount, currentTime.Format("2006-01-02 15:04:05"))
}

// Инициализация данных (первый запуск)
func InitializeCurrencyData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	logger.Printf("Инициализация данных currency - первичная загрузка new_currency...")
	// Очищаем таблицу new_currency перед загрузкой
	db.Exec("TRUNCATE new_currency")

	// Парсим валюты и сохраняем в обе таблицы
	if currencies := parseCurrencyData(logger); currencies != nil {
		// Полностью перезаписываем new_currency актуальными значениями
		db.Exec("TRUNCATE new_currency")
		for _, currency := range currencies {
			// Проставляем время Узбекистана на каждый insert
			currency.CreatedAt = utils.UzbekTime()
			currency.UpdatedAt = utils.UzbekTime()
			db.Table("new_currency").Create(currency)
		}
		logger.Printf("Инициализация завершена - заполнена таблица new_currency (Asia/Tashkent)")
	} else {
		logger.Printf("Ошибка при парсинге валют")
	}
}

func StartCurrencyCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeCurrencyData(db)

	c := cron.New()
	c.AddFunc("0 0 */3 * * *", func() { // Каждые 3 часа (время UTC, но данные с узбекским временем)
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало парсинга currency (каждые 3 часа)...")

		// Парсим валюты заново
		if currencies := parseCurrencyData(logger); currencies != nil {
			// Полностью перезаписываем new_currency с временем Asia/Tashkent
			db.Exec("TRUNCATE new_currency")
			for _, currency := range currencies {
				currency.CreatedAt = utils.UzbekTime()
				currency.UpdatedAt = utils.UzbekTime()
				db.Table("new_currency").Create(currency)
			}
			logger.Printf("Парсинг currency завершен - обновлено %d записей в new_currency (Asia/Tashkent)", len(currencies))
		} else {
			logger.Printf("Ошибка при парсинге currency")
		}
	})
	c.Start()
	log.Printf("[CURRENCY CRON] Планировщик запущен. Парсинг валют будет выполняться каждые 3 часа")
}
