package services

import (
	"errors"
	"kliro/models"
	"log"
	"time"

	"gorm.io/gorm"
)

type CurrencyService struct {
	db *gorm.DB
}

func NewCurrencyService(db *gorm.DB) *CurrencyService {
	return &CurrencyService{db: db}
}

// SaveCurrencyRates сохраняет курсы валют в БД
func (cs *CurrencyService) SaveCurrencyRates(rates []map[string]interface{}) error {
	log.Printf("[CURRENCY SERVICE] Начинаем сохранение %d курсов валют", len(rates))

	// Удаляем старые записи (старше 7 дней)
	if err := cs.db.Where("created_at < ?", time.Now().AddDate(0, 0, -7)).Delete(&models.Currency{}).Error; err != nil {
		log.Printf("[CURRENCY SERVICE ERROR] Ошибка удаления старых записей: %v", err)
		return err
	}

	var currencies []models.Currency
	savedCount := 0

	for _, rate := range rates {
		bankName, ok := rate["bank"].(string)
		if !ok {
			log.Printf("[CURRENCY SERVICE WARNING] Пропускаем запись без bank_name")
			continue
		}

		currencyType, ok := rate["currency"].(string)
		if !ok {
			log.Printf("[CURRENCY SERVICE WARNING] Пропускаем запись без currency для банка %s", bankName)
			continue
		}

		buyRate, ok := rate["buy"].(float64)
		if !ok {
			log.Printf("[CURRENCY SERVICE WARNING] Пропускаем запись без buy_rate для банка %s", bankName)
			continue
		}

		var sellRate *float64
		if sell, exists := rate["sell"]; exists && sell != nil {
			if sellFloat, ok := sell.(float64); ok {
				sellRate = &sellFloat
			}
		}

		// Проверяем, есть ли уже запись для этого банка и валюты
		var existingCurrency models.Currency
		err := cs.db.Where("bank_name = ? AND currency = ?", bankName, currencyType).First(&existingCurrency).Error

		if err == nil {
			// Запись существует - обновляем значения с новым timestamp
			updates := map[string]interface{}{
				"buy_rate":   buyRate,
				"sell_rate":  sellRate,
				"updated_at": time.Now(),
			}

			if err := cs.db.Model(&existingCurrency).Updates(updates).Error; err != nil {
				log.Printf("[CURRENCY SERVICE ERROR] Ошибка обновления записи для %s %s: %v", bankName, currencyType, err)
				continue
			}

			log.Printf("[CURRENCY SERVICE INFO] Обновлена запись: %s %s (buy: %.2f, sell: %v, updated_at: %v)",
				bankName, currencyType, buyRate, sellRate, time.Now().Format("2006-01-02 15:04:05"))
			savedCount++
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Записи нет - добавляем новую
			currency := models.Currency{
				BankName:  bankName,
				Currency:  currencyType,
				BuyRate:   buyRate,
				SellRate:  sellRate,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			currencies = append(currencies, currency)
			savedCount++
		} else {
			// Произошла ошибка при поиске
			log.Printf("[CURRENCY SERVICE ERROR] Ошибка поиска существующей записи: %v", err)
			continue
		}
	}

	if len(currencies) > 0 {
		if err := cs.db.Create(&currencies).Error; err != nil {
			log.Printf("[CURRENCY SERVICE ERROR] Ошибка сохранения курсов: %v", err)
			return err
		}
		log.Printf("[CURRENCY SERVICE] Успешно обработано %d записей валют (%d новых, %d обновлений)", savedCount, len(currencies), savedCount-len(currencies))
	} else {
		log.Printf("[CURRENCY SERVICE] Успешно обработано %d записей валют (все обновления)", savedCount)
	}

	return nil
}

// GetLatestCurrencyRates получает последние курсы валют из основной таблицы currencies
func (cs *CurrencyService) GetLatestCurrencyRates() (map[string][]models.Currency, error) {
	var currencies []models.Currency

	// Получаем курсы из основной таблицы currencies, отсортированные по дате обновления
	if err := cs.db.Order("updated_at DESC").Find(&currencies).Error; err != nil {
		// Если основная таблица пустая, пробуем получить из new_currency
		log.Printf("[CURRENCY SERVICE] Основная таблица пустая, пробуем получить из new_currency")
		if err := cs.db.Table("new_currency").Find(&currencies).Error; err != nil {
			return nil, err
		}
	}

	// Группируем по валютам
	result := make(map[string][]models.Currency)
	for _, currency := range currencies {
		result[currency.Currency] = append(result[currency.Currency], currency)
	}

	return result, nil
}

// GetCurrencyRatesByDate получает курсы валют за определенную дату
func (cs *CurrencyService) GetCurrencyRatesByDate(date time.Time) (map[string][]models.Currency, error) {
	var currencies []models.Currency

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	if err := cs.db.Where("created_at >= ? AND created_at < ?", startOfDay, endOfDay).Find(&currencies).Error; err != nil {
		return nil, err
	}

	result := make(map[string][]models.Currency)
	for _, currency := range currencies {
		result[currency.Currency] = append(result[currency.Currency], currency)
	}

	return result, nil
}

// InitializeCurrencyData инициализирует данные валют при запуске сервера
func (cs *CurrencyService) InitializeCurrencyData() error {
	log.Printf("[CURRENCY SERVICE] Проверяем наличие данных валют...")

	// Проверяем, есть ли данные в таблице
	var count int64
	if err := cs.db.Model(&models.Currency{}).Count(&count).Error; err != nil {
		log.Printf("[CURRENCY SERVICE ERROR] Ошибка проверки таблицы валют: %v", err)
		return err
	}

	if count > 0 {
		log.Printf("[CURRENCY SERVICE] В таблице уже есть %d записей валют, инициализация не требуется", count)
		return nil
	}

	log.Printf("[CURRENCY SERVICE] Таблица валют пустая, начинаем парсинг...")

	// Создаем временный парсер для инициализации
	parser := NewCurrencyParser(cs)

	// Парсим и сохраняем данные
	if err := parser.ParseAndSaveCurrencyRates(); err != nil {
		log.Printf("[CURRENCY SERVICE ERROR] Ошибка инициализации валют: %v", err)
		return err
	}

	log.Printf("[CURRENCY SERVICE] Инициализация валют завершена успешно")
	return nil
}
