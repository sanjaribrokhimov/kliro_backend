package models

import (
	"time"
)

type Transfer struct {
	ID         uint      `gorm:"primaryKey;table:new_transfer" json:"id"`
	AppName    string    `json:"app_name"`
	Commission string    `json:"commission"`
	LimitRU    *string   `gorm:"column:limit_ru" json:"limit_ru"`
	LimitUZ    *string   `gorm:"column:limit_uz" json:"limit_uz"`
	CreatedAt  time.Time `json:"created_at"`
}
