package migrations

import (
	"gorm.io/gorm"
)

func AddURLToMicrocredit(db *gorm.DB) error {
	// Добавляем колонку url в таблицу new_microcredit
	err := db.Exec("ALTER TABLE new_microcredit ADD COLUMN IF NOT EXISTS url TEXT").Error
	if err != nil {
		return err
	}

	return nil
}
