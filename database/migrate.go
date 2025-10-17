package database

import (
	"kliro/migrations"
	"kliro/models"

	"gorm.io/gorm"
)

func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&models.User{}, &models.Region{}, &models.Category{}, &models.Currency{}, &models.OsagoOrder{}, &models.ProductClick{}); err != nil {
		return err
	}

	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_product_click_key_direction ON product_clicks(key, direction)`).Error; err != nil {
		return err
	}

	// Создаем таблицы для microcredit
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_microcredit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			description TEXT,
			rate TEXT,
			term TEXT,
			amount TEXT,
			channel TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	// Создаем таблицы для autocredit (обновляем структуру)
	if err := db.Exec(`DROP TABLE IF EXISTS new_autocredit`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		CREATE TABLE new_autocredit (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			description TEXT,
			rate TEXT,
			term TEXT,
			amount TEXT,
			channel TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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

	// Создаем индексы для переводов
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_transfer_app_name ON new_transfer(app_name)`).Error; err != nil {
		return err
	}

	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_transfer_created_at ON new_transfer(created_at)`).Error; err != nil {
		return err
	}

	// Создаем таблицы для ипотеки
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_mortgage (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			description TEXT,
			rate TEXT,
			term TEXT,
			amount TEXT,
			channel TEXT,
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

	// Создаем индексы для ипотеки и вкладов
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_mortgage_bank_name ON new_mortgage(bank_name)`).Error; err != nil {
		return err
	}

	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_deposit_bank_name ON new_deposit(bank_name)`).Error; err != nil {
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

	// Создаем таблицу для кредитных карт
	if err := db.Exec(`
		CREATE TABLE IF NOT EXISTS new_credit_card (
			id SERIAL PRIMARY KEY,
			bank_name VARCHAR(255),
			title TEXT,
			rate TEXT,
			term TEXT,
			amount TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`).Error; err != nil {
		return err
	}

	// Создаем индексы для карт
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_new_card_bank_name ON new_card(bank_name)`).Error; err != nil {
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

	// Обновляем таблицы микрокредитов с новой структурой
	if err := migrations.UpdateMicrocreditTables(db); err != nil {
		return err
	}

	// Обновляем таблицы ипотеки с новой структурой
	if err := migrations.UpdateMortgageTables(db); err != nil {
		return err
	}

	// Добавляем URL в таблицу микрокредитов
	if err := migrations.AddURLToMicrocredit(db); err != nil {
		return err
	}

	// Создаем таблицу orders
	if err := migrations.CreateOrdersTable(db); err != nil {
		return err
	}

	return nil
}
