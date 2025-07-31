package models

import (
	"time"
)

type Deposit struct {
	ID         uint      `gorm:"primaryKey;table:new_deposit" json:"id"`
	BankName   string    `json:"bank_name"`
	Rate       float64   `json:"rate"`
	TermMonths int       `json:"term_months"`
	MinAmount  float64   `json:"min_amount"`
	URL        string    `json:"url"`
	CreatedAt  time.Time `json:"created_at"`
}
