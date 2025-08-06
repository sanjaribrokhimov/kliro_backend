package migrations

import (
	"gorm.io/gorm"
)

func UpdateDepositTables(db *gorm.DB) error {
	// Удаляем старые таблицы
	if err := db.Exec(`DROP TABLE IF EXISTS new_deposit CASCADE`).Error; err != nil {
		return err
	}
	

	// Создаем новые таблицы с правильной структурой
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_deposit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			rate TEXT,
			term_years TEXT,
			min_amount TEXT,
			title TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	

	// Создаем индексы
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_deposit_bank_name ON new_deposit(bank_name)`).Error; err != nil {
		return err
	}
	

	return nil
}
