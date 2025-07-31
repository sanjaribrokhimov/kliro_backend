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
	log.Println("[TRANSFER CRON] 🚀 Запуск transfer cron...")

	service := NewTransferCronService(db)
	log.Println("[TRANSFER CRON] ✅ TransferCronService создан")

	// Инициализация данных при запуске
	log.Println("[TRANSFER CRON] 🔄 Инициализация данных...")
	service.initializeTransferData()

	// Создаем cron планировщик
	c := cron.New(cron.WithSeconds())
	log.Println("[TRANSFER CRON] ✅ Cron планировщик создан")

	// Запускаем парсинг каждые 3 дня в 20:00
	c.AddFunc("0 0 20 */3 * *", func() {
		log.Println("[TRANSFER CRON] 🕘 Парсинг переводов каждые 3 дня...")
		service.parseAllTransferURLs()
	})
	log.Println("[TRANSFER CRON] ✅ Задача добавлена (каждые 3 дня в 20:00)")

	c.Start()
	log.Println("[TRANSFER CRON] ✅ Cron планировщик запущен")
}

// initializeTransferData инициализирует данные переводов при запуске
func (tcs *TransferCronService) initializeTransferData() {
	log.Println("[TRANSFER CRON] 🚀 Инициализация данных переводов...")

	// Проверяем, есть ли данные в таблицах
	var newCount int64
	var oldCount int64

	err1 := tcs.db.Table("new_transfer").Count(&newCount).Error
	if err1 != nil {
		log.Printf("[TRANSFER CRON] ❌ Ошибка проверки new_transfer: %v", err1)
	}

	err2 := tcs.db.Table("old_transfer").Count(&oldCount).Error
	if err2 != nil {
		log.Printf("[TRANSFER CRON] ❌ Ошибка проверки old_transfer: %v", err2)
	}

	log.Printf("[TRANSFER CRON] 📊 Проверка таблиц: new_transfer=%d, old_transfer=%d", newCount, oldCount)

	// Проверяем условие более подробно
	if newCount == 0 && oldCount == 0 {
		log.Println("[TRANSFER CRON] ✅ Таблицы пустые, начинаем парсинг...")
		tcs.parseAllTransferURLs()
	} else {
		log.Printf("[TRANSFER CRON] ℹ️ В таблицах уже есть данные: new_transfer=%d, old_transfer=%d", newCount, oldCount)
		log.Printf("[TRANSFER CRON] ℹ️ Парсинг при запуске НЕ требуется, ждем расписание (каждые 3 дня в 20:00)")
	}

	log.Println("[TRANSFER CRON] ✅ Инициализация завершена")
}

// parseAllTransferURLs парсит все URL переводов
func (tcs *TransferCronService) parseAllTransferURLs() {
	log.Println("[TRANSFER CRON] 🚀 Начинаем парсинг всех URL переводов...")

	parser := NewTransferParser()
	log.Println("[TRANSFER CRON] ✅ Парсер создан")

	// Перемещаем старые данные
	log.Println("[TRANSFER CRON] 🔄 Ротация данных...")
	tcs.rotateTransferData()

	// Парсим только основной URL с переводными приложениями
	log.Printf("[TRANSFER CRON] 🌐 Парсинг https://bank.uz/perevodi...")
	transfers, err := tcs.parseTransferURL("https://bank.uz/perevodi", parser)

	if err != nil {
		log.Printf("[TRANSFER CRON] ❌ Ошибка парсинга https://bank.uz/perevodi: %v", err)
	} else {
		log.Printf("[TRANSFER CRON] 📊 Получено %d переводов от парсера", len(transfers))

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
					log.Printf("[TRANSFER CRON] ❌ Ошибка сохранения перевода %s: %v", transfer.AppName, err)
				} else {
					savedCount++
					seenNames[normalizedName] = true
				}
			}
			log.Printf("[TRANSFER CRON] 📊 Итого: получено %d, сохранено %d переводов", len(transfers), savedCount)
		} else {
			log.Printf("[TRANSFER CRON] ⚠️ Парсер вернул 0 переводов!")
		}
	}

	log.Println("[TRANSFER CRON] ✅ Парсинг переводов завершен")
}

// parseTransferURL парсит конкретный URL перевода
func (tcs *TransferCronService) parseTransferURL(url string, parser *TransferParser) ([]*models.Transfer, error) {
	log.Printf("[TRANSFER CRON] 🌐 Парсинг URL: %s", url)

	transfers, err := parser.ParseURL(url)
	if err != nil {
		log.Printf("[TRANSFER CRON] ❌ Ошибка запроса %s: %v", url, err)
		return nil, err
	}

	log.Printf("[TRANSFER CRON] ✅ Успешно спарсили %d переводов с %s", len(transfers), url)
	return transfers, nil
}

// rotateTransferData перемещает данные из new_transfer в old_transfer
func (tcs *TransferCronService) rotateTransferData() {
	log.Println("[TRANSFER CRON] 🔄 Ротация данных переводов...")

	// Очищаем старую таблицу
	if err := tcs.db.Exec("DELETE FROM old_transfer").Error; err != nil {
		log.Printf("[TRANSFER CRON] ❌ Ошибка очистки old_transfer: %v", err)
		return
	}
	log.Println("[TRANSFER CRON] ✅ old_transfer очищена")

	// Копируем данные из new в old
	if err := tcs.db.Exec(`
		INSERT INTO old_transfer (app_name, commission, limit_ru, limit_uz, created_at)
		SELECT app_name, commission, limit_ru, limit_uz, created_at
		FROM new_transfer
	`).Error; err != nil {
		log.Printf("[TRANSFER CRON] ❌ Ошибка копирования в old_transfer: %v", err)
		return
	}
	log.Println("[TRANSFER CRON] ✅ Данные скопированы в old_transfer")

	// Очищаем новую таблицу
	if err := tcs.db.Exec("DELETE FROM new_transfer").Error; err != nil {
		log.Printf("[TRANSFER CRON] ❌ Ошибка очистки new_transfer: %v", err)
		return
	}
	log.Println("[TRANSFER CRON] ✅ new_transfer очищена")

	log.Println("[TRANSFER CRON] ✅ Ротация данных завершена")
}
