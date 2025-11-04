package models

import (
	"time"

	"gorm.io/gorm"
)

// Payment - платеж
type Payment struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	InvoiceID     string         `json:"invoice_id" gorm:"uniqueIndex;not null"` // уникальный номер заказа
	PaymentMethod string         `json:"payment_method" gorm:"not null"`         // payme, click, uzum, card
	Amount        int64          `json:"amount" gorm:"not null"`                 // сумма в тийинах (100000 = 1000 сум)
	Status        string         `json:"status" gorm:"default:'pending'"`        // pending, success, failed, cancelled
	StoreID       string         `json:"store_id" gorm:"not null"`               // ID кассы в Multicard
	MulticardUUID string         `json:"multicard_uuid"`                         // ID платежа в Multicard
	CheckoutURL   string         `json:"checkout_url"`                           // ссылка для оплаты
	CardToken     string         `json:"card_token"`                             // токен карты (если оплата картой)
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// PaymentCard - привязанная карта пользователя для платежей
type PaymentCard struct {
	ID            uint           `json:"id" gorm:"primaryKey"`
	UserID        uint           `json:"user_id" gorm:"index"`
	CardToken     string         `json:"card_token" gorm:"uniqueIndex;not null"` // токен от Multicard
	CardPAN       string         `json:"card_pan" gorm:"not null"`               // номер карты (маскированный)
	CardName      string         `json:"card_name"`                              // имя держателя
	PaymentSystem string         `json:"payment_system"`                         // uzcard, humo, visa, mastercard
	Status        string         `json:"status" gorm:"default:'active'"`         // active, deleted
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TableName указывает имя таблицы
func (PaymentCard) TableName() string {
	return "payment_cards"
}

// OFDItem - элемент фискализации
type OFDItem struct {
	Qty         int     `json:"qty" binding:"required"`          // количество единиц товара/услуги
	Price       int64   `json:"price" binding:"required"`        // стоимость единицы в тийинах
	MXIK        string  `json:"mxik" binding:"required"`         // ИКПУ из справочника tasnif.soliq.uz
	PackageCode string  `json:"package_code" binding:"required"` // код упаковки из tasnif.soliq.uz
	Name        string  `json:"name" binding:"required"`         // наименование товара/услуги
	VAT         *int    `json:"vat,omitempty"`                   // НДС (%)
	Total       *int64  `json:"total,omitempty"`                 // общая сумма товаров с учетом количества
	TIN         *string `json:"tin,omitempty"`                   // ИНН компании
}

// CreatePaymentRequest - запрос на создание платежа
type CreatePaymentRequest struct {
	PaymentMethod string    `json:"payment_method" binding:"required"` // payme, click, uzum, card
	Amount        int64     `json:"amount" binding:"required,gt=0"`    // сумма в тийинах
	InvoiceID     string    `json:"invoice_id" binding:"required"`     // формат: avia{id} или hotel{id} или другой
	CardToken     *string   `json:"card_token,omitempty"`              // токен карты (только для payment_method="card")
	OFD           []OFDItem `json:"ofd,omitempty"`                     // данные для фискализации (опционально)
}

// ConfirmPaymentRequest - запрос на подтверждение платежа
type ConfirmPaymentRequest struct {
	OTP string `json:"otp" binding:"required"` // код подтверждения от SMS
}

// BindCardRequest - запрос на привязку карты
type BindCardRequest struct {
	UserID    *uint  `json:"user_id,omitempty"`
	Phone     string `json:"phone,omitempty"` // 998901234567
	ReturnURL string `json:"return_url"`      // куда вернуть после привязки
}
