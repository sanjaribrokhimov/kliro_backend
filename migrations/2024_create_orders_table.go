package migrations

import (
	"gorm.io/gorm"
)

// CreateOrdersTable создает таблицу orders с оптимизированными индексами
func CreateOrdersTable(db *gorm.DB) error {
	return db.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			order_id VARCHAR(255) UNIQUE NOT NULL,
			category VARCHAR(100) NOT NULL,
			company_name VARCHAR(255) NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			deleted_at TIMESTAMP WITH TIME ZONE
		);

		-- Индексы для быстрого поиска по пользователю
		CREATE INDEX IF NOT EXISTS idx_user_orders ON orders(user_id);
		CREATE INDEX IF NOT EXISTS idx_user_created ON orders(user_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_user_status ON orders(user_id, status);

		-- Индексы для поиска по категории и компании
		CREATE INDEX IF NOT EXISTS idx_category ON orders(category);
		CREATE INDEX IF NOT EXISTS idx_company ON orders(company_name);
		CREATE INDEX IF NOT EXISTS idx_status ON orders(status);

		-- Составной индекс для быстрого поиска заказов пользователя по категории
		CREATE INDEX IF NOT EXISTS idx_user_category ON orders(user_id, category);

		-- Индекс для поиска по order_id (уже есть UNIQUE, но добавим для явности)
		CREATE INDEX IF NOT EXISTS idx_order_id ON orders(order_id);

		-- Индекс для soft delete
		CREATE INDEX IF NOT EXISTS idx_deleted_at ON orders(deleted_at);
	`).Error
}
