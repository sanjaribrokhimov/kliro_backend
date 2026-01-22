package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"kliro/models"
	"kliro/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SearchHistoryController struct {
	db *gorm.DB
}

func NewSearchHistoryController() *SearchHistoryController {
	return &SearchHistoryController{db: utils.GetDB()}
}

// validateString проверяет строку: если пустая - возвращает nil, иначе указатель на строку
func validateString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// validateRequiredString проверяет обязательное поле: не может быть пустой строкой
func validateRequiredString(s string, fieldName string) (*string, error) {
	if s == "" {
		return nil, fmt.Errorf("поле %s обязательно и не может быть пустым", fieldName)
	}
	return &s, nil
}

// ==================== AVIA SEARCH HISTORY ====================

// POST /user/search-history/avia
func (shc *SearchHistoryController) CreateAviaSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	var req struct {
		Adults         int                    `json:"adults" binding:"required,min=1"`
		Children       int                    `json:"children" binding:"min=0"`
		Infants        int                    `json:"infants" binding:"min=0"`
		InfantsWithSeat int                   `json:"infants_with_seat" binding:"min=0"`
		ServiceClass   string                 `json:"service_class" binding:"required"`
		Directions     []map[string]interface{} `json:"directions" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request: " + err.Error()})
		return
	}

	// Проверяем существование пользователя
	var userCount int64
	if err := shc.db.Model(&models.User{}).Where("id = ?", userID).Count(&userCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка проверки пользователя"})
		return
	}
	if userCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Пользователь не найден"})
		return
	}

	// Конвертируем directions в JSON
	directionsJSON, err := json.Marshal(req.Directions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "Ошибка обработки directions"})
		return
	}

	search := models.AviaSearchHistory{
		UserID:         userID,
		Adults:         req.Adults,
		Children:       req.Children,
		Infants:        req.Infants,
		InfantsWithSeat: req.InfantsWithSeat,
		ServiceClass:   req.ServiceClass,
		Directions:     directionsJSON,
	}

	if err := shc.db.Create(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка создания истории поиска"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"result": search, "success": true})
}

// GET /user/search-history/avia?page=1&limit=20
func (shc *SearchHistoryController) ListAviaSearch(c *gin.Context) {
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

	query := shc.db.Model(&models.AviaSearchHistory{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка подсчета"})
		return
	}

	var searches []models.AviaSearchHistory
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&searches).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения истории"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"content":       searches,
			"totalPages":    totalPages,
			"totalElements": total,
			"size":          limit,
			"number":        page - 1,
			"first":         page == 1,
			"last":          page >= totalPages,
		},
		"success": true,
	})
}

// GET /user/search-history/avia/:id
func (shc *SearchHistoryController) GetAviaSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.AviaSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": search, "success": true})
}

// PUT /user/search-history/avia/:id
func (shc *SearchHistoryController) UpdateAviaSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.AviaSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	var req struct {
		Adults         int                    `json:"adults" binding:"required,min=1"`
		Children       int                    `json:"children" binding:"min=0"`
		Infants        int                    `json:"infants" binding:"min=0"`
		InfantsWithSeat int                   `json:"infants_with_seat" binding:"min=0"`
		ServiceClass   string                 `json:"service_class" binding:"required"`
		Directions     []map[string]interface{} `json:"directions" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request: " + err.Error()})
		return
	}

	directionsJSON, err := json.Marshal(req.Directions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "Ошибка обработки directions"})
		return
	}

	search.Adults = req.Adults
	search.Children = req.Children
	search.Infants = req.Infants
	search.InfantsWithSeat = req.InfantsWithSeat
	search.ServiceClass = req.ServiceClass
	search.Directions = directionsJSON

	if err := shc.db.Save(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка обновления записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": search, "success": true})
}

// PATCH /user/search-history/avia/:id
func (shc *SearchHistoryController) PatchAviaSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.AviaSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	// Обновляем только переданные поля
	if v, ok := req["adults"].(float64); ok {
		search.Adults = int(v)
	}
	if v, ok := req["children"].(float64); ok {
		search.Children = int(v)
	}
	if v, ok := req["infants"].(float64); ok {
		search.Infants = int(v)
	}
	if v, ok := req["infants_with_seat"].(float64); ok {
		search.InfantsWithSeat = int(v)
	}
	if v, ok := req["service_class"].(string); ok {
		search.ServiceClass = v
	}
	if v, ok := req["directions"].([]interface{}); ok {
		directionsJSON, err := json.Marshal(v)
		if err == nil {
			search.Directions = directionsJSON
		}
	}

	if err := shc.db.Save(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка обновления записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": search, "success": true})
}

// DELETE /user/search-history/avia/:id
func (shc *SearchHistoryController) DeleteAviaSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.AviaSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	if err := shc.db.Delete(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка удаления записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": gin.H{"id": id}, "success": true, "message": "Запись удалена"})
}

// ==================== HOTEL SEARCH HISTORY ====================

// POST /user/search-history/hotel
func (shc *SearchHistoryController) CreateHotelSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	var req struct {
		CityID     int                    `json:"city_id" binding:"required"`
		CheckIn    string                 `json:"check_in" binding:"required"`
		CheckOut   string                 `json:"check_out" binding:"required"`
		IsResident bool                   `json:"is_resident"`
		Occupancies []map[string]interface{} `json:"occupancies" binding:"required,min=1"`
		Currency   string                 `json:"currency" binding:"required"`
		MealPlans  []string               `json:"meal_plans"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request: " + err.Error()})
		return
	}

	// Проверяем существование пользователя
	var userCount int64
	if err := shc.db.Model(&models.User{}).Where("id = ?", userID).Count(&userCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка проверки пользователя"})
		return
	}
	if userCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Пользователь не найден"})
		return
	}

	occupanciesJSON, err := json.Marshal(req.Occupancies)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "Ошибка обработки occupancies"})
		return
	}

	var mealPlansJSON []byte
	if req.MealPlans != nil {
		mealPlansJSON, err = json.Marshal(req.MealPlans)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "Ошибка обработки meal_plans"})
			return
		}
	}

	search := models.HotelSearchHistory{
		UserID:      userID,
		CityID:      req.CityID,
		CheckIn:     req.CheckIn,
		CheckOut:    req.CheckOut,
		IsResident:  req.IsResident,
		Occupancies: occupanciesJSON,
		Currency:    req.Currency,
		MealPlans:   mealPlansJSON,
	}

	if err := shc.db.Create(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка создания истории поиска"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"result": search, "success": true})
}

// GET /user/search-history/hotel?page=1&limit=20
func (shc *SearchHistoryController) ListHotelSearch(c *gin.Context) {
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

	query := shc.db.Model(&models.HotelSearchHistory{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка подсчета"})
		return
	}

	var searches []models.HotelSearchHistory
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&searches).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения истории"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"content":       searches,
			"totalPages":    totalPages,
			"totalElements": total,
			"size":          limit,
			"number":        page - 1,
			"first":         page == 1,
			"last":          page >= totalPages,
		},
		"success": true,
	})
}

// GET /user/search-history/hotel/:id
func (shc *SearchHistoryController) GetHotelSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.HotelSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": search, "success": true})
}

// PUT /user/search-history/hotel/:id
func (shc *SearchHistoryController) UpdateHotelSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.HotelSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	var req struct {
		CityID     int                    `json:"city_id" binding:"required"`
		CheckIn    string                 `json:"check_in" binding:"required"`
		CheckOut   string                 `json:"check_out" binding:"required"`
		IsResident bool                   `json:"is_resident"`
		Occupancies []map[string]interface{} `json:"occupancies" binding:"required,min=1"`
		Currency   string                 `json:"currency" binding:"required"`
		MealPlans  []string               `json:"meal_plans"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request: " + err.Error()})
		return
	}

	occupanciesJSON, err := json.Marshal(req.Occupancies)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "Ошибка обработки occupancies"})
		return
	}

	var mealPlansJSON []byte
	if req.MealPlans != nil {
		mealPlansJSON, err = json.Marshal(req.MealPlans)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "Ошибка обработки meal_plans"})
			return
		}
	}

	search.CityID = req.CityID
	search.CheckIn = req.CheckIn
	search.CheckOut = req.CheckOut
	search.IsResident = req.IsResident
	search.Occupancies = occupanciesJSON
	search.Currency = req.Currency
	search.MealPlans = mealPlansJSON

	if err := shc.db.Save(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка обновления записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": search, "success": true})
}

// PATCH /user/search-history/hotel/:id
func (shc *SearchHistoryController) PatchHotelSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.HotelSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	// Обновляем только переданные поля
	if v, ok := req["city_id"].(float64); ok {
		search.CityID = int(v)
	}
	if v, ok := req["check_in"].(string); ok {
		search.CheckIn = v
	}
	if v, ok := req["check_out"].(string); ok {
		search.CheckOut = v
	}
	if v, ok := req["is_resident"].(bool); ok {
		search.IsResident = v
	}
	if v, ok := req["occupancies"].([]interface{}); ok {
		occupanciesJSON, err := json.Marshal(v)
		if err == nil {
			search.Occupancies = occupanciesJSON
		}
	}
	if v, ok := req["currency"].(string); ok {
		search.Currency = v
	}
	if v, ok := req["meal_plans"].([]interface{}); ok {
		mealPlansJSON, err := json.Marshal(v)
		if err == nil {
			search.MealPlans = mealPlansJSON
		}
	}

	if err := shc.db.Save(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка обновления записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": search, "success": true})
}

// DELETE /user/search-history/hotel/:id
func (shc *SearchHistoryController) DeleteHotelSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.HotelSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	if err := shc.db.Delete(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка удаления записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": gin.H{"id": id}, "success": true, "message": "Запись удалена"})
}

// ==================== INSURANCE SEARCH HISTORY ====================

// POST /user/search-history/insurance
func (shc *SearchHistoryController) CreateInsuranceSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	var req struct {
		PassportSeries     *string `json:"passport_series"`
		PassportNumber     *string `json:"passport_number"`
		BirthDate          *string `json:"birth_date"`
		Pinfl              *string `json:"pinfl"`
		CarNumber          *string `json:"car_number"`
		TechPassportSeries *string `json:"tech_passport_series"`
		TechPassportNumber *string `json:"tech_passport_number"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request: " + err.Error()})
		return
	}

	// Проверяем существование пользователя
	var userCount int64
	if err := shc.db.Model(&models.User{}).Where("id = ?", userID).Count(&userCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка проверки пользователя"})
		return
	}
	if userCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Пользователь не найден"})
		return
	}

	// Валидация: все поля опциональны, но если переданы - не могут быть пустыми строками
	// Нормализуем: если передана пустая строка после trim - делаем nil
	if req.PassportSeries != nil {
		trimmed := strings.TrimSpace(*req.PassportSeries)
		if trimmed == "" {
			req.PassportSeries = nil
		} else {
			req.PassportSeries = &trimmed
		}
	}
	if req.PassportNumber != nil {
		trimmed := strings.TrimSpace(*req.PassportNumber)
		if trimmed == "" {
			req.PassportNumber = nil
		} else {
			req.PassportNumber = &trimmed
		}
	}
	if req.BirthDate != nil {
		trimmed := strings.TrimSpace(*req.BirthDate)
		if trimmed == "" {
			req.BirthDate = nil
		} else {
			req.BirthDate = &trimmed
		}
	}
	if req.Pinfl != nil {
		trimmed := strings.TrimSpace(*req.Pinfl)
		if trimmed == "" {
			req.Pinfl = nil
		} else {
			req.Pinfl = &trimmed
		}
	}
	if req.CarNumber != nil {
		trimmed := strings.TrimSpace(*req.CarNumber)
		if trimmed == "" {
			req.CarNumber = nil
		} else {
			req.CarNumber = &trimmed
		}
	}
	if req.TechPassportSeries != nil {
		trimmed := strings.TrimSpace(*req.TechPassportSeries)
		if trimmed == "" {
			req.TechPassportSeries = nil
		} else {
			req.TechPassportSeries = &trimmed
		}
	}
	if req.TechPassportNumber != nil {
		trimmed := strings.TrimSpace(*req.TechPassportNumber)
		if trimmed == "" {
			req.TechPassportNumber = nil
		} else {
			req.TechPassportNumber = &trimmed
		}
	}

	search := models.InsuranceSearchHistory{
		UserID:             userID,
		PassportSeries:     req.PassportSeries,
		PassportNumber:     req.PassportNumber,
		BirthDate:          req.BirthDate,
		Pinfl:              req.Pinfl,
		CarNumber:          req.CarNumber,
		TechPassportSeries: req.TechPassportSeries,
		TechPassportNumber: req.TechPassportNumber,
	}

	if err := shc.db.Create(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка создания истории поиска"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"result": search, "success": true})
}

// GET /user/search-history/insurance?page=1&limit=20
func (shc *SearchHistoryController) ListInsuranceSearch(c *gin.Context) {
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

	query := shc.db.Model(&models.InsuranceSearchHistory{}).Where("user_id = ?", userID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка подсчета"})
		return
	}

	var searches []models.InsuranceSearchHistory
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&searches).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения истории"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"content":       searches,
			"totalPages":    totalPages,
			"totalElements": total,
			"size":          limit,
			"number":        page - 1,
			"first":         page == 1,
			"last":          page >= totalPages,
		},
		"success": true,
	})
}

// GET /user/search-history/insurance/:id
func (shc *SearchHistoryController) GetInsuranceSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.InsuranceSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": search, "success": true})
}

// PUT /user/search-history/insurance/:id
func (shc *SearchHistoryController) UpdateInsuranceSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.InsuranceSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	var req struct {
		PassportSeries     *string `json:"passport_series"`
		PassportNumber     *string `json:"passport_number"`
		BirthDate          *string `json:"birth_date"`
		Pinfl              *string `json:"pinfl"`
		CarNumber          *string `json:"car_number"`
		TechPassportSeries *string `json:"tech_passport_series"`
		TechPassportNumber *string `json:"tech_passport_number"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request: " + err.Error()})
		return
	}

	// Нормализуем пустые строки в nil
	if req.PassportSeries != nil {
		trimmed := strings.TrimSpace(*req.PassportSeries)
		if trimmed == "" {
			req.PassportSeries = nil
		} else {
			req.PassportSeries = &trimmed
		}
	}
	if req.PassportNumber != nil {
		trimmed := strings.TrimSpace(*req.PassportNumber)
		if trimmed == "" {
			req.PassportNumber = nil
		} else {
			req.PassportNumber = &trimmed
		}
	}
	if req.BirthDate != nil {
		trimmed := strings.TrimSpace(*req.BirthDate)
		if trimmed == "" {
			req.BirthDate = nil
		} else {
			req.BirthDate = &trimmed
		}
	}
	if req.Pinfl != nil {
		trimmed := strings.TrimSpace(*req.Pinfl)
		if trimmed == "" {
			req.Pinfl = nil
		} else {
			req.Pinfl = &trimmed
		}
	}
	if req.CarNumber != nil {
		trimmed := strings.TrimSpace(*req.CarNumber)
		if trimmed == "" {
			req.CarNumber = nil
		} else {
			req.CarNumber = &trimmed
		}
	}
	if req.TechPassportSeries != nil {
		trimmed := strings.TrimSpace(*req.TechPassportSeries)
		if trimmed == "" {
			req.TechPassportSeries = nil
		} else {
			req.TechPassportSeries = &trimmed
		}
	}
	if req.TechPassportNumber != nil {
		trimmed := strings.TrimSpace(*req.TechPassportNumber)
		if trimmed == "" {
			req.TechPassportNumber = nil
		} else {
			req.TechPassportNumber = &trimmed
		}
	}

	search.PassportSeries = req.PassportSeries
	search.PassportNumber = req.PassportNumber
	search.BirthDate = req.BirthDate
	search.Pinfl = req.Pinfl
	search.CarNumber = req.CarNumber
	search.TechPassportSeries = req.TechPassportSeries
	search.TechPassportNumber = req.TechPassportNumber

	if err := shc.db.Save(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка обновления записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": search, "success": true})
}

// PATCH /user/search-history/insurance/:id
func (shc *SearchHistoryController) PatchInsuranceSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.InsuranceSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	// Обновляем только переданные поля (нормализуем пустые строки в nil)
	if v, ok := req["passport_series"].(string); ok {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			search.PassportSeries = nil
		} else {
			search.PassportSeries = &trimmed
		}
	} else if req["passport_series"] == nil {
		search.PassportSeries = nil
	}
	if v, ok := req["passport_number"].(string); ok {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			search.PassportNumber = nil
		} else {
			search.PassportNumber = &trimmed
		}
	} else if req["passport_number"] == nil {
		search.PassportNumber = nil
	}
	if v, ok := req["birth_date"].(string); ok {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			search.BirthDate = nil
		} else {
			search.BirthDate = &trimmed
		}
	} else if req["birth_date"] == nil {
		search.BirthDate = nil
	}
	if v, ok := req["pinfl"].(string); ok {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			search.Pinfl = nil
		} else {
			search.Pinfl = &trimmed
		}
	} else if req["pinfl"] == nil {
		search.Pinfl = nil
	}
	if v, ok := req["car_number"].(string); ok {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			search.CarNumber = nil
		} else {
			search.CarNumber = &trimmed
		}
	} else if req["car_number"] == nil {
		search.CarNumber = nil
	}
	if v, ok := req["tech_passport_series"].(string); ok {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			search.TechPassportSeries = nil
		} else {
			search.TechPassportSeries = &trimmed
		}
	} else if req["tech_passport_series"] == nil {
		search.TechPassportSeries = nil
	}
	if v, ok := req["tech_passport_number"].(string); ok {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			search.TechPassportNumber = nil
		} else {
			search.TechPassportNumber = &trimmed
		}
	} else if req["tech_passport_number"] == nil {
		search.TechPassportNumber = nil
	}

	if err := shc.db.Save(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка обновления записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": search, "success": true})
}

// DELETE /user/search-history/insurance/:id
func (shc *SearchHistoryController) DeleteInsuranceSearch(c *gin.Context) {
	userID := uint(c.GetInt("user_id"))
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "Пользователь не авторизован"})
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	var search models.InsuranceSearchHistory
	if err := shc.db.Where("id = ? AND user_id = ?", id, userID).First(&search).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "Запись не найдена"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка получения записи"})
		}
		return
	}

	if err := shc.db.Delete(&search).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "Ошибка удаления записи"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": gin.H{"id": id}, "success": true, "message": "Запись удалена"})
}
