package services

import (
	"kliro/models"
	"log"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var mortgageURLs = []string{
	"https://www.infinbank.com/ru/private/credits/ipoteka/",
	"https://sqb.uz/individuals/credits/ipoteka-ru/",
	"https://aab.uz/ru/private/crediting/ipoteka/",
	"https://mkbank.uz/ru/private/crediting/mortgage/",
	"https://trustbank.uz/ru/private/crediting/ipoteka/",
	"https://hamkorbank.uz/physical/credits/ipoteka/",
	"https://ru.ipakyulibank.uz/physical/kredity/ipoteka/",
	"https://asakabank.uz/ru/physical-persons/credits/ipoteka",
	"https://xb.uz/page/ipoteka",
	"https://turonbank.uz/ru/private/crediting/ipoteka/",
}

// Функция для парсинга одного URL
func parseMortgageURL(url string, logger *log.Logger) *models.Mortgage {
	// Парсим напрямую, без обращения к API
	parser := NewMortgageParser()
	mortgage, err := parser.ParseURL(url)
	if err != nil {
		logger.Printf("Ошибка парсинга %s: %v", url, err)
		return nil
	}

	mortgage.CreatedAt = time.Now()
	return mortgage
}

// Инициализация данных (первый запуск)
func InitializeMortgageData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	// Проверяем, есть ли данные в таблицах
	var count int64
	db.Table("new_mortgage").Count(&count)

	if count == 0 {
		logger.Printf("Инициализация данных mortgage - таблицы пустые, парсим все сайты...")

		// Парсим все URL'ы и сохраняем в обе таблицы
		for _, url := range mortgageURLs {
			if mortgage := parseMortgageURL(url, logger); mortgage != nil {
				db.Table("new_mortgage").Create(mortgage)
				db.Table("old_mortgage").Create(mortgage)
			}
		}

		logger.Printf("Инициализация завершена - заполнены таблицы new_mortgage и old_mortgage")
	}
}

func StartMortgageCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeMortgageData(db)

	c := cron.New()
	c.AddFunc("0 0 20 */3 * *", func() {
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало парсинга mortgage каждые 3 дня...")

		// Копируем new_mortgage в old_mortgage
		db.Exec("TRUNCATE old_mortgage")
		db.Exec("INSERT INTO old_mortgage SELECT * FROM new_mortgage")
		db.Exec("TRUNCATE new_mortgage")

		// Парсим все URL'ы заново
		for _, url := range mortgageURLs {
			if mortgage := parseMortgageURL(url, logger); mortgage != nil {
				db.Table("new_mortgage").Create(mortgage)
			}
		}

		logger.Printf("Парсинг mortgage каждые 3 дня завершен")
	})
	c.Start()
	log.Printf("[MORTGAGE CRON] Планировщик запущен. Парсинг ипотеки будет выполняться каждые 3 дня в 20:00 UTC")
}
