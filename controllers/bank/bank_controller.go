package bank

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BankController struct {
	db *gorm.DB
}

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

func (bc *BankController) SearchBanks(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	category := strings.TrimSpace(c.Query("category"))

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Параметр 'q' обязателен",
		})
		return
	}

	if category == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Параметр 'category' обязателен",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	if page < 0 {
		page = 0
	}
	if size < 1 || size > 100 {
		size = 10
	}

	tableMap := map[string]string{
		"microcredits": "new_microcredit",
		"autocredits":  "new_autocredit",
		"transfers":    "new_transfer",
		"mortgages":    "new_mortgage",
		"deposits":     "new_deposit",
		"cards":        "new_card",
		"credit-cards": "new_credit_card",
	}

	tableName, exists := tableMap[category]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Неверная категория",
		})
		return
	}

	searchField := "bank_name"
	if category == "transfers" {
		searchField = "app_name"
	}

	var totalElements int64
	bc.db.Table(tableName).
		Where(searchField+" ILIKE ?", "%"+query+"%").
		Count(&totalElements)

	totalPages := int((totalElements + int64(size) - 1) / int64(size))

	offset := page * size
	var results []map[string]interface{}

	err := bc.db.Table(tableName).
		Where(searchField+" ILIKE ?", "%"+query+"%").
		Offset(offset).
		Limit(size).
		Find(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка при получении данных",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"content":          results,
			"totalElements":    totalElements,
			"totalPages":       totalPages,
			"size":             size,
			"number":           page,
			"numberOfElements": len(results),
			"first":            page == 0,
			"last":             page >= totalPages-1,
		},
	})
}

func (bc *BankController) normalizeBankName(bankName string) string {
	normalized := strings.TrimSpace(bankName)

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
