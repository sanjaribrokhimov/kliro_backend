package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Email      *string `gorm:"uniqueIndex"`
	Phone      *string `gorm:"uniqueIndex"`
	RegionID   *uint
	Password   string
	Confirmed  bool   `gorm:"default:false"`
	Role       string `gorm:"default:user"`
	CategoryID *uint
	Name     *string
	GoogleID *string
}
