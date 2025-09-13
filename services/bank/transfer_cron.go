package services

import (
	"kliro/models"
	"kliro/utils"
	"log"
	"os"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var transferURLs = []string{
	"https://bank.uz/uz/perevodi",
}

// Функция для парсинга одного URL переводов
func parseTransferURL(url string, logger *log.Logger) []*models.Transfer {
	// Парсим напрямую, без обращения к API
	parser := NewTransferParser()
	transfers, err := parser.ParseURL(url)
	if err != nil {
		logger.Printf("Ошибка парсинга переводов %s: %v", url, err)
		return nil
	}

	// Устанавливаем время создания для всех переводов
	for _, transfer := range transfers {
		transfer.CreatedAt = utils.UzbekTime()
	}
	return transfers
}

// Инициализация данных переводов (первый запуск)
func InitializeTransferData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	logger.Printf("Инициализация данных переводов - очищаем базу и парсим заново...")

	// Очищаем таблицу переводов
	db.Exec("TRUNCATE new_transfer")

	// Парсим все URL'ы и сохраняем в таблицу
	for _, url := range transferURLs {
		if transfers := parseTransferURL(url, logger); transfers != nil {
			for _, transfer := range transfers {
				transfer.CreatedAt = utils.UzbekTime()
				db.Table("new_transfer").Create(transfer)
			}
		}
	}

	logger.Printf("Инициализация переводов завершена - заполнена таблица new_transfer")
}

// Запуск cron для переводов
func StartTransferCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeTransferData(db)

	c := cron.New()
	c.AddFunc("0 0 21 * * *", func() { // Каждый день в 21:00 UTC (02:00 по Узбекистану)
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало ежедневного парсинга переводов...")

		// Очищаем таблицу переводов
		db.Exec("TRUNCATE new_transfer")

		// Парсим все URL'ы заново
		for _, url := range transferURLs {
			if transfers := parseTransferURL(url, logger); transfers != nil {
				for _, transfer := range transfers {
					transfer.CreatedAt = utils.UzbekTime()
					db.Table("new_transfer").Create(transfer)
				}
			}
		}

		logger.Printf("Ежедневный парсинг переводов завершен")
	})
	c.Start()
	log.Printf("[TRANSFER CRON] Планировщик запущен. Парсинг переводов будет выполняться каждый день в 02:00 UTC")
}
