package models

import "gorm.io/gorm"

// Favorite - избранное пользователя (avia/hotel) по item_id
type Favorite struct {
	gorm.Model
	UserID    uint   `json:"user_id" gorm:"not null;index"`
	Direction string `json:"direction" gorm:"type:varchar(10);not null;index"` // строго: "avia" | "hotel"
	ItemID    string `json:"item_id" gorm:"type:text;not null;index"`

	// Связь с пользователем (не обязательно подгружать)
	User User `json:"-" gorm:"foreignKey:UserID;references:ID"`
}

