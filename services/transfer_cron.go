package services

import (
	"kliro/models"
	"log"
	"strings"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type TransferCronService struct {
	db *gorm.DB
}

func NewTransferCronService(db *gorm.DB) *TransferCronService {
	return &TransferCronService{db: db}
}

// StartTransferCron запускает cron для парсинга переводов
func StartTransferCron(db *gorm.DB) {
	service := NewTransferCronService(db)

	// Инициализация данных при запуске
	service.initializeTransferData()

	// Создаем cron планировщик
	c := cron.New(cron.WithSeconds())

	// Запускаем парсинг каждые 3 дня в 20:00
	c.AddFunc("0 0 20 */3 * *", func() {
		service.parseAllTransferURLs()
	})

	c.Start()
	log.Printf("[TRANSFER CRON] Планировщик запущен. Парсинг переводов будет выполняться каждые 3 дня в 20:00 UTC")
}

// initializeTransferData инициализирует данные переводов при запуске
func (tcs *TransferCronService) initializeTransferData() {
	// Проверяем, есть ли данные в таблицах
	var newCount int64

	err1 := tcs.db.Table("new_transfer").Count(&newCount).Error
	if err1 != nil {
		log.Printf("[TRANSFER CRON] Ошибка проверки new_transfer: %v", err1)
	}

	

	
}

// parseAllTransferURLs парсит все URL переводов
func (tcs *TransferCronService) parseAllTransferURLs() {
	parser := NewTransferParser()



	// Парсим только основной URL с переводными приложениями
	transfers, err := tcs.parseTransferURL("https://bank.uz/uz/perevodi", parser)

	if err != nil {
		log.Printf("[TRANSFER CRON] Ошибка парсинга: %v", err)
	} else {
		if len(transfers) > 0 {
			// Сохраняем все переводы в базу данных с проверкой дубликатов
			savedCount := 0
			seenNames := make(map[string]bool)

			for _, transfer := range transfers {
				// Проверяем на дубликаты в базе данных
				normalizedName := strings.ToLower(strings.TrimSpace(transfer.AppName))
				if seenNames[normalizedName] {
					continue
				}

				if err := tcs.db.Table("new_transfer").Create(transfer).Error; err != nil {
					log.Printf("[TRANSFER CRON] Ошибка сохранения: %v", err)
				} else {
					savedCount++
					seenNames[normalizedName] = true
				}
			}

			log.Printf("[TRANSFER CRON] Успешно сохранено переводов: %d", savedCount)
		} else {
			log.Printf("[TRANSFER CRON] Переводы не найдены")
		}
	}
}

// parseTransferURL парсит конкретный URL перевода
func (tcs *TransferCronService) parseTransferURL(url string, parser *TransferParser) ([]*models.Transfer, error) {
	transfers, err := parser.ParseURL(url)
	if err != nil {
		return nil, err
	}

	return transfers, nil
}


