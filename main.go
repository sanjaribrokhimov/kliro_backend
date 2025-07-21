package main

import (
	"fmt"
	"log"
	"os"

	"context"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"kliro/controllers"
	"kliro/database"
	"kliro/routes"
	"kliro/utils"
)

func main() {
	
	// Загрузка .env
	err := godotenv.Load("./.env")
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

	// Подключение к Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:6379", os.Getenv("DB_HOST")),
		Password: "",
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	log.Println("Connected to Redis")

	// Инициализация Google OAuth
	controllers.InitGoogleOAuth()

	// Создание Gin роутера и настройка всех маршрутов
	r := routes.SetupRouter()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server is running on port %s", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
