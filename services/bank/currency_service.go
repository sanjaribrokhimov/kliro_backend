package services

import (
	"errors"
	"fmt"
	"kliro/models"
	"kliro/utils"
	"log"
	"math"
	"sort"
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
	if err := cs.db.Where("created_at < ?", utils.UzbekTime().AddDate(0, 0, -7)).Delete(&models.Currency{}).Error; err != nil {
		log.Printf("[CURRENCY SERVICE ERROR] Ошибка удаления старых записей: %v", err)
		return err
	}

	var currencies []models.Currency
	savedCount := 0
	currentTime := utils.UzbekTime() // Получаем время Узбекистана один раз

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
			// Запись существует - обновляем значения с новым timestamp (время Узбекистана)
			updates := map[string]interface{}{
				"buy_rate":   buyRate,
				"sell_rate":  sellRate,
				"updated_at": currentTime,
			}

			if err := cs.db.Model(&existingCurrency).Updates(updates).Error; err != nil {
				log.Printf("[CURRENCY SERVICE ERROR] Ошибка обновления записи для %s %s: %v", bankName, currencyType, err)
				continue
			}

			log.Printf("[CURRENCY SERVICE INFO] Обновлена запись: %s %s (buy: %.2f, sell: %v, updated_at: %v)",
				bankName, currencyType, buyRate, sellRate, currentTime.Format("2006-01-02 15:04:05"))
			savedCount++
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Записи нет - добавляем новую с временем Узбекистана
			currency := models.Currency{
				BankName:  bankName,
				Currency:  currencyType,
				BuyRate:   buyRate,
				SellRate:  sellRate,
				CreatedAt: currentTime,
				UpdatedAt: currentTime,
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
		log.Printf("[CURRENCY SERVICE] Успешно обработано %d записей валют (%d новых, %d обновлений) - время Узбекистана: %s", savedCount, len(currencies), savedCount-len(currencies), currentTime.Format("2006-01-02 15:04:05"))
	} else {
		log.Printf("[CURRENCY SERVICE] Успешно обработано %d записей валют (все обновления) - время Узбекистана: %s", savedCount, currentTime.Format("2006-01-02 15:04:05"))
	}

	return nil
}

// GetLatestCurrencyRates получает последние курсы валют из таблицы new_currency
func (cs *CurrencyService) GetLatestCurrencyRates() (map[string][]models.Currency, error) {
	var currencies []models.Currency

	if err := cs.db.Table("new_currency").Order("updated_at DESC").Find(&currencies).Error; err != nil {
		return nil, err
	}

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

// GetSplitSortedCurrencyRates возвращает по каждой валюте два списка: покупка и продажа, отсортированные
func (cs *CurrencyService) GetSplitSortedCurrencyRates() (map[string]map[string][]map[string]string, error) {
	var rows []models.Currency
	if err := cs.db.Table("new_currency").Find(&rows).Error; err != nil {
		return nil, err
	}

	// Группируем по валюте
	grouped := make(map[string][]models.Currency)
	for _, r := range rows {
		grouped[r.Currency] = append(grouped[r.Currency], r)
	}

	result := make(map[string]map[string][]map[string]string)
	for currency, list := range grouped {
		// Копии слайсов для сортировок
		buyList := make([]models.Currency, len(list))
		copy(buyList, list)
		sellList := make([]models.Currency, len(list))
		copy(sellList, list)

		// Сортируем: покупка DESC, при равенстве — по банку ASC (значения <=0 уводим в конец)
		sort.SliceStable(buyList, func(i, j int) bool {
			bi := buyList[i].BuyRate
			bj := buyList[j].BuyRate
			// невалидные (<=0) в конец
			if bi <= 0 && bj > 0 {
				return false
			}
			if bj <= 0 && bi > 0 {
				return true
			}
			if math.Abs(bi-bj) < 1e-9 {
				return buyList[i].BankName < buyList[j].BankName
			}
			return bi > bj
		})

		// Сортируем: продажа ASC (nil или <=0 -> в конец), при равенстве — по банку ASC
		sort.SliceStable(sellList, func(i, j int) bool {
			var si, sj float64
			if sellList[i].SellRate != nil {
				si = *sellList[i].SellRate
			} else {
				si = math.MaxFloat64
			}
			if sellList[j].SellRate != nil {
				sj = *sellList[j].SellRate
			} else {
				sj = math.MaxFloat64
			}
			// невалидные (<=0) в конец
			if si <= 0 {
				si = math.MaxFloat64
			}
			if sj <= 0 {
				sj = math.MaxFloat64
			}
			if math.Abs(si-sj) < 1e-9 {
				return sellList[i].BankName < sellList[j].BankName
			}
			return si < sj
		})

		// Формат времени Ташкента
		uzLoc, _ := time.LoadLocation("Asia/Tashkent")

		// Форматируем строки (и добавляем id, updated_at)
		buyFormatted := make([]map[string]string, 0, len(buyList))
		for _, it := range buyList {
			buyFormatted = append(buyFormatted, map[string]string{
				"id":         fmt.Sprintf("%d", it.ID),
				"bank":       it.BankName,
				"rate":       utils.FormatUZS(it.BuyRate),
				"updated_at": it.UpdatedAt.In(uzLoc).Format("2006-01-02 15:04:05"),
			})
		}
		sellFormatted := make([]map[string]string, 0, len(sellList))
		for _, it := range sellList {
			var sr float64
			if it.SellRate != nil {
				sr = *it.SellRate
			}
			sellFormatted = append(sellFormatted, map[string]string{
				"id":         fmt.Sprintf("%d", it.ID),
				"bank":       it.BankName,
				"rate":       utils.FormatUZS(sr),
				"updated_at": it.UpdatedAt.In(uzLoc).Format("2006-01-02 15:04:05"),
			})
		}

		result[currency] = map[string][]map[string]string{
			"buy_sorted":  buyFormatted,
			"sell_sorted": sellFormatted,
		}
	}

	return result, nil
}
