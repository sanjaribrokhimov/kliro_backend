package models

import (
	"time"

	"gorm.io/gorm"
)

// Order представляет заказ пользователя
type Order struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	UserID      uint           `json:"user_id" gorm:"not null;index:idx_user_orders"`
	OrderID     string         `json:"order_id" gorm:"uniqueIndex;not null"`
	Category    string         `json:"category" gorm:"not null;index:idx_category"`
	CompanyName string         `json:"company_name" gorm:"not null;index:idx_company"`
	Status      string         `json:"status" gorm:"default:'pending';index:idx_status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`

	// Связь с пользователем
	User User `json:"user,omitempty" gorm:"foreignKey:UserID;references:ID"`
}

// OrderRequest структура для создания заказа
type OrderRequest struct {
	UserID      uint   `json:"user_id" binding:"required"`
	OrderID     string `json:"order_id" binding:"required"`
	Category    string `json:"category" binding:"required"`
	CompanyName string `json:"company_name" binding:"required"`
	Status      string `json:"status,omitempty"`
}

// OrderResponse структура ответа для заказа
type OrderResponse struct {
	ID          uint      `json:"id"`
	UserID      uint      `json:"user_id"`
	OrderID     string    `json:"order_id"`
	Category    string    `json:"category"`
	CompanyName string    `json:"company_name"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// OrderListResponse структура для списка заказов с пагинацией
type OrderListResponse struct {
	Orders     []OrderResponse `json:"orders"`
	Total      int64           `json:"total"`
	Page       int             `json:"page"`
	Limit      int             `json:"limit"`
	TotalPages int             `json:"total_pages"`
}
