package database

import (
	"kliro/models"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.User{}, &models.Region{}, &models.Category{}); err != nil {
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

	return nil
}
