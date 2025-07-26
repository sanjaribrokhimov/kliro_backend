package migrations

import (
	"database/sql"
	"fmt"
)

func CreateCurrencyTable(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS currencies (
		id SERIAL PRIMARY KEY,
		bank_name VARCHAR(255) NOT NULL,
		currency VARCHAR(10) NOT NULL,
		buy_rate DECIMAL(10,2) NOT NULL,
		sell_rate DECIMAL(10,2),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		deleted_at TIMESTAMP NULL
	);
	
	CREATE INDEX IF NOT EXISTS idx_currencies_currency ON currencies(currency);
	CREATE INDEX IF NOT EXISTS idx_currencies_created_at ON currencies(created_at);
	CREATE INDEX IF NOT EXISTS idx_currencies_deleted_at ON currencies(deleted_at);
	`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create currencies table: %v", err)
	}

	fmt.Println("✅ Currencies table created successfully")
	return nil
}
