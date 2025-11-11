package models

import "gorm.io/gorm"

type BlogPost struct {
	gorm.Model
	Category    string `gorm:"type:VARCHAR(20);index"`
	Title       string `gorm:"type:VARCHAR(255);not null"`
	Description string `gorm:"type:TEXT;not null"`
	Photos      string `gorm:"type:TEXT"` // JSON string (array of strings)
	Links       string `gorm:"type:TEXT"` // JSON string (array of strings)
}


