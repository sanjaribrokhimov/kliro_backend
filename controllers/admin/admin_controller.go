package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"kliro/models"
	"kliro/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminController контроллер для админских функций
type AdminController struct {
	db *gorm.DB
}

// NewAdminController создает новый экземпляр AdminController
func NewAdminController(db *gorm.DB) *AdminController {
	return &AdminController{db: db}
}

// DeleteUserRequest запрос на удаление пользователя
type DeleteUserRequest struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// DeleteUser удаляет пользователя по email или phone (жёстко)
func (ac *AdminController) DeleteUser(c *gin.Context) {
	var req DeleteUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Укажите только email или только phone"})
		return
	}
	var user models.User
	tx := ac.db
	if req.Email != "" {
		tx = tx.Where("email = ?", req.Email)
	} else {
		tx = tx.Where("phone = ?", req.Phone)
	}
	if err := tx.First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Пользователь не найден"})
		return
	}
	// Жёсткое удаление
	if err := ac.db.Unscoped().Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Не удалось удалить пользователя"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// UsersList список пользователей с пагинацией и фильтрами для админки
func (ac *AdminController) UsersList(c *gin.Context) {
	pageSize := 20
	page := 1
	if v := c.Query("limit"); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 && n <= 100 {
			pageSize = n
		}
	}
	if v := c.Query("page"); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			page = n
		}
	}
	email := c.Query("email")
	phone := c.Query("phone")
	role := c.Query("role")
	confirmed := c.Query("confirmed") // "true"/"false"

	q := ac.db.Model(&models.User{})
	// Для email и phone используем частичное совпадение (hints/автодополнение)
	if email != "" {
		q = q.Where("email ILIKE ?", "%"+email+"%")
		// Для hints ограничиваем количество результатов
		if pageSize > 20 {
			pageSize = 20
		}
	}
	if phone != "" {
		q = q.Where("phone ILIKE ?", "%"+phone+"%")
		// Для hints ограничиваем количество результатов
		if pageSize > 20 {
			pageSize = 20
		}
	}
	if role != "" {
		q = q.Where("role = ?", role)
	}
	if confirmed == "true" {
		q = q.Where("confirmed = ?", true)
	}
	if confirmed == "false" {
		q = q.Where("confirmed = ?", false)
	}

	var total int64
	q.Count(&total)

	var users []models.User
	q.Order("created_at DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&users)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"users": users,
			"total": total,
			"page":  page,
			"limit": pageSize,
		},
	})
}

// ParsingStatusResponse структура ответа для статуса парсинга
type ParsingStatusResponse struct {
	ServiceName     string     `json:"service_name"`
	TotalRecords    int64      `json:"total_records"`
	LastParsingTime *time.Time `json:"last_parsing_time"`
	NextParsingTime string     `json:"next_parsing_time"`
	UpdateInterval  string     `json:"update_interval"`
	Status          string     `json:"status"`
}

// GetParsingStatus возвращает статус всех парсеров
func (ac *AdminController) GetParsingStatus(c *gin.Context) {
	fmt.Println("DEBUG: GetParsingStatus вызван!")

	if ac.db == nil {
		fmt.Println("ERROR: ac.db is nil!")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection is nil"})
		return
	}

	fmt.Println("DEBUG: ac.db не nil, продолжаем...")

	var services = []struct {
		name     string
		table    string
		schedule string
	}{
		{"Transfer", "new_transfer", "Каждый день в 02:00 (Узбекское время)"},
		{"Deposit", "new_deposit", "Каждый день в 02:00 (Узбекское время)"},
		{"Microcredit", "new_microcredit", "Каждый день в 03:00 (Узбекское время)"},
		{"Mortgage", "new_mortgage", "Каждый день в 03:00 (Узбекское время)"},
		{"Card", "new_card", "Каждый день в 03:00 (Узбекское время)"},
		{"CreditCard", "new_credit_card", "Каждый день в 03:10 (Узбекское время)"},
		{"Autocredit", "new_autocredit", "Каждый день в 03:00 (Узбекское время)"},
		{"Currency", "new_currency", "Каждые 3 часа"},
	}

	var statuses []ParsingStatusResponse

	for _, service := range services {
		var count int64
		ac.db.Table(service.table).Count(&count)

		// Получаем время последнего парсинга (максимальное CreatedAt)
		var lastParsingTime *time.Time
		var maxTime time.Time
		if err := ac.db.Table(service.table).Select("MAX(created_at)").Scan(&maxTime).Error; err == nil && !maxTime.IsZero() {
			lastParsingTime = &maxTime
		}

		// Вычисляем следующее время парсинга
		nextParsingTime := ac.calculateNextParsingTime(service.name)

		// Определяем статус
		status := "unknown"
		if lastParsingTime != nil {
			now := utils.UzbekTime()
			diff := now.Sub(*lastParsingTime)
			if diff < 24*time.Hour {
				status = "active"
			} else if diff < 48*time.Hour {
				status = "warning"
			} else {
				status = "inactive"
			}
		} else {
			status = "never_parsed"
		}

		statuses = append(statuses, ParsingStatusResponse{
			ServiceName:     service.name,
			TotalRecords:    count,
			LastParsingTime: lastParsingTime,
			NextParsingTime: nextParsingTime,
			UpdateInterval:  service.schedule,
			Status:          status,
		})
	}

	fmt.Println("DEBUG: Отправляем ответ с", len(statuses), "сервисами")

	c.JSON(http.StatusOK, gin.H{
		"result":  statuses,
		"success": true,
	})
}

// calculateNextParsingTime вычисляет следующее время парсинга
func (ac *AdminController) calculateNextParsingTime(serviceName string) string {
	now := utils.UzbekTime()

	switch serviceName {
	case "Transfer", "Deposit":
		// 02:00 каждый день
		next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next.Format("2006-01-02 15:04:05")
	case "Microcredit", "Mortgage", "Card", "Autocredit":
		// 03:00 каждый день
		next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next.Format("2006-01-02 15:04:05")
	case "CreditCard":
		// 03:10 каждый день
		next := time.Date(now.Year(), now.Month(), now.Day(), 3, 10, 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next.Format("2006-01-02 15:04:05")
	case "Currency":
		// Каждые 3 часа
		next := now.Add(3 * time.Hour)
		return next.Format("2006-01-02 15:04:05")
	default:
		return "Неизвестно"
	}
}

// GetSystemInfo возвращает системную информацию
func (ac *AdminController) GetSystemInfo(c *gin.Context) {
	fmt.Println("DEBUG: GetSystemInfo вызван!")

	info := gin.H{
		"server_time": utils.UzbekTime().Format("2006-01-02 15:04:05"),
		"timezone":    "Asia/Tashkent",
		"version":     "1.0.0",
		"status":      "running",
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  info,
		"success": true,
	})
}

// StartParsing запускает парсинг для конкретного сервиса
func (ac *AdminController) StartParsing(c *gin.Context) {
	service := c.Param("service")
	fmt.Println("DEBUG: StartParsing вызван для сервиса:", service)

	// Здесь должна быть логика запуска парсинга
	// Пока просто возвращаем успех

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"service": service,
			"status":  "started",
			"message": "Парсинг запущен для сервиса " + service,
		},
		"success": true,
	})
}

// GetServiceData возвращает данные конкретного сервиса
func (ac *AdminController) GetServiceData(c *gin.Context) {
	service := c.Param("service")
	fmt.Println("DEBUG: GetServiceData вызван для сервиса:", service)

	// Здесь должна быть логика получения данных сервиса
	// Пока просто возвращаем заглушку

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"service": service,
			"data":    "Данные сервиса " + service,
		},
		"success": true,
	})
}

// RestartAllParsers перезапускает все парсеры
func (ac *AdminController) RestartAllParsers(c *gin.Context) {
	fmt.Println("DEBUG: RestartAllParsers вызван!")

	// Здесь должна быть логика перезапуска всех парсеров
	// Пока просто возвращаем успех

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"status":  "restarted",
			"message": "Все парсеры перезапущены",
		},
		"success": true,
	})
}

// ClearAllParserData очищает все данные парсеров
func (ac *AdminController) ClearAllParserData(c *gin.Context) {
	fmt.Println("DEBUG: ClearAllParserData вызван!")

	// Здесь должна быть логика очистки всех данных
	// Пока просто возвращаем успех

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"status":  "cleared",
			"message": "Все данные парсеров очищены",
		},
		"success": true,
	})
}

// GetAviaOrders получает все заказы авиабилетов с фильтрацией и пагинацией
// @Summary Получение заказов авиабилетов
// @Description Получение всех заказов с category='avia' с фильтрацией по статусу, датам, компании и пагинацией
// @Tags Админка
// @Accept json
// @Produce json
// @Param page query int false "Номер страницы (по умолчанию 1)"
// @Param limit query int false "Количество записей на странице (по умолчанию 20, максимум 100)"
// @Param status query string false "Фильтр по статусу"
// @Param company query string false "Фильтр по названию компании"
// @Param order_id query string false "Поиск по order_id"
// @Param user_id query int false "Фильтр по ID пользователя"
// @Param date_from query string false "Начальная дата (формат: YYYY-MM-DD)"
// @Param date_to query string false "Конечная дата (формат: YYYY-MM-DD)"
// @Param last_day query bool false "Получить заказы за последний день (true/false)"
// @Success 200 {object} map[string]interface{}
// @Router /admin/orders/avia [get]
func (ac *AdminController) GetAviaOrders(c *gin.Context) {
	// Параметры пагинации
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Фильтры
	status := c.Query("status")
	company := c.Query("company")
	orderID := c.Query("order_id")
	userIDStr := c.Query("user_id")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")
	lastDayStr := c.Query("last_day")

	// Строим запрос - фильтруем только по category='avia'
	query := ac.db.Model(&models.Order{}).Where("category = ?", "avia")

	// Фильтр по статусу
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Фильтр по компании
	if company != "" {
		query = query.Where("company_name ILIKE ?", "%"+company+"%")
	}

	// Поиск по order_id
	if orderID != "" {
		query = query.Where("order_id ILIKE ?", "%"+orderID+"%")
	}

	// Фильтр по user_id
	if userIDStr != "" {
		if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			query = query.Where("user_id = ?", uint(userID))
		}
	}

	// Фильтр за последний день
	if lastDayStr == "true" {
		lastDay := time.Now().Add(-24 * time.Hour)
		query = query.Where("created_at >= ?", lastDay)
	} else {
		// Фильтрация по датам (если не используется last_day)
		if dateFrom != "" {
			fromTime, err := time.Parse("2006-01-02", dateFrom)
			if err == nil {
				query = query.Where("created_at >= ?", fromTime)
			}
		}
		if dateTo != "" {
			toTime, err := time.Parse("2006-01-02", dateTo)
			if err == nil {
				// Добавляем 23:59:59 к конечной дате
				toTime = toTime.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
				query = query.Where("created_at <= ?", toTime)
			}
		}
	}

	// Получаем общее количество
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка при подсчете заказов",
		})
		return
	}

	// Получаем заказы с пагинацией, включая информацию о пользователе
	var orders []models.Order
	if err := query.Preload("User").Order("created_at DESC").Offset(offset).Limit(limit).Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка при получении заказов",
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
		"success": true,
		"result":  response,
	})
}

// GetHotelOrders получает все заказы отелей с фильтрацией и пагинацией
// @Summary Получение заказов отелей
// @Description Получение всех заказов с category='hotel' с фильтрацией по статусу, датам, компании и пагинацией
// @Tags Админка
// @Accept json
// @Produce json
// @Param page query int false "Номер страницы (по умолчанию 1)"
// @Param limit query int false "Количество записей на странице (по умолчанию 20, максимум 100)"
// @Param status query string false "Фильтр по статусу"
// @Param company query string false "Фильтр по названию компании"
// @Param order_id query string false "Поиск по order_id"
// @Param user_id query int false "Фильтр по ID пользователя"
// @Param date_from query string false "Начальная дата (формат: YYYY-MM-DD)"
// @Param date_to query string false "Конечная дата (формат: YYYY-MM-DD)"
// @Param last_day query bool false "Получить заказы за последний день (true/false)"
// @Success 200 {object} map[string]interface{}
// @Router /admin/orders/hotel [get]
func (ac *AdminController) GetHotelOrders(c *gin.Context) {
	// Параметры пагинации
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Фильтры
	status := c.Query("status")
	company := c.Query("company")
	orderID := c.Query("order_id")
	userIDStr := c.Query("user_id")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")
	lastDayStr := c.Query("last_day")

	// Строим запрос - фильтруем только по category='hotel'
	query := ac.db.Model(&models.Order{}).Where("category = ?", "hotel")

	// Фильтр по статусу
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// Фильтр по компании
	if company != "" {
		query = query.Where("company_name ILIKE ?", "%"+company+"%")
	}

	// Поиск по order_id
	if orderID != "" {
		query = query.Where("order_id ILIKE ?", "%"+orderID+"%")
	}

	// Фильтр по user_id
	if userIDStr != "" {
		if userID, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			query = query.Where("user_id = ?", uint(userID))
		}
	}

	// Фильтр за последний день
	if lastDayStr == "true" {
		lastDay := time.Now().Add(-24 * time.Hour)
		query = query.Where("created_at >= ?", lastDay)
	} else {
		// Фильтрация по датам (если не используется last_day)
		if dateFrom != "" {
			fromTime, err := time.Parse("2006-01-02", dateFrom)
			if err == nil {
				query = query.Where("created_at >= ?", fromTime)
			}
		}
		if dateTo != "" {
			toTime, err := time.Parse("2006-01-02", dateTo)
			if err == nil {
				// Добавляем 23:59:59 к конечной дате
				toTime = toTime.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
				query = query.Where("created_at <= ?", toTime)
			}
		}
	}

	// Получаем общее количество
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка при подсчете заказов",
		})
		return
	}

	// Получаем заказы с пагинацией, включая информацию о пользователе
	var orders []models.Order
	if err := query.Preload("User").Order("created_at DESC").Offset(offset).Limit(limit).Find(&orders).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка при получении заказов",
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
		"success": true,
		"result":  response,
	})
}
