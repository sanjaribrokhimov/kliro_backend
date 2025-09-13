package services

import (
	"kliro/models"
	"kliro/utils"
	"log"
	"os"

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

// Кредитные карты: страницы
var creditCardURLs = []string{
	"https://bank.uz/uz/cards/kreditnye-karty",
	"https://bank.uz/uz/cards/kreditnye-karty?PAGEN_4=2",
	"https://bank.uz/uz/cards/kreditnye-karty?PAGEN_4=3",
	"https://bank.uz/uz/cards/kreditnye-karty?PAGEN_4=4",
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
		card.CreatedAt = utils.UzbekTime()
	}
	return cards
}

// Функция для парсинга одного URL (кредитные)
func parseCreditCardURL(url string, logger *log.Logger) []*models.CreditCard {
	parser := NewCardParser()
	cards, err := parser.ParseCreditCardsURL(url)
	if err != nil {
		logger.Printf("Ошибка парсинга (credit) %s: %v", url, err)
		return nil
	}
	for _, cc := range cards {
		cc.CreatedAt = utils.UzbekTime()
	}
	return cards
}

// Инициализация данных (первый запуск)
func InitializeCardData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	logger.Printf("Инициализация данных card - очищаем базу и парсим заново...")

	// Очищаем таблицу
	db.Exec("TRUNCATE new_card")

	// Парсим все URL'ы и сохраняем в таблицу
	for _, url := range cardURLs {
		if cards := parseCardURL(url, logger); cards != nil {
			for _, card := range cards {
				card.CreatedAt = utils.UzbekTime()
				db.Table("new_card").Create(card)
			}
		}
	}

	logger.Printf("Инициализация завершена - заполнена таблица new_card")
}

// Инициализация данных кредитных карт
func InitializeCreditCardData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	logger.Printf("Инициализация данных credit_card - очищаем базу и парсим заново...")
	db.Exec("TRUNCATE new_credit_card")
	for _, url := range creditCardURLs {
		if cards := parseCreditCardURL(url, logger); cards != nil {
			for _, cc := range cards {
				cc.CreatedAt = utils.UzbekTime()
				db.Table("new_credit_card").Create(cc)
			}
		}
	}
	logger.Printf("Инициализация завершена - заполнена таблица new_credit_card")
}

func StartCardCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeCardData(db)

	c := cron.New()
	c.AddFunc("0 0 22 * * *", func() { // Каждый день в 22:00 UTC (03:00 по Узбекистану)
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало ежедневного парсинга card...")

		// Очищаем таблицу перед новым парсингом
		db.Exec("TRUNCATE new_card")

		// Парсим все URL'ы заново
		for _, url := range cardURLs {
			if cards := parseCardURL(url, logger); cards != nil {
				for _, card := range cards {
					card.CreatedAt = utils.UzbekTime()
					db.Table("new_card").Create(card)
				}
			}
		}

		logger.Printf("Ежедневный парсинг card завершен")
	})
	c.Start()
	log.Printf("[CARD CRON] Планировщик запущен. Парсинг карт будет выполняться каждый день в 03:00 UTC")
}

// Крон кредитных карт
func StartCreditCardCron(db *gorm.DB) {
	InitializeCreditCardData(db)

	c := cron.New()
	c.AddFunc("0 10 22 * * *", func() { // Каждый день в 22:10 UTC (03:10 по Узбекистану)
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало ежедневного парсинга credit_card...")
		db.Exec("TRUNCATE new_credit_card")
		for _, url := range creditCardURLs {
			if cards := parseCreditCardURL(url, logger); cards != nil {
				for _, cc := range cards {
					cc.CreatedAt = utils.UzbekTime()
					db.Table("new_credit_card").Create(cc)
				}
			}
		}
		logger.Printf("Ежедневный парсинг credit_card завершен")
	})
	c.Start()
	log.Printf("[CREDIT CARD CRON] Планировщик запущен. Парсинг кредитных карт будет выполняться каждый день в 03:10 UTC")
}
