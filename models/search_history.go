package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AviaSearchHistory - история поиска авиабилетов
type AviaSearchHistory struct {
	gorm.Model
	UserID uint `json:"user_id" gorm:"not null;index"`

	// Данные поиска (JSON)
	Adults         int                    `json:"adults" gorm:"not null"`
	Children       int                    `json:"children" gorm:"default:0"`
	Infants        int                    `json:"infants" gorm:"default:0"`
	InfantsWithSeat int                   `json:"infants_with_seat" gorm:"default:0"`
	ServiceClass   string                 `json:"service_class" gorm:"type:varchar(10);not null"`
	Directions     datatypes.JSON         `json:"directions" gorm:"type:jsonb;not null"` // массив направлений

	// Связь с пользователем
	User User `json:"-" gorm:"foreignKey:UserID;references:ID"`
}

// HotelSearchHistory - история поиска отелей
type HotelSearchHistory struct {
	gorm.Model
	UserID uint `json:"user_id" gorm:"not null;index"`

	// Данные поиска (JSON)
	CityID     int                    `json:"city_id" gorm:"not null"`
	CheckIn    string                 `json:"check_in" gorm:"type:varchar(50);not null"` // "2026/01/27 14:00"
	CheckOut   string                 `json:"check_out" gorm:"type:varchar(50);not null"` // "2026/01/29 12:00"
	IsResident bool                   `json:"is_resident" gorm:"default:false"`
	Occupancies datatypes.JSON        `json:"occupancies" gorm:"type:jsonb;not null"` // массив occupancies
	Currency   string                 `json:"currency" gorm:"type:varchar(10);not null"` // "uzs", "usd", etc
	MealPlans  datatypes.JSON         `json:"meal_plans" gorm:"type:jsonb"` // массив строк ["RO", "BB", "HB", "FB"]

	// Связь с пользователем
	User User `json:"-" gorm:"foreignKey:UserID;references:ID"`
}

// InsuranceSearchHistory - история поиска страховки
type InsuranceSearchHistory struct {
	gorm.Model
	UserID uint `json:"user_id" gorm:"not null;index"`

	// Данные поиска
	PassportSeries     *string `json:"passport_series" gorm:"type:varchar(10)"`
	PassportNumber     *string `json:"passport_number" gorm:"type:varchar(50)"`
	BirthDate          *string `json:"birth_date" gorm:"type:varchar(20)"` // "YYYY-MM-DD" или другой формат
	Pinfl              *string `json:"pinfl" gorm:"type:varchar(14)"` // PINFL код (14 цифр)
	CarNumber          *string `json:"car_number" gorm:"type:varchar(20)"` // номер машины
	TechPassportSeries *string `json:"tech_passport_series" gorm:"type:varchar(10)"` // серия техпаспорта
	TechPassportNumber *string `json:"tech_passport_number" gorm:"type:varchar(50)"` // номер техпаспорта

	// Связь с пользователем
	User User `json:"-" gorm:"foreignKey:UserID;references:ID"`
}
