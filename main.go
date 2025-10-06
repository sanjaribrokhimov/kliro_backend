package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"context"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"kliro/controllers"
	"kliro/database"
	"kliro/routes"
	bankServices "kliro/services/bank"
	"kliro/utils"
)

func main() {
	// Устанавливаем часовой пояс Узбекистана для всех логов
	uzbekLocation, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		uzbekLocation = time.FixedZone("UZT", 5*60*60)
	}
	time.Local = uzbekLocation

	// Загрузка .env
	err = godotenv.Load(".env")
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Подключение к PostgreSQL
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// Устанавливаем глобальный *gorm.DB для контроллеров
	utils.SetDB(db)

	// Миграция
	if err := database.Migrate(db); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}
	log.Println("Migration complete")

	// Сидирование регионов
	if err := database.SeedRegions(db); err != nil {
		log.Fatalf("failed to seed regions: %v", err)
	}
	log.Println("Regions seeded (if needed)")

	// Запуск всех cron'ов асинхронно
	go func() {
		log.Println("Starting bank services in background...")

		// Запуск microcredit cron
		bankServices.StartMicrocreditCron(db)
		log.Println("Microcredit cron started")

		// Запуск autocredit cron
		bankServices.StartAutocreditCron(db)
		log.Println("Autocredit cron started")

		// Запуск transfer cron
		bankServices.StartTransferCron(db)
		log.Println("Transfer cron started")

		// Запуск mortgage cron
		bankServices.StartMortgageCron(db)
		log.Println("Mortgage cron started")

		// Запуск deposit cron
		bankServices.StartDepositCron(db)
		log.Println("Deposit cron started")

		// Запуск card cron
		bankServices.StartCardCron(db)
		log.Println("Card cron started")

		// Запуск currency cron
		bankServices.StartCurrencyCron(db)
		log.Println("Currency cron started")

		// Запуск credit card cron (инициализация и ежедневный парсинг кредитных карт)
		bankServices.StartCreditCardCron(db)
		log.Println("Credit Card cron started")

		log.Println("All bank services started successfully!")
	}()

	log.Println("Bank services starting in background...")

	// Подключение к Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     getenvOr("REDIS_ADDR", fmt.Sprintf("%s:6379", os.Getenv("DB_HOST"))),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	utils.SetRedis(rdb)
	log.Println("Connected to Redis")

	// Инициализация Google OAuth
	controllers.InitGoogleOAuth()

	// Создание Gin роутера и настройка всех маршрутов
	fmt.Println("==========================================")
	fmt.Println("DEBUG: ВЫЗЫВАЕМ routes.SetupRouter()!")
	fmt.Println("==========================================")
	r := routes.SetupRouter()
	fmt.Println("==========================================")
	fmt.Println("DEBUG: routes.SetupRouter() ВЫЗВАН!")
	fmt.Println("==========================================")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server is running on port %s", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}

}

func getenvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
