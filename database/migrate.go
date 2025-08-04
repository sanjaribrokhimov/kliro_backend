package database

import (
	"kliro/migrations"
	"kliro/models"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.User{}, &models.Region{}, &models.Category{}, &models.Currency{}); err != nil {
		return err
	}

	// Создаем таблицы для microcredit
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_microcredit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			max_amount DECIMAL(15,2),
			rate_max DECIMAL(5,2),
			rate_min DECIMAL(5,2),
			term_months INTEGER,
			url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS old_microcredit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			max_amount DECIMAL(15,2),
			rate_max DECIMAL(5,2),
			rate_min DECIMAL(5,2),
			term_months INTEGER,
			url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	// Создаем таблицы для autocredit
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_autocredit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			rate DECIMAL(5,2) DEFAULT 0,
			initial_payment DECIMAL(10,2) DEFAULT 0,
			term_months INTEGER DEFAULT 0,
			max_amount VARCHAR(50) DEFAULT 'VIP',
			url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS old_autocredit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			rate DECIMAL(5,2) DEFAULT 0,
			initial_payment DECIMAL(10,2) DEFAULT 0,
			term_months INTEGER DEFAULT 0,
			max_amount VARCHAR(50) DEFAULT 'VIP',
			url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	// Создаем таблицу для валют
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS currencies (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255) NOT NULL,
			currency VARCHAR(10) NOT NULL,
			buy_rate DECIMAL(10,2) NOT NULL,
			sell_rate DECIMAL(10,2),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL
		)
	`).Error; err != nil {
		return err
	}

	// Создаем таблицы new_currency и old_currency
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_currency (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255) NOT NULL,
			currency VARCHAR(10) NOT NULL,
			buy_rate DECIMAL(10,2) NOT NULL,
			sell_rate DECIMAL(10,2),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS old_currency (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255) NOT NULL,
			currency VARCHAR(10) NOT NULL,
			buy_rate DECIMAL(10,2) NOT NULL,
			sell_rate DECIMAL(10,2),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	// Создаем индексы для валют
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_currencies_currency ON currencies(currency)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_currencies_created_at ON currencies(created_at)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_currencies_deleted_at ON currencies(deleted_at)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_currency_currency ON new_currency(currency)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_old_currency_currency ON old_currency(currency)`).Error; err != nil {
		return err
	}

	// Создаем таблицы для переводов (создаем если не существуют)
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_transfer (
			id SERIAL PRIMARY KEY,
			app_name VARCHAR(100) NOT NULL,
			commission VARCHAR(50) NOT NULL,
			limit_ru TEXT NULL,
			limit_uz TEXT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS old_transfer (
			id SERIAL PRIMARY KEY,
			app_name VARCHAR(100) NOT NULL,
			commission VARCHAR(50) NOT NULL,
			limit_ru TEXT NULL,
			limit_uz TEXT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	// Создаем индексы для переводов
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_transfer_app_name ON new_transfer(app_name)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_old_transfer_app_name ON old_transfer(app_name)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_transfer_created_at ON new_transfer(created_at)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_old_transfer_created_at ON old_transfer(created_at)`).Error; err != nil {
		return err
	}

	// Создаем таблицы для ипотеки
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_mortgage (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			rate DECIMAL(5,2),
			term_years INTEGER,
			max_amount DECIMAL(15,2),
			initial_payment DECIMAL(15,2),
			url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS old_mortgage (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			rate DECIMAL(5,2),
			term_years INTEGER,
			max_amount DECIMAL(15,2),
			initial_payment DECIMAL(15,2),
			url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	// Создаем таблицы для вкладов
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_deposit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			rate TEXT,
			term_years TEXT,
			min_amount TEXT,
			title TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS old_deposit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			rate TEXT,
			term_years TEXT,
			min_amount TEXT,
			title TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	// Создаем индексы для ипотеки и вкладов
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_mortgage_bank_name ON new_mortgage(bank_name)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_old_mortgage_bank_name ON old_mortgage(bank_name)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_deposit_bank_name ON new_deposit(bank_name)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_old_deposit_bank_name ON old_deposit(bank_name)`).Error; err != nil {
		return err
	}

	// Создаем таблицы для карт
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_card (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			title TEXT,
			currency TEXT,
			system TEXT,
			opening_type TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS old_card (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			title TEXT,
			currency TEXT,
			system TEXT,
			opening_type TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	// Создаем индексы для карт
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_card_bank_name ON new_card(bank_name)`).Error; err != nil {
		return err
	}
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_old_card_bank_name ON old_card(bank_name)`).Error; err != nil {
		return err
	}

	// Обновляем таблицы вкладов с новой структурой
	if err := migrations.UpdateDepositTables(db); err != nil {
		return err
	}

	// Удаляем поле description из таблиц карт
	if err := migrations.RemoveDescriptionFromCardTables(db); err != nil {
		return err
	}

	return nil
}
