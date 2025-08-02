package models

import (
	"time"
)

type Deposit struct {
	ID        uint      `gorm:"primaryKey;table:new_deposit" json:"id"`
	BankName  string    `json:"bank_name"`
	Rate      string   `json:"rate"`
	TermYears string     `json:"term_years"`
	MinAmount string   `json:"min_amount"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}
