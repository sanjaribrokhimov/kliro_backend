package services

import (
	"kliro/models"
	"log"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var cardURLs = []string{
	"https://bank.uz/uz/cards",
	"https://bank.uz/uz/cards?PAGEN_4=2",
	"https://bank.uz/uz/cards?PAGEN_4=3",
	"https://bank.uz/uz/cards?PAGEN_4=4",
	"https://bank.uz/uz/cards?PAGEN_4=5",
	"https://bank.uz/uz/cards?PAGEN_4=6",
	"https://bank.uz/uz/cards?PAGEN_4=7",
	"https://bank.uz/uz/cards?PAGEN_4=8",
	"https://bank.uz/uz/cards?PAGEN_4=9",
	"https://bank.uz/uz/cards?PAGEN_4=10",
	"https://bank.uz/uz/cards?PAGEN_4=11",
	"https://bank.uz/uz/cards?PAGEN_4=12",
	"https://bank.uz/uz/cards?PAGEN_4=13",
	"https://bank.uz/uz/cards?PAGEN_4=14",
	"https://bank.uz/uz/cards?PAGEN_4=15",
	"https://bank.uz/uz/cards?PAGEN_4=16",
	"https://bank.uz/uz/cards?PAGEN_4=17",
	"https://bank.uz/uz/cards?PAGEN_4=18",
	"https://bank.uz/uz/cards?PAGEN_4=19",
	"https://bank.uz/uz/cards?PAGEN_4=20",
}

// Функция для парсинга одного URL
func parseCardURL(url string, logger *log.Logger) []*models.Card {
	// Парсим напрямую, без обращения к API
	parser := NewCardParser()
	cards, err := parser.ParseURL(url)
	if err != nil {
		logger.Printf("Ошибка парсинга %s: %v", url, err)
		return nil
	}

	// Устанавливаем время создания для всех карт
	for _, card := range cards {
		card.CreatedAt = time.Now()
	}
	return cards
}

// Инициализация данных (первый запуск)
func InitializeCardData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	logger.Printf("Инициализация данных card - очищаем базу и парсим заново...")

	// Очищаем обе таблицы
	db.Exec("TRUNCATE new_card")
	db.Exec("TRUNCATE old_card")

	// Парсим все URL'ы и сохраняем в обе таблицы
	for _, url := range cardURLs {
		if cards := parseCardURL(url, logger); cards != nil {
			for _, card := range cards {
				db.Table("new_card").Create(card)
				db.Table("old_card").Create(card)
			}
		}
	}

	logger.Printf("Инициализация завершена - заполнены таблицы new_card и old_card")
}

func StartCardCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeCardData(db)

	c := cron.New()
	c.AddFunc("0 0 3 * * *", func() { // Каждый день в 03:00 UTC (ночь)
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало ежедневного парсинга card...")

		// Копируем new_card в old_card
		db.Exec("TRUNCATE old_card")
		db.Exec("INSERT INTO old_card SELECT * FROM new_card")
		db.Exec("TRUNCATE new_card")

		// Парсим все URL'ы заново
		for _, url := range cardURLs {
			if cards := parseCardURL(url, logger); cards != nil {
				for _, card := range cards {
					db.Table("new_card").Create(card)
				}
			}
		}

		logger.Printf("Ежедневный парсинг card завершен")
	})
	c.Start()
	log.Printf("[CARD CRON] Планировщик запущен. Парсинг карт будет выполняться каждый день в 03:00 UTC")
}
