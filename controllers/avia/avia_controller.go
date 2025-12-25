package avia

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
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

// getAviaSplitPercent получает процент split для авиа из .env
func getAviaSplitPercent() *float64 {
	percentStr := os.Getenv("AVIA_SPLIT_1_PERCENT")
	if percentStr == "" {
		return nil
	}
	if percent, err := strconv.ParseFloat(percentStr, 64); err == nil && percent > 0 && percent <= 100 {
		return &percent
	}
	return nil
}

// addSplitPercentToResponse добавляет процент split в JSON ответ
func addSplitPercentToAviaResponse(respBody []byte, percent *float64) []byte {
	if percent == nil || len(respBody) == 0 {
		return respBody
	}

	// Пытаемся распарсить как объект
	var responseObj map[string]interface{}
	if err := json.Unmarshal(respBody, &responseObj); err == nil && responseObj != nil {
		// Это объект - добавляем процент напрямую в объект
		responseObj["split_percent"] = *percent
		result, err := json.Marshal(responseObj)
		if err == nil {
			return result
		}
	}

	// Пытаемся распарсить как массив
	var responseArr []interface{}
	if err := json.Unmarshal(respBody, &responseArr); err == nil && responseArr != nil {
		// Это массив - оборачиваем в объект с массивом и процентом
		result := map[string]interface{}{
			"data":          responseArr,
			"split_percent": *percent,
		}
		jsonResult, err := json.Marshal(result)
		if err == nil {
			return jsonResult
		}
	}

	// Если не удалось распарсить как JSON, возвращаем как есть
	// (не оборачиваем, чтобы не сломать не-JSON ответы)
	return respBody
}

func (ac *AviaController) proxyRawWithOptions(c *gin.Context, method, endpoint string, includeQuery, includeBody bool, addSplitToResponse bool) {
	fullEndpoint := endpoint
	if includeQuery {
		if rawQuery := c.Request.URL.RawQuery; rawQuery != "" {
			separator := "?"
			if strings.Contains(fullEndpoint, "?") {
				separator = "&"
			}
			fullEndpoint += separator + rawQuery
		}
	}

	var body []byte
	if includeBody {
		data, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Не удалось прочитать тело запроса: " + err.Error(),
			})
			return
		}
		body = data
	}

	resp, err := ac.bukharaService.ProxyRequest(method, fullEndpoint, body, c.Request.Header)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	respBody := resp.Body
	if addSplitToResponse && len(respBody) > 0 {
		percent := getAviaSplitPercent()
		respBody = addSplitPercentToAviaResponse(respBody, percent)
	}

	copyProxyHeaders(c.Writer.Header(), resp.Headers)
	c.Status(resp.Status)
	if len(respBody) > 0 {
		_, _ = c.Writer.Write(respBody)
	}
}

func (ac *AviaController) proxyRaw(c *gin.Context, method, endpoint string, includeQuery, includeBody bool) {
	ac.proxyRawWithOptions(c, method, endpoint, includeQuery, includeBody, true)
}

// proxyRawPure делает полностью "сырой" прокси: без модификации тела ответа (например, без split_percent).
func (ac *AviaController) proxyRawPure(c *gin.Context, method, endpoint string, includeQuery, includeBody bool) {
	ac.proxyRawWithOptions(c, method, endpoint, includeQuery, includeBody, false)
}

func copyProxyHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		if strings.EqualFold(key, "Content-Length") || strings.EqualFold(key, "Transfer-Encoding") {
			continue
		}
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
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
	ac.proxyRaw(c, "GET", "/api/v1/offers", true, false)
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

	ac.proxyRaw(c, "POST", fmt.Sprintf("/api/v1/offers/%s/booking", offerID), true, true)
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

	ac.proxyRaw(c, "POST", fmt.Sprintf("/api/v1/booking/%s/payment", bookingID), true, true)
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

	ac.proxyRaw(c, "GET", fmt.Sprintf("/api/v1/booking/%s", bookingID), true, false)
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
// @Router /avia/booking/{booking_id}/cancel [delete]
func (ac *AviaController) CancelBooking(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	ac.proxyRaw(c, "DELETE", fmt.Sprintf("/api/v1/booking/%s/cancel", bookingID), true, false)
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

	ac.proxyRaw(c, "GET", fmt.Sprintf("/api/v1/offers/%s/rules", offerID), true, false)
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

	ac.proxyRaw(c, "GET", fmt.Sprintf("/api/v1/offers/%s", offerID), true, false)
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

	// Добавляем split процент в ответ
	if response == nil {
		response = map[string]interface{}{}
	}
	if percent := getAviaSplitPercent(); percent != nil {
		response["split_percent"] = *percent
	}

	// Возвращаем ответ от Bukhara API
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
	response := gin.H{
		"success": true,
		"result":  serviceClasses,
		"message": "Классы обслуживания получены успешно",
	}
	if percent := getAviaSplitPercent(); percent != nil {
		response["split_percent"] = *percent
	}

	c.JSON(http.StatusOK, response)
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

	response := gin.H{
		"success": true,
		"result":  passengerTypes,
		"message": "Типы пассажиров получены успешно",
	}
	if percent := getAviaSplitPercent(); percent != nil {
		response["split_percent"] = *percent
	}
	c.JSON(http.StatusOK, response)
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

	response := gin.H{
		"success":   true,
		"status":    "available",
		"message":   "Сервис авиабилетов работает нормально",
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
	}
	if percent := getAviaSplitPercent(); percent != nil {
		response["split_percent"] = *percent
	}

	c.JSON(http.StatusOK, response)
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
	ac.proxyRaw(c, "POST", "/api/v1/accounts/tokens", true, true)
}

// CheckBalance получает детали по балансу
// @Summary Получение деталей по балансу
// @Description Получение информации о балансе аккаунта
// @Tags Авиабилеты
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/accounts/check-balance [get]
func (ac *AviaController) CheckBalance(c *gin.Context) {
	ac.proxyRaw(c, "GET", "/api/v1/accounts/check-balance", true, false)
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

	ac.proxyRaw(c, "GET", fmt.Sprintf("/api/v1/offers/%s/fare-family", offerID), true, false)
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

	ac.proxyRaw(c, "GET", fmt.Sprintf("/api/v1/offers/%s", offerID), true, false)
}

// CancelUnpaidBooking отменяет неоплаченное бронирование
// @Summary Отмена неоплаченного бронирования
// @Description Отмена неоплаченного бронирования
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/cancel-unpaid [delete]
func (ac *AviaController) CancelUnpaidBooking(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	ac.proxyRawPure(c, "DELETE", fmt.Sprintf("/api/v1/booking/%s/cancel-unpaid", bookingID), true, false)
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

	ac.proxyRaw(c, "GET", fmt.Sprintf("/api/v1/booking/%s/rules", bookingID), true, false)
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

	ac.proxyRaw(c, "GET", fmt.Sprintf("/api/v1/booking/%s/check-price", bookingID), true, false)
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

	ac.proxyRaw(c, "GET", fmt.Sprintf("/api/v1/booking/%s/payment-permission", bookingID), true, false)
}

// VoidBooking возврат (VOID) оплаченных билетов
// @Summary Возврат оплаченных билетов
// @Description Возврат (VOID) оплаченных билетов
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/void [delete]
func (ac *AviaController) VoidBooking(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	ac.proxyRawPure(c, "DELETE", fmt.Sprintf("/api/v1/booking/%s/void", bookingID), true, false)
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

	ac.proxyRaw(c, "GET", fmt.Sprintf("/api/v1/booking/%s/get-refund-amounts", bookingID), true, false)
}

// AutoCancel возврат выписанных билетов со штрафом
// @Summary Возврат со штрафом
// @Description Возврат выписанных билетов со штрафом
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/auto-cancel [delete]
func (ac *AviaController) AutoCancel(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	ac.proxyRawPure(c, "DELETE", fmt.Sprintf("/api/v1/booking/%s/auto-cancel", bookingID), true, false)
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

	ac.proxyRaw(c, "GET", fmt.Sprintf("/api/v1/booking/%s/pdf-receipt", bookingID), true, false)
}

// ManualRefund отмена оплаченного заказа
// @Summary Отмена оплаченного заказа
// @Description Запрос на отмену оплаченного заказа
// @Tags Авиабилеты
// @Produce json
// @Param booking_id path string true "ID бронирования"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/booking/{booking_id}/manual-refund [delete]
func (ac *AviaController) ManualRefund(c *gin.Context) {
	bookingID := c.Param("booking_id")
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "ID бронирования обязателен",
		})
		return
	}

	ac.proxyRawPure(c, "DELETE", fmt.Sprintf("/api/v1/booking/%s/manual-refund", bookingID), true, true)
}

// GetSchedule получает расписание рейсов
// @Summary Расписание рейсов
// @Description Получение расписания рейсов
// @Tags Авиабилеты
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/services/schedule [get]
func (ac *AviaController) GetSchedule(c *gin.Context) {
	ac.proxyRaw(c, "GET", "/api/v1/services/schedule", true, false)
}

// GetVisaTypes получает типы въездных виз
// @Summary Типы въездных виз
// @Description Получение типов въездных виз для граждан Узбекистана
// @Tags Авиабилеты
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/visa-types [get]
func (ac *AviaController) GetVisaTypes(c *gin.Context) {
	ac.proxyRaw(c, "GET", "/api/v1/visa-types", true, false)
}
