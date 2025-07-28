package models

import (
	"time"
)

type Autocredit struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	BankName       string    `json:"bank_name"`
	Rate           float64   `json:"rate"`
	InitialPayment float64   `json:"initial_payment"`
	TermMonths     int       `json:"term_months"`
	MaxAmount      string    `json:"max_amount"`
	URL            string    `json:"url"`
	CreatedAt      time.Time `json:"created_at"`
}
