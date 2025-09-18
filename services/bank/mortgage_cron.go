package services

import (
	"kliro/models"
	"kliro/utils"
	"log"
	"os"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var mortgageURLs = []string{
	"https://bank.uz/uz/ipoteka",
	"https://bank.uz/uz/ipoteka?PAGEN_3=2",
	"https://bank.uz/uz/ipoteka?PAGEN_3=3",
	"https://bank.uz/uz/ipoteka?PAGEN_3=4",
	"https://bank.uz/uz/ipoteka?PAGEN_3=5",
	"https://bank.uz/uz/ipoteka?PAGEN_3=6",
	"https://bank.uz/uz/ipoteka?PAGEN_3=7",
}

// Функция для парсинга одного URL
func parseMortgageURL(url string, logger *log.Logger) []*models.Mortgage {
	// Парсим напрямую, без обращения к API
	parser := NewMortgageParser()
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
func InitializeMortgageData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	logger.Printf("Инициализация данных mortgage - очищаем базу и парсим заново...")

	// Очищаем таблицу
	db.Exec("TRUNCATE new_mortgage")

	// Парсим все URL'ы и сохраняем в таблицу
	for _, url := range mortgageURLs {
		if credits := parseMortgageURL(url, logger); credits != nil {
			for _, credit := range credits {
				credit.CreatedAt = utils.UzbekTime()
				db.Table("new_mortgage").Create(credit)
			}
		}
	}

	logger.Printf("Инициализация завершена - заполнена таблица new_mortgage")
}

func StartMortgageCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeMortgageData(db)

	c := cron.New()
	c.AddFunc("0 5 22 * * *", func() { // Каждый день в 22:05 UTC (03:05 по Узбекистану)
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало ежедневного парсинга mortgage...")

		// Очищаем таблицу перед новым парсингом
		db.Exec("TRUNCATE new_mortgage")

		// Парсим все URL'ы заново
		for _, url := range mortgageURLs {
			if credits := parseMortgageURL(url, logger); credits != nil {
				for _, credit := range credits {
					credit.CreatedAt = utils.UzbekTime()
					db.Table("new_mortgage").Create(credit)
				}
			}
		}

		logger.Printf("Ежедневный парсинг mortgage завершен")
	})
	c.Start()
	log.Printf("[MORTGAGE CRON] Планировщик запущен. Парсинг ипотеки будет выполняться каждый день в 03:05 UTC")
}
