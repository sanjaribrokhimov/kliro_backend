package services

import (
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

	// Удаляем старые записи (старше 1 дня)
	if err := cs.db.Where("created_at < ?", time.Now().AddDate(0, 0, -1)).Delete(&models.Currency{}).Error; err != nil {
		log.Printf("[CURRENCY SERVICE ERROR] Ошибка удаления старых записей: %v", err)
		return err
	}

	var currencies []models.Currency
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

		currency := models.Currency{
			BankName:  bankName,
			Currency:  currencyType,
			BuyRate:   buyRate,
			SellRate:  sellRate,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		currencies = append(currencies, currency)
	}

	if len(currencies) > 0 {
		if err := cs.db.Create(&currencies).Error; err != nil {
			log.Printf("[CURRENCY SERVICE ERROR] Ошибка сохранения курсов: %v", err)
			return err
		}
		log.Printf("[CURRENCY SERVICE] Успешно сохранено %d курсов валют", len(currencies))
	} else {
		log.Printf("[CURRENCY SERVICE WARNING] Нет данных для сохранения")
	}

	return nil
}

// GetLatestCurrencyRates получает последние курсы валют
func (cs *CurrencyService) GetLatestCurrencyRates() (map[string][]models.Currency, error) {
	var currencies []models.Currency

	// Получаем курсы за последние 24 часа
	if err := cs.db.Where("created_at >= ?", time.Now().AddDate(0, 0, -1)).Find(&currencies).Error; err != nil {
		return nil, err
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
