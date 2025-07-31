package controllers

import (
	"kliro/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DepositController struct {
	db *gorm.DB
}

func NewDepositController(db *gorm.DB) *DepositController {
	return &DepositController{db: db}
}

// GetNewDeposits получает новые вклады с пагинацией
func (dc *DepositController) GetNewDeposits(c *gin.Context) {
	getDepositsWithPagination(c, dc.db, "new_deposit")
}

// GetOldDeposits получает старые вклады с пагинацией
func (dc *DepositController) GetOldDeposits(c *gin.Context) {
	getDepositsWithPagination(c, dc.db, "old_deposit")
}

func getDepositsWithPagination(c *gin.Context, db *gorm.DB, tableName string) {
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
	var deposits []models.Deposit
	offset := (page - 1) * limit

	query := db.Table(tableName)
	if sortOrder == "desc" {
		query = query.Order(sortBy + " DESC")
	} else {
		query = query.Order(sortBy + " ASC")
	}

	query.Offset(offset).Limit(limit).Find(&deposits)

	// Формируем ответ
	response := gin.H{
		"data": deposits,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"total":      total,
			"totalPages": (total + int64(limit) - 1) / int64(limit),
		},
	}

	c.JSON(http.StatusOK, response)
}
