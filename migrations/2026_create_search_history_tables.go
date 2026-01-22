package migrations

import "gorm.io/gorm"

// CreateSearchHistoryTables создает таблицы для истории поисков (avia, hotel, insurance)
func CreateSearchHistoryTables(db *gorm.DB) error {
	// Таблица avia_search_history
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS avia_search_history (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			adults INTEGER NOT NULL,
			children INTEGER DEFAULT 0,
			infants INTEGER DEFAULT 0,
			infants_with_seat INTEGER DEFAULT 0,
			service_class VARCHAR(10) NOT NULL,
			directions JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			deleted_at TIMESTAMP WITH TIME ZONE
		);
		
		CREATE INDEX IF NOT EXISTS idx_avia_search_user_id ON avia_search_history(user_id);
		CREATE INDEX IF NOT EXISTS idx_avia_search_created_at ON avia_search_history(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_avia_search_deleted_at ON avia_search_history(deleted_at);
	`).Error; err != nil {
		return err
	}

	// Таблица hotel_search_history
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS hotel_search_history (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			city_id INTEGER NOT NULL,
			check_in VARCHAR(50) NOT NULL,
			check_out VARCHAR(50) NOT NULL,
			is_resident BOOLEAN DEFAULT FALSE,
			occupancies JSONB NOT NULL,
			currency VARCHAR(10) NOT NULL,
			meal_plans JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			deleted_at TIMESTAMP WITH TIME ZONE
		);
		
		CREATE INDEX IF NOT EXISTS idx_hotel_search_user_id ON hotel_search_history(user_id);
		CREATE INDEX IF NOT EXISTS idx_hotel_search_created_at ON hotel_search_history(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_hotel_search_deleted_at ON hotel_search_history(deleted_at);
	`).Error; err != nil {
		return err
	}

	// Таблица insurance_search_history
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS insurance_search_history (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			passport_series VARCHAR(10),
			passport_number VARCHAR(50),
			birth_date VARCHAR(20),
			pinfl VARCHAR(14),
			car_number VARCHAR(20),
			tech_passport_series VARCHAR(10),
			tech_passport_number VARCHAR(50),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			deleted_at TIMESTAMP WITH TIME ZONE
		);
		
		CREATE INDEX IF NOT EXISTS idx_insurance_search_user_id ON insurance_search_history(user_id);
		CREATE INDEX IF NOT EXISTS idx_insurance_search_created_at ON insurance_search_history(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_insurance_search_deleted_at ON insurance_search_history(deleted_at);
	`).Error; err != nil {
		return err
	}

	return nil
}
