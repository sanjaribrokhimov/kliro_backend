package migrations

import (
	"gorm.io/gorm"
)

func UpdateMicrocreditTables(db *gorm.DB) error {
	// Удаляем старые таблицы
	if err := db.Exec(`DROP TABLE IF EXISTS new_microcredit`).Error; err != nil {
		return err
	}


	// Создаем новые таблицы с правильной структурой
	if err := db.Exec(`
		CREATE TABLE new_microcredit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			description TEXT,
			rate TEXT,
			term TEXT,
			amount TEXT,
			channel TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	

	// Создаем индексы
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_microcredit_bank_name ON new_microcredit(bank_name)`).Error; err != nil {
		return err
	}
	

	return nil
}
