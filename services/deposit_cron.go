package services

import (
	"kliro/models"
	"log"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var depositURLs = []string{
	"https://bank.uz/uz/deposits",
	"https://bank.uz/uz/deposits?PAGEN_4=2",
	"https://bank.uz/uz/deposits?PAGEN_4=3",
	"https://bank.uz/uz/deposits?PAGEN_4=4",
	"https://bank.uz/uz/deposits?PAGEN_4=5",
	"https://bank.uz/uz/deposits?PAGEN_4=6",
	"https://bank.uz/uz/deposits?PAGEN_4=7",
	"https://bank.uz/uz/deposits?PAGEN_4=8",
	"https://bank.uz/uz/deposits?PAGEN_4=9",
	"https://bank.uz/uz/deposits?PAGEN_4=10",
	"https://bank.uz/uz/deposits?PAGEN_4=11",
	"https://bank.uz/uz/deposits?PAGEN_4=12",
	"https://bank.uz/uz/deposits?PAGEN_4=13",

}

// Функция для парсинга одного URL
func parseDepositURL(url string, logger *log.Logger) []*models.Deposit {
	// Парсим напрямую, без обращения к API
	parser := NewDepositParser()
	deposits, err := parser.ParseURL(url)
	if err != nil {
		logger.Printf("Ошибка парсинга %s: %v", url, err)
		return nil
	}

	// Устанавливаем время создания для всех вкладов
	for _, deposit := range deposits {
		deposit.CreatedAt = time.Now()
	}
	return deposits
}

// Инициализация данных (первый запуск)
func InitializeDepositData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	logger.Printf("Инициализация данных deposit - очищаем базу и парсим заново...")

	// Очищаем обе таблицы
	db.Exec("TRUNCATE new_deposit")
	db.Exec("TRUNCATE old_deposit")

	// Парсим все URL'ы и сохраняем в обе таблицы
	for _, url := range depositURLs {
		if deposits := parseDepositURL(url, logger); deposits != nil {
			for _, deposit := range deposits {
				db.Table("new_deposit").Create(deposit)
				db.Table("old_deposit").Create(deposit)
			}
		}
	}

	logger.Printf("Инициализация завершена - заполнены таблицы new_deposit и old_deposit")
}

func StartDepositCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeDepositData(db)

	c := cron.New()
	c.AddFunc("0 0 2 * * *", func() { // Каждый день в 02:00 UTC (ночь)
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало ежедневного парсинга deposit...")

		// Копируем new_deposit в old_deposit
		db.Exec("TRUNCATE old_deposit")
		db.Exec("INSERT INTO old_deposit SELECT * FROM new_deposit")
		db.Exec("TRUNCATE new_deposit")

		// Парсим все URL'ы заново
		for _, url := range depositURLs {
			if deposits := parseDepositURL(url, logger); deposits != nil {
				for _, deposit := range deposits {
					db.Table("new_deposit").Create(deposit)
				}
			}
		}

		logger.Printf("Ежедневный парсинг deposit завершен")
	})
	c.Start()
	log.Printf("[DEPOSIT CRON] Планировщик запущен. Парсинг вкладов будет выполняться каждый день в 02:00 UTC")
}
