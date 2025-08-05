package models

import (
	"time"
)

type Autocredit struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	BankName    string    `json:"bank_name"`
	Description string    `json:"description"`
	Rate        string    `json:"rate"`
	Term        string    `json:"term"`
	Amount      string    `json:"amount"`
	Channel     string    `json:"channel"`
	CreatedAt   time.Time `json:"created_at"`
}
