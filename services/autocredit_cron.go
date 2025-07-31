package services

import (
	"kliro/models"
	"log"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var autocreditURLs = []string{
	"https://www.infinbank.com/ru/private/credits/avto_credit/",
	"https://sqb.uz/individuals/credits/avtokredit-ru/",
	"https://aab.uz/ru/private/crediting/avtokredit-vtorichnyy-rynok-/",
	"https://mkbank.uz/ru/private/crediting/car-loan/",
	"https://trustbank.uz/ru/private/crediting/auto/",
	"https://hamkorbank.uz/physical/credits/autolight/",
	"https://ru.ipakyulibank.uz/physical/kredity/avtokredity/avtokredit-uzauto",
	"https://asakabank.uz/ru/physical-persons/credits/avtokredit-25",
	"https://xb.uz/page/tezkor-avtokredit",
	"https://turonbank.uz/ru/private/crediting/avtokredit-imkoniyat/",
}

// Функция для парсинга одного URL
func parseAutocreditURL(url string, logger *log.Logger) *models.Autocredit {
	// Парсим напрямую, без обращения к API
	parser := NewAutocreditParser()
	credit, err := parser.ParseURL(url)
	if err != nil {
		logger.Printf("Ошибка парсинга %s: %v", url, err)
		return nil
	}

	credit.CreatedAt = time.Now()
	return credit
}

// Инициализация данных (первый запуск)
func InitializeAutocreditData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	// Проверяем, есть ли данные в таблицах
	var count int64
	db.Table("new_autocredit").Count(&count)

	if count == 0 {
		logger.Printf("Инициализация данных autocredit - таблицы пустые, парсим все сайты...")

		// Парсим все URL'ы и сохраняем в обе таблицы
		for _, url := range autocreditURLs {
			if credit := parseAutocreditURL(url, logger); credit != nil {
				db.Table("new_autocredit").Create(credit)
				db.Table("old_autocredit").Create(credit)
			}
		}

		logger.Printf("Инициализация завершена - заполнены таблицы new_autocredit и old_autocredit")
	}
}

func StartAutocreditCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeAutocreditData(db)

	c := cron.New()
	c.AddFunc("0 0 20 */3 * *", func() {
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало парсинга autocredit каждые 3 дня...")

		// Копируем new_autocredit в old_autocredit
		db.Exec("TRUNCATE old_autocredit")
		db.Exec("INSERT INTO old_autocredit SELECT * FROM new_autocredit")
		db.Exec("TRUNCATE new_autocredit")

		// Парсим все URL'ы заново
		for _, url := range autocreditURLs {
			if credit := parseAutocreditURL(url, logger); credit != nil {
				db.Table("new_autocredit").Create(credit)
			}
		}

		logger.Printf("Парсинг autocredit каждые 3 дня завершен")
	})

	c.Start()
	log.Printf("[AUTOCREDIT CRON] Планировщик запущен. Парсинг автокредитов будет выполняться каждые 3 дня в 20:00 UTC")
}
