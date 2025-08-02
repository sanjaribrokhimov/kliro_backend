package migrations

import (
	"gorm.io/gorm"
)

func RemoveDescriptionFromCardTables(db *gorm.DB) error {
	// Удаляем поле description из таблицы new_card
	if err := db.Exec(`ALTER TABLE new_card DROP COLUMN IF EXISTS description`).Error; err != nil {
		return err
	}

	// Удаляем поле description из таблицы old_card
	if err := db.Exec(`ALTER TABLE old_card DROP COLUMN IF EXISTS description`).Error; err != nil {
		return err
	}

	return nil
}
