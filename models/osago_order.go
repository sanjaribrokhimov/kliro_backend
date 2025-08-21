package models

import "time"

// OsagoOrder хранит заказы ОСАГО, связанные с пользователем
type OsagoOrder struct {
	ID              uint   `gorm:"primaryKey"`
	UserID          uint   `gorm:"index:uniq_osago_user_extorder,unique;not null"`
	ExternalOrderID int64  `gorm:"index:uniq_osago_user_extorder,unique;not null"`
	Status          string `gorm:"type:varchar(50)"`
	AmountUZS       *int64
	PolicyNumber    *string    `gorm:"type:varchar(100)"`
	GosNumber       *string    `gorm:"type:varchar(32)"`
	BeginDate       *string    `gorm:"type:varchar(20)"`
	EndDate         *string    `gorm:"type:varchar(20)"`
	PdfURL          *string    `gorm:"type:text"`
	IssuedAt        *time.Time // дата оформления (при успешной оплате/активации)
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
