package models

import (
	"time"

	"gorm.io/gorm"
)

type Currency struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	BankName  string         `json:"bank_name" gorm:"not null"`
	Currency  string         `json:"currency" gorm:"not null"`
	BuyRate   float64        `json:"buy_rate" gorm:"not null"`
	SellRate  *float64       `json:"sell_rate"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName указывает имя таблицы
func (Currency) TableName() string {
	return "currencies"
}
