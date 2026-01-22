package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"kliro/models"
	"kliro/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FavoriteController struct {
	db *gorm.DB
}

func NewFavoriteController() *FavoriteController {
	return &FavoriteController{db: utils.GetDB()}
}

func normalizeDirection(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "avia" || v == "hotel" {
		return v
	}
	return ""
}

// POST /user/favorites
func (fc *FavoriteController) Create(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	var req struct {
		Direction string `json:"direction" binding:"required"`
		ItemID    string `json:"item_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	direction := normalizeDirection(req.Direction)
	if direction == "" {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "direction должен быть строго: avia или hotel"})
		return
	}
	itemID := strings.TrimSpace(req.ItemID)
	if itemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "item_id обязателен"})
		return
	}

	// Проверяем существование пользователя
	var userCount int64
	if err := fc.db.Model(&models.User{}).Where("id = ?", userID).Count(&userCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка проверки пользователя"})
		return
	}
	if userCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Пользователь не найден"})
		return
	}

	// Проверяем, не добавлено ли уже в избранное
	var existing models.Favorite
	if err := fc.db.Where("user_id = ? AND direction = ? AND item_id = ? AND deleted_at IS NULL", userID, direction, itemID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"result": nil, "success": false, "error": "Уже в избранном"})
		return
	}

	fav := models.Favorite{
		UserID:    userID,
		Direction: direction,
		ItemID:    itemID,
	}

	if err := fc.db.Create(&fav).Error; err != nil {
		// Проверяем тип ошибки
		if strings.Contains(err.Error(), "foreign key constraint") || strings.Contains(err.Error(), "23503") {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Пользователь не найден"})
			return
		}
		if strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "23505") {
			c.JSON(http.StatusConflict, gin.H{"result": nil, "success": false, "error": "Уже в избранном"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка создания избранного"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"result": fav, "success": true})
}

// GET /user/favorites?page=1&limit=20&direction=avia|hotel
func (fc *FavoriteController) List(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	dirQ := c.Query("direction")
	direction := ""
	if dirQ != "" {
		direction = normalizeDirection(dirQ)
		if direction == "" {
			c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "direction должен быть строго: avia или hotel"})
			return
		}
	}

	query := fc.db.Model(&models.Favorite{}).Where("user_id = ?", userID)
	if direction != "" {
		query = query.Where("direction = ?", direction)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка подсчета избранного"})
		return
	}

	var favorites []models.Favorite
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&favorites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения избранного"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))

	c.JSON(http.StatusOK, gin.H{"result": gin.H{
		"totalPages":    totalPages,
		"totalElements": total,
		"first":         page == 1,
		"last":          page >= totalPages && totalPages != 0,
		"size":          limit,
		"content":       favorites,
		"number":        page - 1,
		"numberOfElements": len(favorites),
		"empty":            len(favorites) == 0,
		"pageable": gin.H{
			"offset":     offset,
			"pageNumber": page - 1,
			"pageSize":   limit,
			"paged":      true,
			"unpaged":    false,
		},
	}, "success": true})
}

// GET /user/favorites/:id
func (fc *FavoriteController) Get(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid id"})
		return
	}

	var fav models.Favorite
	if err := fc.db.Where("id = ? AND user_id = ?", id, userID).First(&fav).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Не найдено"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": fav, "success": true})
}

// PUT /user/favorites/:id (полная замена direction + item_id)
func (fc *FavoriteController) Put(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid id"})
		return
	}

	var req struct {
		Direction string `json:"direction" binding:"required"`
		ItemID    string `json:"item_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	direction := normalizeDirection(req.Direction)
	if direction == "" {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "direction должен быть строго: avia или hotel"})
		return
	}
	itemID := strings.TrimSpace(req.ItemID)
	if itemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "item_id обязателен"})
		return
	}

	var fav models.Favorite
	if err := fc.db.Where("id = ? AND user_id = ?", id, userID).First(&fav).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Не найдено"})
		return
	}

	fav.Direction = direction
	fav.ItemID = itemID

	if err := fc.db.Save(&fav).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"result": nil, "success": false, "error": "Уже в избранном"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": fav, "success": true})
}

// PATCH /user/favorites/:id (частичное обновление direction / item_id)
func (fc *FavoriteController) Patch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid id"})
		return
	}

	var req struct {
		Direction *string `json:"direction"`
		ItemID    *string `json:"item_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	var fav models.Favorite
	if err := fc.db.Where("id = ? AND user_id = ?", id, userID).First(&fav).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Не найдено"})
		return
	}

	if req.Direction != nil {
		direction := normalizeDirection(*req.Direction)
		if direction == "" {
			c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "direction должен быть строго: avia или hotel"})
			return
		}
		fav.Direction = direction
	}
	if req.ItemID != nil {
		itemID := strings.TrimSpace(*req.ItemID)
		if itemID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "item_id обязателен"})
			return
		}
		fav.ItemID = itemID
	}

	if err := fc.db.Save(&fav).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"result": nil, "success": false, "error": "Уже в избранном"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": fav, "success": true})
}

// DELETE /user/favorites/:id
func (fc *FavoriteController) Delete(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid id"})
		return
	}

	var fav models.Favorite
	if err := fc.db.Where("id = ? AND user_id = ?", id, userID).First(&fav).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Не найдено"})
		return
	}

	if err := fc.db.Delete(&fav).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка удаления"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": gin.H{"id": fav.ID}, "success": true})
}

