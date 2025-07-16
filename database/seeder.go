package database

import (
	"kliro/models"

	"gorm.io/gorm"
)

// SeedRegions проверяет таблицу regions и, если она пуста, заполняет её регионами Узбекистана (латиницей)
func SeedRegions(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.Region{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil // Уже есть регионы, ничего не делаем
	}
	regions := []models.Region{
		{Name: "Andijan"},
		{Name: "Bukhara"},
		{Name: "Fergana"},
		{Name: "Jizzakh"},
		{Name: "Khorezm"},
		{Name: "Namangan"},
		{Name: "Navoi"},
		{Name: "Kashkadarya"},
		{Name: "Karakalpakstan"},
		{Name: "Samarkand"},
		{Name: "Surkhandarya"},
		{Name: "Syrdarya"},
		{Name: "Tashkent"},
		{Name: "Tashkent Region"},
	}
	return db.Create(&regions).Error
}
