package controllers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BankController - контроллер для работы с банками
type BankController struct {
	db *gorm.DB
}

// NewBankController - создает новый экземпляр контроллера банков
func NewBankController(db *gorm.DB) *BankController {
	return &BankController{db: db}
}

// GetBankInfo - получает информацию о банке по названию
func (bc *BankController) GetBankInfo(c *gin.Context) {
	bankName := c.Param("name")
	if bankName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Название банка не указано",
		})
		return
	}

	// Нормализуем название банка
	normalizedName := bc.normalizeBankName(bankName)

	// Получаем информацию о банке из справочника
	var bankReference struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		LogoPath    string `json:"logo_path"`
		IsActive    bool   `json:"is_active"`
	}

	err := bc.db.Table("bank_references").
		Select("id, name, display_name, logo_path, is_active").
		Where("name = ? OR aliases LIKE ?", normalizedName, "%"+bankName+"%").
		First(&bankReference).Error

	if err != nil {
		// Если банк не найден в справочнике, возвращаем базовую информацию
		c.JSON(http.StatusOK, gin.H{
			"result": gin.H{
				"name":         normalizedName,
				"display_name": normalizedName,
				"logo_path":    "",
				"is_active":    true,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": bankReference,
	})
}

// GetBanksList - получает список всех банков
func (bc *BankController) GetBanksList(c *gin.Context) {
	var banks []struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		LogoPath    string `json:"logo_path"`
		IsActive    bool   `json:"is_active"`
	}

	err := bc.db.Table("bank_references").
		Select("id, name, display_name, logo_path, is_active").
		Where("is_active = ?", true).
		Order("display_name").
		Find(&banks).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка при получении списка банков",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": banks,
	})
}

// UpdateBankLogo - обновляет логотип банка
func (bc *BankController) UpdateBankLogo(c *gin.Context) {
	bankName := c.Param("name")
	logoPath := c.PostForm("logo_path")

	if bankName == "" || logoPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Название банка и путь к логотипу обязательны",
		})
		return
	}

	// Нормализуем название банка
	normalizedName := bc.normalizeBankName(bankName)

	// Обновляем логотип в справочнике
	result := bc.db.Table("bank_references").
		Where("name = ?", normalizedName).
		Update("logo_path", logoPath)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка при обновлении логотипа",
		})
		return
	}

	if result.RowsAffected == 0 {
		// Если банк не найден в справочнике, создаем запись
		err := bc.db.Exec(`
			INSERT INTO bank_references (name, display_name, logo_path, is_active) 
			VALUES (?, ?, ?, true)
			ON CONFLICT (name) DO UPDATE SET 
			logo_path = EXCLUDED.logo_path,
			updated_at = CURRENT_TIMESTAMP
		`, normalizedName, normalizedName, logoPath).Error

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Ошибка при создании записи банка",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"message":   "Логотип банка успешно обновлен",
			"bank_name": normalizedName,
			"logo_path": logoPath,
		},
	})
}

// normalizeBankName - нормализует название банка
func (bc *BankController) normalizeBankName(bankName string) string {
	// Базовые правила нормализации
	normalized := strings.TrimSpace(bankName)

	// Маппинг для стандартизации
	mappings := map[string]string{
		"Turon bank":               "Turon Bank",
		"Davr bank":                "Davr Bank",
		"Garant bank":              "Garant Bank",
		"Poytaxt bank":             "Poytaxt Bank",
		"Universal bank":           "Universal Bank",
		"Ipoteka bank":             "Ipoteka Bank",
		"O'zbekiston Milliy banki": "O'zbekiston Milliy Banki",
		"InfinBank":                "Infinbank",
		"TBC UZ":                   "TBC Bank",
		"AVO bank":                 "AVO Bank",
		"AVO":                      "AVO Bank",
	}

	if mapped, exists := mappings[normalized]; exists {
		return mapped
	}

	// Если нет в маппинге, применяем общие правила
	words := strings.Fields(normalized)
	for i, word := range words {
		if strings.ToLower(word) == "bank" || strings.ToLower(word) == "banki" {
			words[i] = strings.Title(strings.ToLower(word))
		} else {
			words[i] = strings.Title(strings.ToLower(word))
		}
	}

	return strings.Join(words, " ")
}

// SetupBankRoutes - настраивает маршруты для банков
func (bc *BankController) SetupBankRoutes(router *gin.RouterGroup) {
	banks := router.Group("/banks")
	{
		banks.GET("/list", bc.GetBanksList)
		banks.GET("/:name", bc.GetBankInfo)
		banks.POST("/:name/logo", bc.UpdateBankLogo)
	}
}
