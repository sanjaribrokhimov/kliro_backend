package models

import (
	"time"
)

type Card struct {
	ID          uint      `gorm:"primaryKey"`
	BankName    string
	Title       string
	Currency    string
	System      string
	OpeningType string
	CreatedAt   time.Time
}

