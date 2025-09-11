package avia

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	avia "kliro/services/avia"

	"github.com/gin-gonic/gin"
)

// AviaController контроллер для работы с авиабилетами
type AviaController struct {
	bukharaService *avia.BukharaService
}

// NewAviaController создает новый экземпляр контроллера
func NewAviaController() *AviaController {
	return &AviaController{
		bukharaService: avia.NewBukharaService(),
	}
}

// SearchFlights выполняет поиск авиабилетов
// @Summary Поиск авиабилетов
// @Description Поиск авиабилетов по заданным параметрам
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param directions[0][departure_airport] query string true "Код аэропорта вылета"
// @Param directions[0][arrival_airport] query string true "Код аэропорта прибытия"
// @Param directions[0][date] query string true "Дата вылета (YYYY-MM-DD)"
// @Param service_class query string true "Класс обслуживания (E, B, A)"
// @Param adults query int true "Количество взрослых"
// @Param children query int false "Количество детей"
// @Param infants query int false "Количество младенцев без места"
// @Param infants_with_seat query int false "Количество младенцев с местом"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /avia/search [get]
func (ac *AviaController) SearchFlights(c *gin.Context) {
	// Получаем параметры из query string
	departureAirport := c.Query("directions[0][departure_airport]")
	arrivalAirport := c.Query("directions[0][arrival_airport]")
	date := c.Query("directions[0][date]")
	serviceClass := c.Query("service_class")
	adults := c.DefaultQuery("adults", "1")
	children := c.DefaultQuery("children", "0")
	infants := c.DefaultQuery("infants", "0")
	infantsWithSeat := c.DefaultQuery("infants_with_seat", "0")

	// Валидация обязательных параметров
	if departureAirport == "" || arrivalAirport == "" || date == "" || serviceClass == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Необходимо указать все обязательные параметры: departure_airport, arrival_airport, date, service_class",
		})
		return
	}

	// Формируем query параметры для GET запроса
	queryParams := fmt.Sprintf("?directions[0][departure_airport]=%s&directions[0][arrival_airport]=%s&directions[0][date]=%s&service_class=%s&adults=%s&children=%s&infants=%s&infants_with_seat=%s",
		departureAirport, arrivalAirport, date, serviceClass, adults, children, infants, infantsWithSeat)

	// Проверяем наличие обратного направления
	if returnDeparture := c.Query("directions[1][departure_airport]"); returnDeparture != "" {
		returnArrival := c.Query("directions[1][arrival_airport]")
		returnDate := c.Query("directions[1][date]")

		if returnArrival != "" && returnDate != "" {
			queryParams += fmt.Sprintf("&directions[1][departure_airport]=%s&directions[1][arrival_airport]=%s&directions[1][date]=%s",
				returnDeparture, returnArrival, returnDate)
		}
	}

	endpoint := "/api/v1/offers" + queryParams

	// Проксируем запрос напрямую к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", endpoint, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка поиска рейсов: " + err.Error(),
		})
		return
	}

	// Возвращаем ответ от Bukhara API как есть
	c.JSON(http.StatusOK, response)
}

// CreateBooking создает бронирование
// @Summary Создание бронирования
// @Description Создание бронирования на основе выбранного оффера
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param offer_id path string true "ID оффера"
// @Param request body map[string]interface{} true "Данные для бронирования"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/offers/{offer_id}/booking [post]
func (ac *AviaController) CreateBooking(c *gin.Context) {
	offerID := c.Param("offer_id")
	if offerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID оффера обязателен",
		})
		return
	}

	var bookingReq map[string]interface{}
	if err := c.ShouldBindJSON(&bookingReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Неверные параметры запроса: " + err.Error(),
		})
		return
	}

	endpoint := fmt.Sprintf("/api/v1/offers/%s/booking", offerID)

	// Проксируем запрос напрямую к Bukhara API
	response, err := ac.bukharaService.MakeRequest("POST", endpoint, bookingReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка создания бронирования: " + err.Error(),
		})
		return
	}

	// Возвращаем ответ от Bukhara API как есть
	c.JSON(http.StatusOK, response)
}

// PayBooking оплачивает бронирование
// @Summary Оплата бронирования
// @Description Оплата созданного бронирования
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/payment [post]
func (ac *AviaController) PayBooking(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	endpoint := fmt.Sprintf("/api/v1/booking/%s/payment", bookingID)

	// Проксируем запрос напрямую к Bukhara API
	response, err := ac.bukharaService.MakeRequest("POST", endpoint, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка оплаты: " + err.Error(),
		})
		return
	}

	// Возвращаем ответ от Bukhara API как есть
	c.JSON(http.StatusOK, response)
}

// GetBookingInfo получает информацию о бронировании
// @Summary Информация о бронировании
// @Description Получение детальной информации о бронировании
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id} [get]
func (ac *AviaController) GetBookingInfo(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	endpoint := fmt.Sprintf("/api/v1/booking/%s", bookingID)

	// Проксируем запрос напрямую к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", endpoint, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка получения информации: " + err.Error(),
		})
		return
	}

	// Возвращаем ответ от Bukhara API как есть
	c.JSON(http.StatusOK, response)
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

	// Используем специальный метод для airport-hints (без авторизации)
	response, err := ac.bukharaService.GetAirportHints(phrase, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Ошибка получения подсказок: %v", err),
		})
		return
	}

	// Возвращаем ответ от Bukhara API как есть
	c.JSON(http.StatusOK, response)
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

// Auth выполняет авторизацию
// @Summary Авторизация
// @Description Авторизация пользователя в авиационной системе
// @Tags Авиабилеты
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Данные авторизации"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/accounts/tokens [post]
func (ac *AviaController) Auth(c *gin.Context) {
	var authReq map[string]interface{}
	if err := c.ShouldBindJSON(&authReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Неверные параметры запроса: " + err.Error(),
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.Auth(authReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка авторизации: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// CheckBalance получает детали по балансу
// @Summary Получение деталей по балансу
// @Description Получение информации о балансе аккаунта
// @Tags Авиабилеты
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/accounts/check-balance [get]
func (ac *AviaController) CheckBalance(c *gin.Context) {
	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", "/api/v1/accounts/check-balance", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка получения баланса: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetFareFamily запрашивает семейство тарифов
// @Summary Запрос семейства тарифов
// @Description Получение информации о семействе тарифов для оффера
// @Tags Авиабилеты
// @Produce json
// @Param offer_id path string true "ID оффера"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/offers/{offer_id}/fare-family [get]
func (ac *AviaController) GetFareFamily(c *gin.Context) {
	offerID := c.Param("offer_id")
	if offerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID оффера обязателен",
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", fmt.Sprintf("/api/v1/offers/%s/fare-family", offerID), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка получения семейства тарифов: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// CheckAvailability проверяет наличие мест и цену перед бронированием
// @Summary Проверка наличия мест и цены
// @Description Проверка наличия мест и актуальной цены перед бронированием
// @Tags Авиабилеты
// @Produce json
// @Param offer_id path string true "ID оффера"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/offers/{offer_id} [get]
func (ac *AviaController) CheckAvailability(c *gin.Context) {
	offerID := c.Param("offer_id")
	if offerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID оффера обязателен",
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", fmt.Sprintf("/api/v1/offers/%s", offerID), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка проверки наличия мест: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// CancelUnpaidBooking отменяет неоплаченное бронирование
// @Summary Отмена неоплаченного бронирования
// @Description Отмена неоплаченного бронирования
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/cancel-unpaid [post]
func (ac *AviaController) CancelUnpaidBooking(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("POST", fmt.Sprintf("/api/v1/booking/%s/cancel-unpaid", bookingID), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка отмены бронирования: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetBookingRules получает условия тарифа после бронирования
// @Summary Условия тарифа после бронирования
// @Description Получение условий тарифа для бронирования
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/rules [get]
func (ac *AviaController) GetBookingRules(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", fmt.Sprintf("/api/v1/booking/%s/rules", bookingID), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка получения условий тарифа: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// CheckPrice проверяет цену перед оплатой
// @Summary Проверка цены перед оплатой
// @Description Проверка актуальной цены перед оплатой бронирования
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/check-price [get]
func (ac *AviaController) CheckPrice(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", fmt.Sprintf("/api/v1/booking/%s/check-price", bookingID), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка проверки цены: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// CheckPaymentPermission проверяет возможность оплатить заказ
// @Summary Проверка возможности оплаты
// @Description Проверка возможности оплатить заказ
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/payment-permission [get]
func (ac *AviaController) CheckPaymentPermission(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", fmt.Sprintf("/api/v1/booking/%s/payment-permission", bookingID), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка проверки возможности оплаты: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// VoidBooking возврат (VOID) оплаченных билетов
// @Summary Возврат оплаченных билетов
// @Description Возврат (VOID) оплаченных билетов
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/void [post]
func (ac *AviaController) VoidBooking(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("POST", fmt.Sprintf("/api/v1/booking/%s/void", bookingID), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка возврата билетов: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetRefundAmounts получает сумму возмещения и штраф при возврате
// @Summary Сумма возмещения и штраф
// @Description Получение суммы возмещения и штрафа при возврате выписанного билета
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/get-refund-amounts [get]
func (ac *AviaController) GetRefundAmounts(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", fmt.Sprintf("/api/v1/booking/%s/get-refund-amounts", bookingID), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка получения суммы возмещения: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// AutoCancel возврат выписанных билетов со штрафом
// @Summary Возврат со штрафом
// @Description Возврат выписанных билетов со штрафом
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/auto-cancel [post]
func (ac *AviaController) AutoCancel(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("POST", fmt.Sprintf("/api/v1/booking/%s/auto-cancel", bookingID), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка возврата со штрафом: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetPDFReceipt запрашивает маршрутную квитанцию
// @Summary Маршрутная квитанция
// @Description Запрос маршрутной квитанции
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/pdf-receipt [get]
func (ac *AviaController) GetPDFReceipt(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", fmt.Sprintf("/api/v1/booking/%s/pdf-receipt", bookingID), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка получения квитанции: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ManualRefund отмена оплаченного заказа
// @Summary Отмена оплаченного заказа
// @Description Запрос на отмену оплаченного заказа
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/manual-refund [post]
func (ac *AviaController) ManualRefund(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("POST", fmt.Sprintf("/api/v1/booking/%s/manual-refund", bookingID), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка отмены заказа: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetSchedule получает расписание рейсов
// @Summary Расписание рейсов
// @Description Получение расписания рейсов
// @Tags Авиабилеты
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/services/schedule [get]
func (ac *AviaController) GetSchedule(c *gin.Context) {
	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", "/api/v1/services/schedule", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка получения расписания: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetVisaTypes получает типы въездных виз
// @Summary Типы въездных виз
// @Description Получение типов въездных виз для граждан Узбекистана
// @Tags Авиабилеты
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/visa-types [get]
func (ac *AviaController) GetVisaTypes(c *gin.Context) {
	// Проксируем запрос к Bukhara API
	response, err := ac.bukharaService.MakeRequest("GET", "/api/v1/visa-types", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "Ошибка получения типов виз: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
