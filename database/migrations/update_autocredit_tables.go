package migrations

import (
	"gorm.io/gorm"
)

// UpdateAutocreditTables обновляет структуру таблиц автокредитов
func UpdateAutocreditTables(db *gorm.DB) error {
	// Удаляем старые таблицы
	if err := db.Exec(`DROP TABLE IF EXISTS new_autocredit`).Error; err != nil {
		return err
	}
	if err := db.Exec(`DROP TABLE IF EXISTS old_autocredit`).Error; err != nil {
		return err
	}

	// Создаем новые таблицы с правильной структурой
	if err := db.Exec(`
		CREATE TABLE new_autocredit (
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

	if err := db.Exec(`
		CREATE TABLE old_autocredit (
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
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_autocredit_bank_name ON new_autocredit(bank_name)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_old_autocredit_bank_name ON old_autocredit(bank_name)`).Error; err != nil {
		return err
	}

	return nil
}
