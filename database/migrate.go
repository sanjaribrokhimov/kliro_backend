package database

import (
	"kliro/models"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.User{}, &models.Region{}, &models.Category{}, &models.Currency{}); err != nil {
		return err
	}

	// Создаем таблицы для microcredit
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_microcredit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			max_amount DECIMAL(15,2),
			rate_max DECIMAL(5,2),
			rate_min DECIMAL(5,2),
			term_months INTEGER,
			url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS old_microcredit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			max_amount DECIMAL(15,2),
			rate_max DECIMAL(5,2),
			rate_min DECIMAL(5,2),
			term_months INTEGER,
			url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	// Создаем таблицу для валют
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS currencies (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255) NOT NULL,
			currency VARCHAR(10) NOT NULL,
			buy_rate DECIMAL(10,2) NOT NULL,
			sell_rate DECIMAL(10,2),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL
		)
	`).Error; err != nil {
		return err
	}

	// Создаем индексы для валют
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_currencies_currency ON currencies(currency)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_currencies_created_at ON currencies(created_at)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_currencies_deleted_at ON currencies(deleted_at)`).Error; err != nil {
		return err
	}

	return nil
}
