package services

import (
	"kliro/models"
	"kliro/utils"
	"log"
	"os"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var autocreditURLs = []string{
	"https://bank.uz/uz/credits/avtokredit",
	"https://bank.uz/uz/credits/avtokredit?PAGEN_3=2",
	"https://bank.uz/uz/credits/avtokredit?PAGEN_3=3",
	"https://bank.uz/uz/credits/avtokredit?PAGEN_3=4",
	"https://bank.uz/uz/credits/avtokredit?PAGEN_3=5",
	"https://bank.uz/uz/credits/avtokredit?PAGEN_3=6",
	"https://bank.uz/uz/credits/avtokredit?PAGEN_3=7",
	"https://bank.uz/uz/credits/avtokredit?PAGEN_3=8",
}

// Функция для парсинга одного URL
func parseAutocreditURL(url string, logger *log.Logger) []*models.Autocredit {
	// Парсим напрямую, без обращения к API
	parser := NewAutocreditParser()
	credits, err := parser.ParseURL(url)
	if err != nil {
		logger.Printf("Ошибка парсинга %s: %v", url, err)
		return nil
	}

	// Устанавливаем время создания для всех кредитов
	for _, credit := range credits {
		credit.CreatedAt = utils.UzbekTime()
	}
	return credits
}

// Инициализация данных (первый запуск)
func InitializeAutocreditData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	logger.Printf("Инициализация данных autocredit - очищаем базу и парсим заново...")

	// Очищаем обе таблицы
	db.Exec("TRUNCATE new_autocredit")

	// Парсим все URL'ы и сохраняем в обе таблицы
	for _, url := range autocreditURLs {
		if credits := parseAutocreditURL(url, logger); credits != nil {
			for _, credit := range credits {
				credit.CreatedAt = utils.UzbekTime()
				db.Table("new_autocredit").Create(credit)
			}
		}
	}

	logger.Printf("Инициализация завершена - заполнены таблицы new_autocredit ")
}

func StartAutocreditCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeAutocreditData(db)

	c := cron.New()
	c.AddFunc("0 0 22 * * *", func() { // Каждый день в 22:00 UTC (03:00 по Узбекистану)
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало ежедневного парсинга autocredit...")

		// Копируем new_autocredi
		db.Exec("TRUNCATE new_autocredit")

		// Парсим все URL'ы заново
		for _, url := range autocreditURLs {
			if credits := parseAutocreditURL(url, logger); credits != nil {
				for _, credit := range credits {
					credit.CreatedAt = utils.UzbekTime()
					db.Table("new_autocredit").Create(credit)
				}
			}
		}

		logger.Printf("Ежедневный парсинг autocredit завершен")
	})
	c.Start()
	log.Printf("[AUTOCREDIT CRON] Планировщик запущен. Парсинг автокредитов будет выполняться каждый день в 03:00 UTC")
}
