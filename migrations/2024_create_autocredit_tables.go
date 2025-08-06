package migrations

import (
	"gorm.io/gorm"
)

func CreateAutocreditTables(db *gorm.DB) error {
	// Создаем таблицу new_autocredit
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_autocredit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255) NOT NULL,
			rate_min DECIMAL(10,2) DEFAULT 0,
			rate_max DECIMAL(10,2) DEFAULT 0,
			initial_payment DECIMAL(10,2) DEFAULT 0,
			term_months INTEGER DEFAULT 0,
			url TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	

	return nil
}
