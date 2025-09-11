package services

import (
	"kliro/models"
	"log"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var microcreditURLs = []string{
	"https://bank.uz/uz/credits/mikrozaymy",
	"https://bank.uz/uz/credits/mikrozaymy?PAGEN_3=2",
	"https://bank.uz/uz/credits/mikrozaymy?PAGEN_3=3",
	"https://bank.uz/uz/credits/mikrozaymy?PAGEN_3=4",
	"https://bank.uz/uz/credits/mikrozaymy?PAGEN_3=5",
	"https://bank.uz/uz/credits/mikrozaymy?PAGEN_3=6",
	"https://bank.uz/uz/credits/mikrozaymy?PAGEN_3=7",
	"https://bank.uz/uz/credits/mikrozaymy?PAGEN_3=8",
}

// Функция для парсинга одного URL
func parseMicrocreditURL(url string, logger *log.Logger) []*models.Microcredit {
	// Парсим напрямую, без обращения к API
	parser := NewMicrocreditParser()
	credits, err := parser.ParseURL(url)
	if err != nil {
		logger.Printf("Ошибка парсинга %s: %v", url, err)
		return nil
	}

	// Устанавливаем время создания для всех кредитов
	for _, credit := range credits {
		credit.CreatedAt = time.Now()
	}
	return credits
}

// Инициализация данных (первый запуск)
func InitializeMicrocreditData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	logger.Printf("Инициализация данных microcredit - очищаем базу и парсим заново...")

	// Очищаем обе таблицы
	db.Exec("TRUNCATE new_microcredit")

	// Парсим все URL'ы и сохраняем в обе таблицы
	for _, url := range microcreditURLs {
		if credits := parseMicrocreditURL(url, logger); credits != nil {
			for _, credit := range credits {
				db.Table("new_microcredit").Create(credit)
			}
		}
	}

}

func StartMicrocreditCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeMicrocreditData(db)

	c := cron.New()
	c.AddFunc("0 0 3 * * *", func() { // Каждый день в 03:00 UTC
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало ежедневного парсинга microcredit...")

		db.Exec("TRUNCATE new_microcredit")

		// Парсим все URL'ы заново
		for _, url := range microcreditURLs {
			if credits := parseMicrocreditURL(url, logger); credits != nil {
				for _, credit := range credits {
					db.Table("new_microcredit").Create(credit)
				}
			}
		}

		logger.Printf("Ежедневный парсинг microcredit завершен")
	})
	c.Start()
	log.Printf("[MICROCREDIT CRON] Планировщик запущен. Парсинг микрокредитов будет выполняться каждый день в 03:00 UTC")
}
