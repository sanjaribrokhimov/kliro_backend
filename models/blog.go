package models

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// BlogPost хранит контент блога с локализациями и метаданными
type BlogPost struct {
	gorm.Model
	Category     string         `gorm:"type:VARCHAR(50);index"`
	Title        string         `gorm:"type:VARCHAR(255);not null"`
	Description  string         `gorm:"type:TEXT;not null"`
	Tags         datatypes.JSON `gorm:"type:jsonb"`
	PhotosBase64 datatypes.JSON `gorm:"type:jsonb"`
	Uz           datatypes.JSON `gorm:"type:jsonb"`
	Oz           datatypes.JSON `gorm:"type:jsonb"`
	Ru           datatypes.JSON `gorm:"type:jsonb"`
	En           datatypes.JSON `gorm:"type:jsonb"`
	Views        int64          `gorm:"default:0"`
	Likes        int64          `gorm:"default:0"`
	Alias        string         `gorm:"type:VARCHAR(255);uniqueIndex"`
}


