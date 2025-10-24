package migrations

import (
	"gorm.io/gorm"
)

// UpdateInsuranceProfileDirectionToDate обновляет колонку direction на date в таблице insurance_profiles
func UpdateInsuranceProfileDirectionToDate(db *gorm.DB) error {
	// Проверяем, существует ли таблица insurance_profiles
	var tableExists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'insurance_profiles'
		)
	`).Scan(&tableExists).Error

	if err != nil {
		return err
	}

	if !tableExists {
		// Если таблица не существует, создаем ее с правильной структурой
		return CreateInsuranceProfileTable(db)
	}

	// Проверяем, существует ли колонка direction
	var columnExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.columns 
			WHERE table_schema = 'public' 
			AND table_name = 'insurance_profiles' 
			AND column_name = 'direction'
		)
	`).Scan(&columnExists).Error

	if err != nil {
		return err
	}

	if columnExists {
		// Если колонка direction существует, удаляем ее и добавляем date
		if err := db.Exec(`ALTER TABLE insurance_profiles DROP COLUMN IF EXISTS direction`).Error; err != nil {
			return err
		}
	}

	// Добавляем колонку date, если она не существует
	var dateColumnExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.columns 
			WHERE table_schema = 'public' 
			AND table_name = 'insurance_profiles' 
			AND column_name = 'date'
		)
	`).Scan(&dateColumnExists).Error

	if err != nil {
		return err
	}

	if !dateColumnExists {
		if err := db.Exec(`ALTER TABLE insurance_profiles ADD COLUMN date DATE`).Error; err != nil {
			return err
		}
	}

	return nil
}
