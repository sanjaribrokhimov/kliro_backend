package models

import "gorm.io/gorm"

// UserHuman хранит данные пассажиров/людей пользователя
type UserHuman struct {
	gorm.Model
	UserID         uint   `gorm:"index"`
	FirstName      string `gorm:"type:VARCHAR(100)"`
	LastName       string `gorm:"type:VARCHAR(100)"`
	MiddleName     string `gorm:"type:VARCHAR(100)"`
	BirthDate      string `gorm:"type:VARCHAR(20)"`
	Gender         string `gorm:"type:VARCHAR(20)"`
	Citizenship    string `gorm:"type:VARCHAR(100)"`
	PassportNumber string `gorm:"type:VARCHAR(50)"`
	PassportExpiry string `gorm:"type:VARCHAR(20)"`
	Phone          string `gorm:"type:VARCHAR(50)"`
}


