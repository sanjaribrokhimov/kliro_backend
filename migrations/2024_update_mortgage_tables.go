package migrations

import (
	"gorm.io/gorm"
)

func UpdateMortgageTables(db *gorm.DB) error {
	// Обновляем таблицу new_mortgage
	if err := db.Exec(`
		ALTER TABLE new_mortgage 
		ADD COLUMN IF NOT EXISTS description TEXT,
		ADD COLUMN IF NOT EXISTS rate TEXT,
		ADD COLUMN IF NOT EXISTS term TEXT,
		ADD COLUMN IF NOT EXISTS amount TEXT,
		ADD COLUMN IF NOT EXISTS channel TEXT
	`).Error; err != nil {
		return err
	}

	

	// Если поле rate уже существует как numeric, изменяем его тип на TEXT
	if err := db.Exec(`ALTER TABLE new_mortgage ALTER COLUMN rate TYPE TEXT`).Error; err != nil {
		// Игнорируем ошибку, если поле не существует или уже имеет правильный тип
	}
	

	return nil
}
