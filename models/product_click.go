package models

import "time"

type ProductClick struct {
	ID         uint   `gorm:"primaryKey"`
	Key        string `gorm:"type:varchar(255);not null"`
	Direction  string `gorm:"type:varchar(100);not null"`
	URL        string `gorm:"type:text;not null"`
	ClickCount int    `gorm:"default:1;not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
