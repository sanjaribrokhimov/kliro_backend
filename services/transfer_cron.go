package services

import (
	"kliro/models"
	"log"

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

	// Запускаем парсинг каждый день в 9:00
	c.AddFunc("0 0 9 * * *", func() {
		log.Println("[TRANSFER CRON] Начинаем ежедневный парсинг переводов...")
		service.parseAllTransferURLs()
	})

	c.Start()
	log.Println("[TRANSFER CRON] Cron планировщик запущен")
}

// initializeTransferData инициализирует данные переводов при запуске
func (tcs *TransferCronService) initializeTransferData() {
	log.Println("[TRANSFER CRON] Инициализация данных переводов...")

	// Проверяем, есть ли данные в таблицах
	var newCount int64
	var oldCount int64

	tcs.db.Table("new_transfer").Count(&newCount)
	tcs.db.Table("old_transfer").Count(&oldCount)

	if newCount == 0 && oldCount == 0 {
		log.Println("[TRANSFER CRON] Таблицы пустые, парсим все сайты...")
		tcs.parseAllTransferURLs()
	} else {
		log.Printf("[TRANSFER CRON] В таблицах уже есть данные: new_transfer=%d, old_transfer=%d", newCount, oldCount)
	}

	log.Println("[TRANSFER CRON] Инициализация завершена")
}

// parseAllTransferURLs парсит все URL переводов
func (tcs *TransferCronService) parseAllTransferURLs() {
	parser := NewTransferParser()

	// Перемещаем старые данные
	tcs.rotateTransferData()

	// Парсим основной URL с переводными приложениями
	log.Printf("[TRANSFER CRON] Начинаем парсинг https://bank.uz/perevodi...")
	transfers, err := tcs.parseTransferURL("https://bank.uz/perevodi", parser)

	// Пробуем дополнительные URL
	log.Printf("[TRANSFER CRON] Начинаем парсинг https://bank.uz/uz/perevodi...")
	transfers2, err2 := tcs.parseTransferURL("https://bank.uz/uz/perevodi", parser)
	if err2 == nil && len(transfers2) > 0 {
		transfers = append(transfers, transfers2...)
		log.Printf("[TRANSFER CRON] Добавлено %d переводов с второго URL", len(transfers2))
	}
	if err != nil {
		log.Printf("[TRANSFER CRON ERROR] Ошибка парсинга https://bank.uz/perevodi: %v", err)
	} else {
		log.Printf("[TRANSFER CRON] Получено %d переводов от парсера", len(transfers))

		if len(transfers) > 0 {
			// Сохраняем все переводы в базу данных
			savedCount := 0
			for i, transfer := range transfers {
				log.Printf("[TRANSFER CRON] Сохраняем перевод %d/%d: %s", i+1, len(transfers), transfer.AppName)

				if err := tcs.db.Table("new_transfer").Create(transfer).Error; err != nil {
					log.Printf("[TRANSFER CRON ERROR] Ошибка сохранения перевода %s: %v", transfer.AppName, err)
				} else {
					log.Printf("[TRANSFER CRON] ✅ Успешно сохранен перевод: %s", transfer.AppName)
					savedCount++
				}
			}
			log.Printf("[TRANSFER CRON] 📊 Итого: получено %d, сохранено %d переводов", len(transfers), savedCount)
		} else {
			log.Printf("[TRANSFER CRON] ⚠️ Парсер вернул 0 переводов!")
		}
	}

	log.Println("[TRANSFER CRON] Парсинг переводов завершен")
}

// parseTransferURL парсит конкретный URL перевода
func (tcs *TransferCronService) parseTransferURL(url string, parser *TransferParser) ([]*models.Transfer, error) {
	log.Printf("[TRANSFER CRON] Парсинг URL: %s", url)

	transfers, err := parser.ParseURL(url)
	if err != nil {
		log.Printf("[TRANSFER CRON ERROR] Ошибка запроса %s: %v", url, err)
		return nil, err
	}

	return transfers, nil
}

// rotateTransferData перемещает данные из new_transfer в old_transfer
func (tcs *TransferCronService) rotateTransferData() {
	log.Println("[TRANSFER CRON] Ротация данных переводов...")

	// Очищаем старую таблицу
	if err := tcs.db.Exec("DELETE FROM old_transfer").Error; err != nil {
		log.Printf("[TRANSFER CRON ERROR] Ошибка очистки old_transfer: %v", err)
		return
	}

	// Копируем данные из new в old
	if err := tcs.db.Exec(`
		INSERT INTO old_transfer (app_name, commission, limit_ru, limit_uz, created_at)
		SELECT app_name, commission, limit_ru, limit_uz, created_at 
		FROM new_transfer
	`).Error; err != nil {
		log.Printf("[TRANSFER CRON ERROR] Ошибка копирования в old_transfer: %v", err)
		return
	}

	// Очищаем новую таблицу
	if err := tcs.db.Exec("DELETE FROM new_transfer").Error; err != nil {
		log.Printf("[TRANSFER CRON ERROR] Ошибка очистки new_transfer: %v", err)
		return
	}

	log.Println("[TRANSFER CRON] Ротация данных завершена")
}
