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

// CreditCard модель для кредитных карт
type CreditCard struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BankName  string    `json:"bank_name"`
	Title     string    `json:"title"`
	Rate      string    `json:"rate"`
	Term      string    `json:"term"`
	Amount    string    `json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

func (CreditCard) TableName() string { return "new_credit_card" }
