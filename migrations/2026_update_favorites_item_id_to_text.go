package migrations

import "gorm.io/gorm"

// UpdateFavoritesItemIDToText изменяет тип item_id с VARCHAR(255) на TEXT
// для поддержки длинных ID (например, avia offer IDs)
func UpdateFavoritesItemIDToText(db *gorm.DB) error {
	// Проверяем, существует ли таблица и колонка
	var exists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns 
			WHERE table_name = 'favorites' 
			AND column_name = 'item_id'
			AND data_type = 'character varying'
		)
	`).Scan(&exists).Error
	if err != nil {
		return err
	}

	// Если колонка существует и имеет тип VARCHAR, изменяем на TEXT
	if exists {
		return db.Exec(`
			ALTER TABLE favorites 
			ALTER COLUMN item_id TYPE TEXT
		`).Error
	}

	// Если таблица не существует или колонка уже TEXT, ничего не делаем
	return nil
}
