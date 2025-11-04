package migrations

import (
	"gorm.io/gorm"
)

// CreatePaymentsTable создает таблицы для платежей
func CreatePaymentsTable(db *gorm.DB) error {
	// Таблица payments
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS payments (
			id SERIAL PRIMARY KEY,
			invoice_id VARCHAR(255) UNIQUE NOT NULL,
			payment_method VARCHAR(50) NOT NULL,
			amount BIGINT NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			store_id VARCHAR(50) NOT NULL,
			multicard_uuid VARCHAR(255),
			checkout_url TEXT,
			card_token VARCHAR(255),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			deleted_at TIMESTAMP WITH TIME ZONE
		);

		CREATE INDEX IF NOT EXISTS idx_payments_invoice ON payments(invoice_id);
		CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
		CREATE INDEX IF NOT EXISTS idx_payments_uuid ON payments(multicard_uuid);
	`).Error; err != nil {
		return err
	}

	// Таблица payment_cards для привязанных карт пользователей
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS payment_cards (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			card_token VARCHAR(255) UNIQUE NOT NULL,
			card_pan VARCHAR(50) NOT NULL,
			card_name VARCHAR(100),
			payment_system VARCHAR(20),
			status VARCHAR(50) DEFAULT 'active',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			deleted_at TIMESTAMP WITH TIME ZONE
		);

		CREATE INDEX IF NOT EXISTS idx_payment_cards_user ON payment_cards(user_id);
		CREATE INDEX IF NOT EXISTS idx_payment_cards_token ON payment_cards(card_token);
	`).Error; err != nil {
		return err
	}

	return nil
}
