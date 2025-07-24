package services

import (
	"encoding/json"
	"io/ioutil"
	"kliro/models"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var microcreditURLs = []string{
	"https://tengebank.uz/credit/mikrozajm-onlajn",
	"https://ru.ipakyulibank.uz/physical/kredity/mikrozaymy",
	"https://tbcbank.uz/ru/product/kredity/",
	"https://aloqabank.uz/uz/private/crediting/onlayn-mikroqarz/",
	"https://mkbank.uz/uz/private/crediting/microloan/",
	"https://xb.uz/page/onlayn-mikroqarz",
	"https://turonbank.uz/ru/private/crediting/mikrokredit-dlya-samozanyatykh-lits/",
	"https://hamkorbank.uz/physical/credits/microloan-online/",
	"https://sqb.uz/uz/individuals/credits/mikrozaym-ru/",
	"https://www.ipotekabank.uz/private/crediting/micro_new/",
}

// Функция для парсинга одного URL
func parseMicrocreditURL(url string, logger *log.Logger) *models.Microcredit {
	resp, err := http.Get("http://localhost:8080/parse?url=" + url)
	if err != nil {
		logger.Printf("Ошибка запроса %s: %v", url, err)
		return nil
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var parsed struct {
		Result  models.Microcredit `json:"result"`
		Success bool               `json:"success"`
	}

	if err := json.Unmarshal(body, &parsed); err != nil || !parsed.Success {
		logger.Printf("Ошибка парсинга %s: %v", url, err)
		return nil
	}

	parsed.Result.CreatedAt = time.Now()
	return &parsed.Result
}

// Инициализация данных (первый запуск)
func InitializeMicrocreditData(db *gorm.DB) {
	logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	logger := log.New(logFile, "", log.LstdFlags)
	defer logFile.Close()

	// Проверяем, есть ли данные в таблицах
	var count int64
	db.Table("new_microcredit").Count(&count)

	if count == 0 {
		logger.Printf("Инициализация данных microcredit - таблицы пустые, парсим все сайты...")

		// Парсим все URL'ы и сохраняем в обе таблицы
		for _, url := range microcreditURLs {
			if credit := parseMicrocreditURL(url, logger); credit != nil {
				db.Table("new_microcredit").Create(credit)
				db.Table("old_microcredit").Create(credit)
			}
		}

		logger.Printf("Инициализация завершена - заполнены таблицы new_microcredit и old_microcredit")
	}
}

func StartMicrocreditCron(db *gorm.DB) {
	// Инициализируем данные при первом запуске
	InitializeMicrocreditData(db)

	c := cron.New()
	c.AddFunc("0 20 * * *", func() {
		logFile, _ := os.OpenFile("logs/parser_errors.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		logger := log.New(logFile, "", log.LstdFlags)
		defer logFile.Close()

		logger.Printf("Начало ежедневного парсинга microcredit...")

		// Копируем new_microcredit в old_microcredit
		db.Exec("TRUNCATE old_microcredit")
		db.Exec("INSERT INTO old_microcredit SELECT * FROM new_microcredit")
		db.Exec("TRUNCATE new_microcredit")

		// Парсим все URL'ы заново
		for _, url := range microcreditURLs {
			if credit := parseMicrocreditURL(url, logger); credit != nil {
				db.Table("new_microcredit").Create(credit)
			}
		}

		logger.Printf("Ежедневный парсинг microcredit завершен")
	})
	c.Start()
}
