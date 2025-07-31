package controllers

import (
	"kliro/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MortgageController struct {
	db *gorm.DB
}

func NewMortgageController(db *gorm.DB) *MortgageController {
	return &MortgageController{db: db}
}

// GetNewMortgages получает новые ипотечные кредиты с пагинацией
func (mc *MortgageController) GetNewMortgages(c *gin.Context) {
	getMortgagesWithPagination(c, mc.db, "new_mortgage")
}

// GetOldMortgages получает старые ипотечные кредиты с пагинацией
func (mc *MortgageController) GetOldMortgages(c *gin.Context) {
	getMortgagesWithPagination(c, mc.db, "old_mortgage")
}

func getMortgagesWithPagination(c *gin.Context, db *gorm.DB, tableName string) {
	// Получаем параметры пагинации
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	sortBy := c.DefaultQuery("sortBy", "created_at")
	sortOrder := c.DefaultQuery("sortOrder", "desc")

	// Валидация параметров
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	// Подсчитываем общее количество записей
	var total int64
	db.Table(tableName).Count(&total)

	// Получаем данные с пагинацией и сортировкой
	var mortgages []models.Mortgage
	offset := (page - 1) * limit

	query := db.Table(tableName)
	if sortOrder == "desc" {
		query = query.Order(sortBy + " DESC")
	} else {
		query = query.Order(sortBy + " ASC")
	}

	query.Offset(offset).Limit(limit).Find(&mortgages)

	// Формируем ответ в стандартном формате
	totalPages := (total + int64(limit) - 1) / int64(limit)

	response := gin.H{
		"result": gin.H{
			"totalPages":       totalPages,
			"totalElements":    total,
			"first":            page == 1,
			"last":             page >= int(totalPages),
			"size":             limit,
			"content":          mortgages,
			"number":           page - 1, // Spring Boot использует 0-based индексацию
			"numberOfElements": len(mortgages),
			"empty":            len(mortgages) == 0,
		},
		"success": true,
		"error":   nil,
	}

	c.JSON(http.StatusOK, response)
}
