package migrations

import "gorm.io/gorm"

// UpdateInsuranceSearchAddTechPassport добавляет поля техпаспорта в таблицу insurance_search_history
func UpdateInsuranceSearchAddTechPassport(db *gorm.DB) error {
	// Проверяем, существует ли колонка
	var exists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'insurance_search_history' 
			AND column_name = 'tech_passport_series'
		)
	`).Scan(&exists).Error
	if err != nil {
		return err
	}

	// Если колонки не существует, добавляем
	if !exists {
		if err := db.Exec(`
			ALTER TABLE insurance_search_history 
			ADD COLUMN tech_passport_series VARCHAR(10),
			ADD COLUMN tech_passport_number VARCHAR(50)
		`).Error; err != nil {
			return err
		}
	}

	// Убираем NOT NULL с passport_number если есть
	if err := db.Exec(`
		ALTER TABLE insurance_search_history 
		ALTER COLUMN passport_number DROP NOT NULL
	`).Error; err != nil {
		// Игнорируем ошибку если NOT NULL уже нет
	}

	return nil
}
