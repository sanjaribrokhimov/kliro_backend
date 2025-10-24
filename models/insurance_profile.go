package models

import (
	"time"

	"gorm.io/gorm"
)

// InsuranceProfile представляет профиль страховки пользователя
type InsuranceProfile struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	UserID      uint           `json:"user_id" gorm:"not null;index:idx_user_insurance"`
	Product     string         `json:"product" gorm:"not null;index:idx_product"`
	Date        time.Time      `json:"date" gorm:"not null"`
	OrderID     string         `json:"order_id" gorm:"not null;index:idx_order"`
	Amount      float64        `json:"amount" gorm:"not null"`
	IsPaid      bool           `json:"is_paid" gorm:"default:false;index:idx_payment_status"`
	DocumentURL *string        `json:"document_url"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`

	// Связь с пользователем
	User User `json:"user,omitempty" gorm:"foreignKey:UserID;references:ID"`
}

// InsuranceProfileRequest структура для создания профиля страховки
type InsuranceProfileRequest struct {
	Product     string  `json:"product" binding:"required"`
	Date        string  `json:"date" binding:"required"`
	OrderID     string  `json:"order_id" binding:"required"`
	Amount      float64 `json:"amount" binding:"required"`
	IsPaid      bool    `json:"is_paid"`
	DocumentURL *string `json:"document_url"`
}

// InsuranceProfileResponse структура ответа для профиля страховки
type InsuranceProfileResponse struct {
	ID          uint      `json:"id"`
	UserID      uint      `json:"user_id"`
	Product     string    `json:"product"`
	Date        time.Time `json:"date"`
	OrderID     string    `json:"order_id"`
	Amount      float64   `json:"amount"`
	IsPaid      bool      `json:"is_paid"`
	DocumentURL *string   `json:"document_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// InsuranceProfileListResponse структура для списка профилей страховки с пагинацией
type InsuranceProfileListResponse struct {
	Profiles   []InsuranceProfileResponse `json:"profiles"`
	Total      int64                      `json:"total"`
	Page       int                        `json:"page"`
	Limit      int                        `json:"limit"`
	TotalPages int                        `json:"total_pages"`
}
