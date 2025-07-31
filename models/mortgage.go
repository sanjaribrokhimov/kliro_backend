package models

import (
	"time"
)

type Mortgage struct {
	ID             uint      `gorm:"primaryKey;table:new_mortgage" json:"id"`
	BankName       string    `json:"bank_name"`
	RateMax        float64   `json:"rate_max"`
	RateMin        float64   `json:"rate_min"`
	TermYears      int       `json:"term_years"`
	MaxAmount      float64   `json:"max_amount"`
	InitialPayment float64   `json:"initial_payment"`
	URL            string    `json:"url"`
	CreatedAt      time.Time `json:"created_at"`
}
