package controllers

import (
	"net/http"
	"strconv"

	"kliro/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// OrderController контроллер для работы с заказами
type OrderController struct {
	db *gorm.DB
}

// NewOrderController создает новый экземпляр OrderController
func NewOrderController(db *gorm.DB) *OrderController {
	return &OrderController{db: db}
}

// CreateOrder создает новый заказ
func (oc *OrderController) CreateOrder(c *gin.Context) {
	// Получаем user_id из JWT токена
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Пользователь не авторизован",
		})
		return
	}

	userIDInt, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка получения ID пользователя",
		})
		return
	}
	userIDUint := uint(userIDInt)

	var req struct {
		OrderID     string `json:"order_id" binding:"required"`
		Category    string `json:"category" binding:"required"`
		CompanyName string `json:"company_name" binding:"required"`
		Status      string `json:"status,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Неверные данные запроса",
			"details": err.Error(),
		})
		return
	}

	// Проверяем, не существует ли уже заказ с таким order_id
	var existingOrder models.Order
	if err := oc.db.Where("order_id = ?", req.OrderID).First(&existingOrder).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Заказ с таким order_id уже существует",
		})
		return
	}

	// Устанавливаем статус по умолчанию
	if req.Status == "" {
		req.Status = "pending"
	}

	// Создаем заказ
	order := models.Order{
		UserID:      userIDUint,
		OrderID:     req.OrderID,
		Category:    req.Category,
		CompanyName: req.CompanyName,
		Status:      req.Status,
	}

	if err := oc.db.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка при создании заказа",
		})
		return
	}

	// Возвращаем созданный заказ
	response := models.OrderResponse{
		ID:          order.ID,
		UserID:      order.UserID,
		OrderID:     order.OrderID,
		Category:    order.Category,
		CompanyName: order.CompanyName,
		Status:      order.Status,
		CreatedAt:   order.CreatedAt,
		UpdatedAt:   order.UpdatedAt,
	}

	c.JSON(http.StatusCreated, gin.H{
		"result":  response,
		"success": true,
		"message": "Заказ успешно создан",
	})
}

// GetUserOrders получает заказы пользователя с пагинацией
func (oc *OrderController) GetUserOrders(c *gin.Context) {
	// Получаем user_id из JWT токена
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Пользователь не авторизован",
		})
		return
	}

	userIDInt, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка получения ID пользователя",
		})
		return
	}
	userIDUint := uint(userIDInt)

	// Параметры пагинации
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	offset := (page - 1) * limit

	// Фильтры
	category := c.Query("category")
	status := c.Query("status")
	company := c.Query("company")

	// Строим запрос
	query := oc.db.Model(&models.Order{}).Where("user_id = ?", userIDUint)

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if company != "" {
		query = query.Where("company_name ILIKE ?", "%"+company+"%")
	}

	// Получаем общее количество
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка при подсчете заказов",
		})
		return
	}

	// Получаем заказы с пагинацией
	var orders []models.Order
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка при получении заказов",
		})
		return
	}

	// Преобразуем в response
	var orderResponses []models.OrderResponse
	for _, order := range orders {
		orderResponses = append(orderResponses, models.OrderResponse{
			ID:          order.ID,
			UserID:      order.UserID,
			OrderID:     order.OrderID,
			Category:    order.Category,
			CompanyName: order.CompanyName,
			Status:      order.Status,
			CreatedAt:   order.CreatedAt,
			UpdatedAt:   order.UpdatedAt,
		})
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))

	response := models.OrderListResponse{
		Orders:     orderResponses,
		Total:      total,
		Page:       page,
		Limit:      limit,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  response,
		"success": true,
	})
}

// GetOrderByID получает заказ по ID
func (oc *OrderController) GetOrderByID(c *gin.Context) {
	orderID := c.Param("order_id")

	var order models.Order
	if err := oc.db.Where("order_id = ?", orderID).First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Заказ не найден",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Ошибка при получении заказа",
			})
		}
		return
	}

	response := models.OrderResponse{
		ID:          order.ID,
		UserID:      order.UserID,
		OrderID:     order.OrderID,
		Category:    order.Category,
		CompanyName: order.CompanyName,
		Status:      order.Status,
		CreatedAt:   order.CreatedAt,
		UpdatedAt:   order.UpdatedAt,
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  response,
		"success": true,
	})
}

// UpdateOrderStatus обновляет статус заказа
func (oc *OrderController) UpdateOrderStatus(c *gin.Context) {
	orderID := c.Param("order_id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Неверные данные запроса",
		})
		return
	}

	var order models.Order
	if err := oc.db.Where("order_id = ?", orderID).First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Заказ не найден",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Ошибка при получении заказа",
			})
		}
		return
	}

	// Обновляем статус
	if err := oc.db.Model(&order).Update("status", req.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка при обновлении статуса",
		})
		return
	}

	// Получаем обновленный заказ
	oc.db.First(&order, order.ID)

	response := models.OrderResponse{
		ID:          order.ID,
		UserID:      order.UserID,
		OrderID:     order.OrderID,
		Category:    order.Category,
		CompanyName: order.CompanyName,
		Status:      order.Status,
		CreatedAt:   order.CreatedAt,
		UpdatedAt:   order.UpdatedAt,
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  response,
		"success": true,
		"message": "Статус заказа обновлен",
	})
}

// DeleteOrder удаляет заказ (soft delete)
func (oc *OrderController) DeleteOrder(c *gin.Context) {
	orderID := c.Param("order_id")

	var order models.Order
	if err := oc.db.Where("order_id = ?", orderID).First(&order).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Заказ не найден",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Ошибка при получении заказа",
			})
		}
		return
	}

	// Soft delete
	if err := oc.db.Delete(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка при удалении заказа",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  gin.H{"order_id": orderID},
		"success": true,
		"message": "Заказ удален",
	})
}

// GetOrderStats получает статистику заказов пользователя
func (oc *OrderController) GetOrderStats(c *gin.Context) {
	// Получаем user_id из JWT токена
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Пользователь не авторизован",
		})
		return
	}

	userIDInt, ok := userID.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Ошибка получения ID пользователя",
		})
		return
	}
	userIDUint := uint(userIDInt)

	var stats struct {
		TotalOrders     int64 `json:"total_orders"`
		PendingOrders   int64 `json:"pending_orders"`
		CompletedOrders int64 `json:"completed_orders"`
		CancelledOrders int64 `json:"cancelled_orders"`
	}

	// Подсчитываем общее количество заказов
	oc.db.Model(&models.Order{}).Where("user_id = ?", userIDUint).Count(&stats.TotalOrders)

	// Подсчитываем по статусам
	oc.db.Model(&models.Order{}).Where("user_id = ? AND status = ?", userIDUint, "pending").Count(&stats.PendingOrders)
	oc.db.Model(&models.Order{}).Where("user_id = ? AND status = ?", userIDUint, "completed").Count(&stats.CompletedOrders)
	oc.db.Model(&models.Order{}).Where("user_id = ? AND status = ?", userIDUint, "cancelled").Count(&stats.CancelledOrders)

	c.JSON(http.StatusOK, gin.H{
		"result":  stats,
		"success": true,
	})
}
