package migrations

import (
	"gorm.io/gorm"
)

func AddFirstNameLastNameToUsers(db *gorm.DB) error {
	return db.Exec(`
		ALTER TABLE users 
		ADD COLUMN first_name VARCHAR(255),
		ADD COLUMN last_name VARCHAR(255),
		DROP COLUMN name;
	`).Error
}
