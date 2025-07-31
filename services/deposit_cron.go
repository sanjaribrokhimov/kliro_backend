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
	"https://www.infinbank.com/ru/private/deposits/",
	"https://sqb.uz/individuals/deposits/",
	"https://aab.uz/ru/private/deposits/",
	"https://mkbank.uz/ru/private/deposits/",
	"https://trustbank.uz/ru/private/deposits/",
	"https://hamkorbank.uz/physical/deposits/",
	"https://ru.ipakyulibank.uz/physical/vklady/",
	"https://asakabank.uz/ru/physical-persons/deposits/",
	"https://xb.uz/page/vklady",
	"https://turonbank.uz/ru/private/deposits/",
}

// Функция для парсинга одного URL
func parseDepositURL(url string, logger *log.Logger) *models.Deposit {
	// Парсим напрямую, без обращения к API
	parser := NewDepositParser()
	deposit, err := parser.ParseURL(url)
	if err != nil {
		logger.Printf("Ошибка парсинга %s: %v", url, err)
		return nil
	}

	deposit.CreatedAt = time.Now()
	return deposit
}

// Инициализация данных (первый запуск)
func InitializeDepositData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	// Проверяем, есть ли данные в таблицах
	var count int64
	db.Table("new_deposit").Count(&count)

	if count == 0 {
		logger.Printf("Инициализация данных deposit - таблицы пустые, парсим все сайты...")

		// Парсим все URL'ы и сохраняем в обе таблицы
		for _, url := range depositURLs {
			if deposit := parseDepositURL(url, logger); deposit != nil {
				db.Table("new_deposit").Create(deposit)
				db.Table("old_deposit").Create(deposit)
			}
		}

		logger.Printf("Инициализация завершена - заполнены таблицы new_deposit и old_deposit")
	}
}

func StartDepositCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeDepositData(db)

	c := cron.New()
	c.AddFunc("0 0 20 */3 * *", func() {
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало парсинга deposit каждые 3 дня...")

		// Копируем new_deposit в old_deposit
		db.Exec("TRUNCATE old_deposit")
		db.Exec("INSERT INTO old_deposit SELECT * FROM new_deposit")
		db.Exec("TRUNCATE new_deposit")

		// Парсим все URL'ы заново
		for _, url := range depositURLs {
			if deposit := parseDepositURL(url, logger); deposit != nil {
				db.Table("new_deposit").Create(deposit)
			}
		}

		logger.Printf("Парсинг deposit каждые 3 дня завершен")
	})
	c.Start()
	log.Printf("[DEPOSIT CRON] Планировщик запущен. Парсинг вкладов будет выполняться каждые 3 дня в 20:00 UTC")
}
