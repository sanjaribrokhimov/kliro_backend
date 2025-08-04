package models

import (
	"time"
)

type Card struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	BankName    string    `json:"bank_name"`
	Title       string    `json:"title"`
	Currency    string    `json:"currency"`
	System      string    `json:"system"`
	OpeningType string    `json:"opening_type"`
	CreatedAt   time.Time `json:"created_at"`
}
