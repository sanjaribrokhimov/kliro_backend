package migrations

import (
	"gorm.io/gorm"
)

// CreateInsuranceProfileTable создает таблицу для профилей страховки
func CreateInsuranceProfileTable(db *gorm.DB) error {
	// Создаем таблицу insurance_profiles
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS insurance_profiles (
			id SERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			product VARCHAR(255) NOT NULL,
			date DATE NOT NULL,
			order_id VARCHAR(255) NOT NULL,
			amount DECIMAL(15,2) NOT NULL,
			is_paid BOOLEAN DEFAULT FALSE,
			document_url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL,
			
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`).Error; err != nil {
		return err
	}

	// Создаем индексы отдельно
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_user_insurance ON insurance_profiles(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_product ON insurance_profiles(product)",
		"CREATE INDEX IF NOT EXISTS idx_order ON insurance_profiles(order_id)",
		"CREATE INDEX IF NOT EXISTS idx_payment_status ON insurance_profiles(is_paid)",
		"CREATE INDEX IF NOT EXISTS idx_deleted_at ON insurance_profiles(deleted_at)",
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			return err
		}
	}

	return nil
}
