package models

import "time"

// DeletedUserBackup хранит полную копию пользователя перед hard delete (бэкап).
// Таблица: deleted_users_backup.
type DeletedUserBackup struct {
	ID                uint      `gorm:"primaryKey"`
	OriginalUserID    uint      `gorm:"not null;index:idx_deleted_backup_original_id"`
	Email             *string   `gorm:"type:varchar(255)"`
	Phone             *string   `gorm:"type:varchar(255)"`
	RegionID          *uint
	Password          string    `gorm:"type:varchar(255)"`
	Confirmed         bool
	Role              string    `gorm:"type:varchar(64)"`
	CategoryID        *uint
	FirstName         *string   `gorm:"type:varchar(255)"`
	LastName          *string   `gorm:"type:varchar(255)"`
	GoogleID          *string   `gorm:"type:varchar(255)"`
	OriginalCreatedAt time.Time `gorm:"not null"`
	OriginalUpdatedAt time.Time `gorm:"not null"`
	DeletedAt         time.Time `gorm:"not null;index:idx_deleted_backup_deleted_at"` // когда выполнен delete
}

// TableName задаёт имя таблицы бэкапа.
func (DeletedUserBackup) TableName() string {
	return "deleted_users_backup"
}
