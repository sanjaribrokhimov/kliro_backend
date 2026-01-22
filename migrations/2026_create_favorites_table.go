package migrations

import "gorm.io/gorm"

// CreateFavoritesTable создает таблицу favorites для избранного пользователя
func CreateFavoritesTable(db *gorm.DB) error {
	// NOTE: используем soft delete (deleted_at), поэтому делаем partial unique index,
	// чтобы после удаления можно было заново добавить тот же item.
	return db.Exec(`
		CREATE TABLE IF NOT EXISTS favorites (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			direction VARCHAR(10) NOT NULL,
			item_id TEXT NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			deleted_at TIMESTAMP WITH TIME ZONE
		);

		CREATE INDEX IF NOT EXISTS idx_favorites_user_id ON favorites(user_id);
		CREATE INDEX IF NOT EXISTS idx_favorites_direction ON favorites(direction);
		CREATE INDEX IF NOT EXISTS idx_favorites_item_id ON favorites(item_id);
		CREATE INDEX IF NOT EXISTS idx_favorites_created_at ON favorites(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_favorites_deleted_at ON favorites(deleted_at);

		-- Уникальность по пользователю + типу + item_id (только для не удаленных записей)
		CREATE UNIQUE INDEX IF NOT EXISTS uniq_favorites_user_direction_item
		ON favorites(user_id, direction, item_id)
		WHERE deleted_at IS NULL;
	`).Error
}

