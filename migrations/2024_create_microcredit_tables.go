package migrations

import (
	"gorm.io/gorm"
)

func CreateMicrocreditTables(db *gorm.DB) error {
	type Microcredit struct {
		ID         uint `gorm:"primaryKey"`
		BankName   string
		MaxAmount  float64
		RateMax    float64
		RateMin    float64
		TermMonths int
		URL        string
		CreatedAt  int64
	}
	if err := db.Migrator().CreateTable(&Microcredit{}); err != nil {
		return err
	}
	return nil
}
