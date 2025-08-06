package migrations

import (
	"gorm.io/gorm"
)

func CreateCurrencyTables(db *gorm.DB) error {
	// Создаем таблицу new_currency
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_currency (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255) NOT NULL,
			currency VARCHAR(10) NOT NULL,
			buy_rate DECIMAL(10,2) NOT NULL,
			sell_rate DECIMAL(10,2),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}



	return nil
} 