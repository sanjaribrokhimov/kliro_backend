package controllers

import (
	"log"
	"strconv"
	"time"

	"kliro/models"
	"kliro/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type InsuranceProfileController struct{}

func NewInsuranceProfileController() *InsuranceProfileController {
	return &InsuranceProfileController{}
}

// POST /api/insurance-profile
func (ipc *InsuranceProfileController) CreateInsuranceProfile(c *gin.Context) {
	// Получаем user_id из JWT токена
	userID, exists := c.Get("user_id")
	if !exists {
		log.Printf("[CREATE_INSURANCE_PROFILE] User ID not found in context")
		c.JSON(401, gin.H{"result": nil, "success": false, "error": "Unauthorized"})
		return
	}

	var req models.InsuranceProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[CREATE_INSURANCE_PROFILE] Invalid request: %v", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	// Валидация продукта - только разрешенные значения
	allowedProducts := []string{"KASKO", "OSAGO", "TRAVEL", "ACCIDENT"}
	isValidProduct := false
	for _, allowedProduct := range allowedProducts {
		if req.Product == allowedProduct {
			isValidProduct = true
			break
		}
	}

	if !isValidProduct {
		log.Printf("[CREATE_INSURANCE_PROFILE] Invalid product: %s. Allowed products: KASKO, OSAGO, TRAVEL, ACCIDENT", req.Product)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Invalid product. Allowed products: KASKO, OSAGO, TRAVEL, ACCIDENT"})
		return
	}

	log.Printf("[CREATE_INSURANCE_PROFILE] Creating insurance profile for user_id=%d, product=%s, order_id=%s, date=%s",
		userID, req.Product, req.OrderID, req.Date)

	db := utils.GetDB()

	// Проверяем, что пользователь существует
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		log.Printf("[CREATE_INSURANCE_PROFILE] User not found: user_id=%d", userID)
		c.JSON(404, gin.H{"result": nil, "success": false, "error": "User not found"})
		return
	}

	// Парсим дату - поддерживаем разные форматы
	var date time.Time
	var err error

	// Пробуем разные форматы даты
	formats := []string{
		"2006-01-02", // YYYY-MM-DD
		"2006/01/02", // YYYY/MM/DD
		"02-01-2006", // DD-MM-YYYY
		"02/01/2006", // DD/MM/YYYY
		"01-02-2006", // MM-DD-YYYY
		"01/02/2006", // MM/DD/YYYY
	}

	for _, format := range formats {
		date, err = time.Parse(format, req.Date)
		if err == nil {
			break
		}
	}

	if err != nil {
		log.Printf("[CREATE_INSURANCE_PROFILE] Invalid date format: %v, received: %s", err, req.Date)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Invalid date format. Supported formats: YYYY-MM-DD, YYYY/MM/DD, DD-MM-YYYY, DD/MM/YYYY, MM-DD-YYYY, MM/DD/YYYY"})
		return
	}

	// Создаем профиль страховки
	profile := &models.InsuranceProfile{
		UserID:      uint(userID.(int)),
		Product:     req.Product,
		Date:        date,
		OrderID:     req.OrderID,
		Amount:      req.Amount,
		IsPaid:      req.IsPaid,
		DocumentURL: req.DocumentURL,
	}

	if err := db.Create(profile).Error; err != nil {
		log.Printf("[CREATE_INSURANCE_PROFILE] Error creating profile: %v", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Error creating insurance profile"})
		return
	}

	// Формируем ответ
	response := models.InsuranceProfileResponse{
		ID:          profile.ID,
		UserID:      profile.UserID,
		Product:     profile.Product,
		Date:        profile.Date,
		OrderID:     profile.OrderID,
		Amount:      profile.Amount,
		IsPaid:      profile.IsPaid,
		DocumentURL: profile.DocumentURL,
		CreatedAt:   profile.CreatedAt,
		UpdatedAt:   profile.UpdatedAt,
	}

	log.Printf("[CREATE_INSURANCE_PROFILE] Insurance profile created successfully: id=%d, user_id=%d",
		profile.ID, profile.UserID)

	c.JSON(200, gin.H{"result": response, "success": true})
}

// GET /api/insurance-profile
func (ipc *InsuranceProfileController) GetInsuranceProfiles(c *gin.Context) {
	// Получаем user_id из JWT токена
	userID, exists := c.Get("user_id")
	if !exists {
		log.Printf("[GET_INSURANCE_PROFILES] User ID not found in context")
		c.JSON(401, gin.H{"result": nil, "success": false, "error": "Unauthorized"})
		return
	}

	// Получаем параметры пагинации
	page := 1
	limit := 10

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Получаем фильтр по продукту
	product := c.Query("product")

	log.Printf("[GET_INSURANCE_PROFILES] Getting profiles for user_id=%d, page=%d, limit=%d, product=%s",
		userID, page, limit, product)

	db := utils.GetDB()

	// Проверяем, что пользователь существует
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		log.Printf("[GET_INSURANCE_PROFILES] User not found: user_id=%d", userID)
		c.JSON(404, gin.H{"result": nil, "success": false, "error": "User not found"})
		return
	}

	// Строим запрос
	query := db.Model(&models.InsuranceProfile{}).Where("user_id = ?", uint(userID.(int)))

	// Добавляем фильтр по продукту если указан
	if product != "" {
		query = query.Where("product = ?", product)
	}

	// Получаем общее количество записей
	var total int64
	if err := query.Count(&total).Error; err != nil {
		log.Printf("[GET_INSURANCE_PROFILES] Error counting profiles: %v", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Error counting profiles"})
		return
	}

	// Получаем профили с пагинацией
	var profiles []models.InsuranceProfile
	offset := (page - 1) * limit

	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&profiles).Error; err != nil {
		log.Printf("[GET_INSURANCE_PROFILES] Error fetching profiles: %v", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Error fetching profiles"})
		return
	}

	// Формируем ответы
	var responses []models.InsuranceProfileResponse
	for _, profile := range profiles {
		response := models.InsuranceProfileResponse{
			ID:          profile.ID,
			UserID:      profile.UserID,
			Product:     profile.Product,
			Date:        profile.Date,
			OrderID:     profile.OrderID,
			Amount:      profile.Amount,
			IsPaid:      profile.IsPaid,
			DocumentURL: profile.DocumentURL,
			CreatedAt:   profile.CreatedAt,
			UpdatedAt:   profile.UpdatedAt,
		}
		responses = append(responses, response)
	}

	// Вычисляем общее количество страниц
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Формируем финальный ответ
	result := models.InsuranceProfileListResponse{
		Profiles:   responses,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}

	log.Printf("[GET_INSURANCE_PROFILES] Successfully retrieved %d profiles for user_id=%d",
		len(responses), userID)

	c.JSON(200, gin.H{"result": result, "success": true})
}

// GET /api/insurance-profile/:id
func (ipc *InsuranceProfileController) GetInsuranceProfileByID(c *gin.Context) {
	// Получаем user_id из JWT токена
	userID, exists := c.Get("user_id")
	if !exists {
		log.Printf("[GET_INSURANCE_PROFILE_BY_ID] User ID not found in context")
		c.JSON(401, gin.H{"result": nil, "success": false, "error": "Unauthorized"})
		return
	}

	// Получаем ID профиля из URL
	profileIDStr := c.Param("id")
	profileID, err := strconv.ParseUint(profileIDStr, 10, 32)
	if err != nil {
		log.Printf("[GET_INSURANCE_PROFILE_BY_ID] Invalid profile ID: %s", profileIDStr)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Invalid profile ID"})
		return
	}

	log.Printf("[GET_INSURANCE_PROFILE_BY_ID] Getting profile id=%d for user_id=%d", profileID, userID)

	db := utils.GetDB()

	// Получаем профиль
	var profile models.InsuranceProfile
	if err := db.Where("id = ? AND user_id = ?", profileID, uint(userID.(int))).First(&profile).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("[GET_INSURANCE_PROFILE_BY_ID] Profile not found: id=%d, user_id=%d", profileID, userID)
			c.JSON(404, gin.H{"result": nil, "success": false, "error": "Profile not found"})
		} else {
			log.Printf("[GET_INSURANCE_PROFILE_BY_ID] Error fetching profile: %v", err)
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "Error fetching profile"})
		}
		return
	}

	// Формируем ответ
	response := models.InsuranceProfileResponse{
		ID:          profile.ID,
		UserID:      profile.UserID,
		Product:     profile.Product,
		Date:        profile.Date,
		OrderID:     profile.OrderID,
		Amount:      profile.Amount,
		IsPaid:      profile.IsPaid,
		DocumentURL: profile.DocumentURL,
		CreatedAt:   profile.CreatedAt,
		UpdatedAt:   profile.UpdatedAt,
	}

	log.Printf("[GET_INSURANCE_PROFILE_BY_ID] Successfully retrieved profile id=%d for user_id=%d",
		profile.ID, userID)

	c.JSON(200, gin.H{"result": response, "success": true})
}
