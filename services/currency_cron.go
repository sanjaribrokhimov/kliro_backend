package services

import (
	"log"
	"os"
	"time"

	"github.com/robfig/cron/v3"
)

const DEEPSEEK_API_URL = "https://api.deepseek.com/v1/chat/completions"

type CurrencyCronService struct {
	currencyService *CurrencyService
	cron            *cron.Cron
}

func NewCurrencyCronService(currencyService *CurrencyService) *CurrencyCronService {
	return &CurrencyCronService{
		currencyService: currencyService,
		cron:            cron.New(cron.WithLocation(time.UTC)),
	}
}

// Start запускает планировщик
func (ccs *CurrencyCronService) Start() {
	// Запускаем парсинг каждый день в 20:00 по UTC
	ccs.cron.AddFunc("0 20 * * *", ccs.ParseAndSaveCurrencyRates)

	// Запускаем планировщик
	ccs.cron.Start()
	log.Printf("[CURRENCY CRON] Планировщик запущен. Парсинг валют будет выполняться каждый день в 20:00 UTC")
}

// Stop останавливает планировщик
func (ccs *CurrencyCronService) Stop() {
	ccs.cron.Stop()
	log.Printf("[CURRENCY CRON] Планировщик остановлен")
}

// ParseAndSaveCurrencyRates парсит и сохраняет курсы валют
func (ccs *CurrencyCronService) ParseAndSaveCurrencyRates() {
	// Настраиваем логирование в файл
	logFile, err := os.OpenFile("logs/currency_parser.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[CURRENCY CRON ERROR] Ошибка открытия лог файла: %v", err)
		return
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags)
	logger.Printf("[CURRENCY CRON] Начинаем парсинг курсов валют...")

	// Создаем парсер и выполняем парсинг
	parser := NewCurrencyParser(ccs.currencyService)
	if err := parser.ParseAndSaveCurrencyRates(); err != nil {
		logger.Printf("[CURRENCY CRON ERROR] Ошибка парсинга: %v", err)
		return
	}

	logger.Printf("[CURRENCY CRON] Парсинг курсов валют завершен успешно")
}


