package models

import (
	"time"
)

type Microcredit struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	BankName   string    `json:"bank_name"`
	MaxAmount  float64   `json:"max_amount"`
	RateMax    float64   `json:"rate_max"`
	RateMin    float64   `json:"rate_min"`
	TermMonths int       `json:"term_months"`
	URL        string    `json:"url"`
	CreatedAt  time.Time `json:"created_at"`
}
