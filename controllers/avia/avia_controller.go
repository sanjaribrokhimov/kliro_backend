package avia

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"kliro/models"
	"kliro/services"

	"github.com/gin-gonic/gin"
)

// AviaController контроллер для работы с авиабилетами
type AviaController struct {
	bukharaService *services.BukharaService
}

// NewAviaController создает новый экземпляр контроллера
func NewAviaController() *AviaController {
	return &AviaController{
		bukharaService: services.NewBukharaService(),
	}
}

// SearchFlights выполняет поиск авиабилетов
// @Summary Поиск авиабилетов
// @Description Поиск авиабилетов по заданным параметрам
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param request body models.FlightSearchRequest true "Параметры поиска"
// @Success 200 {object} models.FlightSearchResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /avia/search [post]
func (ac *AviaController) SearchFlights(c *gin.Context) {
	var searchReq models.FlightSearchRequest

	if err := c.ShouldBindJSON(&searchReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Неверные параметры запроса: " + err.Error(),
		})
		return
	}

	// Валидация параметров
	if len(searchReq.Directions) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Необходимо указать хотя бы одно направление",
		})
		return
	}

	if searchReq.Adults < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Количество взрослых должно быть не менее 1",
		})
		return
	}

	// Проверяем что количество младенцев не превышает количество взрослых
	if searchReq.Infants > searchReq.Adults {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Количество младенцев не может превышать количество взрослых",
		})
		return
	}

	// Проверяем общее количество пассажиров
	totalPassengers := searchReq.Adults + searchReq.Children + searchReq.Infants + searchReq.InfantsWithSeat
	if totalPassengers > 9 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Общее количество пассажиров не может превышать 9",
		})
		return
	}

	// Выполняем поиск
	searchResp, err := ac.bukharaService.SearchFlights(searchReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка поиска рейсов: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  searchResp,
		"message": "Поиск выполнен успешно",
	})
}

// CreateBooking создает бронирование
// @Summary Создание бронирования
// @Description Создание бронирования на основе выбранного оффера
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param offer_id path string true "ID оффера"
// @Param request body models.BookingRequest true "Данные для бронирования"
// @Success 200 {object} models.BookingResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /avia/offers/{offer_id}/booking [post]
func (ac *AviaController) CreateBooking(c *gin.Context) {
	offerID := c.Param("offer_id")
	if offerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID оффера обязателен",
		})
		return
	}

	var bookingReq models.BookingRequest

	if err := c.ShouldBindJSON(&bookingReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Неверные параметры запроса: " + err.Error(),
		})
		return
	}

	// Валидация данных пассажиров
	if len(bookingReq.Passengers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Необходимо указать данные пассажиров",
		})
		return
	}

	// Проверяем что количество пассажиров соответствует запросу
	// Здесь можно добавить дополнительную валидацию

	// Создаем бронирование
	bookingResp, err := ac.bukharaService.CreateBooking(offerID, bookingReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка создания бронирования: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  bookingResp,
		"message": "Бронирование создано успешно",
	})
}

// PayBooking оплачивает бронирование
// @Summary Оплата бронирования
// @Description Оплата созданного бронирования
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} models.PaymentResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /avia/booking/{booking_id}/payment [post]
func (ac *AviaController) PayBooking(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Выполняем оплату
	paymentResp, err := ac.bukharaService.PayBooking(bookingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка оплаты: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  paymentResp,
		"message": "Оплата выполнена успешно",
	})
}

// GetBookingInfo получает информацию о бронировании
// @Summary Информация о бронировании
// @Description Получение детальной информации о бронировании
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} models.BookingResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /avia/booking/{booking_id} [get]
func (ac *AviaController) GetBookingInfo(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Получаем информацию о бронировании
	bookingResp, err := ac.bukharaService.GetBookingInfo(bookingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка получения информации: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  bookingResp,
		"message": "Информация получена успешно",
	})
}

// CancelBooking отменяет бронирование
// @Summary Отмена бронирования
// @Description Отмена созданного бронирования
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /avia/booking/{booking_id}/cancel [post]
func (ac *AviaController) CancelBooking(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Отменяем бронирование
	err := ac.bukharaService.CancelBooking(bookingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка отмены бронирования: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Бронирование отменено успешно",
	})
}

// GetFareRules получает правила тарифа
// @Summary Правила тарифа
// @Description Получение правил тарифа для выбранного оффера
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param offer_id path string true "ID оффера"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /avia/offers/{offer_id}/rules [get]
func (ac *AviaController) GetFareRules(c *gin.Context) {
	offerID := c.Param("offer_id")
	if offerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID оффера обязателен",
		})
		return
	}

	// Получаем правила тарифа
	rules, err := ac.bukharaService.GetFareRules(offerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка получения правил тарифа: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  rules,
		"message": "Правила тарифа получены успешно",
	})
}

// UpdateOffer обновляет информацию об оффере
// @Summary Обновление оффера
// @Description Получение актуальной информации об оффере
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param offer_id path string true "ID оффера"
// @Success 200 {object} models.FlightOffer
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /avia/offers/{offer_id} [get]
func (ac *AviaController) UpdateOffer(c *gin.Context) {
	offerID := c.Param("offer_id")
	if offerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID оффера обязателен",
		})
		return
	}

	// Обновляем информацию об оффере
	offer, err := ac.bukharaService.UpdateOffer(offerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка обновления оффера: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  offer,
		"message": "Информация об оффере обновлена успешно",
	})
}

// GetAirportHints получает подсказки аэропортов от Bukhara API
// @Summary Подсказки аэропортов
// @Description Получение подсказок аэропортов для автодополнения
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param phrase query string true "Поисковая фраза"
// @Param limit query int false "Максимальное количество результатов (по умолчанию 8)"
// @Success 200 {object} map[string]interface{}
// @Router /avia/airport-hints [get]
func (ac *AviaController) GetAirportHints(c *gin.Context) {
	phrase := c.Query("phrase")
	if phrase == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Параметр 'phrase' обязателен для поиска",
		})
		return
	}

	limitStr := c.DefaultQuery("limit", "8")
	limit := 8
	if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
		limit = parsedLimit
	}

	hints, err := ac.bukharaService.GetAirportHints(phrase, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Ошибка получения подсказок: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  hints,
		"message": fmt.Sprintf("Найдено %d подсказок аэропортов", len(hints)),
		"phrase":  phrase,
		"limit":   limit,
	})
}

// GetServiceClasses получает доступные классы обслуживания
// @Summary Классы обслуживания
// @Description Получение доступных классов обслуживания
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /avia/service-classes [get]
func (ac *AviaController) GetServiceClasses(c *gin.Context) {
	serviceClasses := []map[string]string{
		{"code": "E", "name": "Эконом класс", "description": "Стандартный класс обслуживания"},
		{"code": "B", "name": "Бизнес класс", "description": "Премиум класс обслуживания"},
		{"code": "A", "name": "Любой класс", "description": "Поиск по всем доступным классам"},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  serviceClasses,
		"message": "Классы обслуживания получены успешно",
	})
}

// GetPassengerTypes получает типы пассажиров
// @Summary Типы пассажиров
// @Description Получение доступных типов пассажиров
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /avia/passenger-types [get]
func (ac *AviaController) GetPassengerTypes(c *gin.Context) {
	passengerTypes := []map[string]string{
		{"code": "adt", "name": "Взрослый", "description": "От 12 лет и старше"},
		{"code": "chd", "name": "Ребенок", "description": "От 2 до 12 лет"},
		{"code": "inf", "name": "Младенец без места", "description": "От 0 до 2 лет"},
		{"code": "ins", "name": "Младенец с местом", "description": "От 0 до 2 лет"},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"result":  passengerTypes,
		"message": "Типы пассажиров получены успешно",
	})
}

// HealthCheck проверка состояния сервиса
// @Summary Проверка состояния
// @Description Проверка работоспособности сервиса авиабилетов
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /avia/health [get]
func (ac *AviaController) HealthCheck(c *gin.Context) {
	// Проверяем подключение к Bukhara API
	err := ac.bukharaService.EnsureTokenValid()

	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"status":  "unavailable",
			"error":   "Сервис недоступен: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"status":    "available",
		"message":   "Сервис авиабилетов работает нормально",
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
	})
}
