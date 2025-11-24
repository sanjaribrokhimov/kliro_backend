package hotels

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	hotels "kliro/services/hotels"

	"github.com/gin-gonic/gin"
)

// HotelController контроллер для работы с отелями
type HotelController struct {
	hoteliosService *hotels.HoteliosService
}

// NewHotelController создает новый экземпляр контроллера
func NewHotelController() *HotelController {
	return &HotelController{
		hoteliosService: hotels.NewHoteliosService(),
	}
}

// ensureBookingFlowCredentials добавляет login/password/access_key в тело запроса, если они отсутствуют
func ensureBookingFlowCredentials(raw []byte, svc *hotels.HoteliosService) []byte {
	var body map[string]interface{}
	if len(raw) == 0 {
		body = map[string]interface{}{}
	} else {
		if err := json.Unmarshal(raw, &body); err != nil || body == nil {
			body = map[string]interface{}{}
		}
	}

	if v, ok := body["login"]; !ok || v == "" {
		body["login"] = svc.GetLogin()
	}
	if v, ok := body["password"]; !ok || v == "" {
		body["password"] = svc.GetPassword()
	}
	if v, ok := body["access_key"]; !ok || v == "" {
		body["access_key"] = svc.GetAccessKey()
	}
	b, _ := json.Marshal(body)
	return b
}

// getHotelSplitPercent получает процент split для отелей из .env
func getHotelSplitPercent() *float64 {
	percentStr := os.Getenv("HOTEL_SPLIT_1_PERCENT")
	if percentStr == "" {
		return nil
	}
	if percent, err := strconv.ParseFloat(percentStr, 64); err == nil && percent > 0 && percent <= 100 {
		return &percent
	}
	return nil
}

// addSplitPercentToResponse добавляет процент split в JSON ответ
func addSplitPercentToResponse(respBody []byte, percent *float64) []byte {
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

// ===== СПРАВОЧНЫЕ МЕТОДЫ (v1.0) =====

// GetCountryList получает список стран
func (hc *HotelController) GetCountryList(c *gin.Context) {
	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetCountryList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetRegionList получает список регионов
func (hc *HotelController) GetRegionList(c *gin.Context) {
	var data interface{}
	if countryIDStr := c.Query("country_id"); countryIDStr != "" {
		if countryID, err := strconv.Atoi(countryIDStr); err == nil {
			data = map[string]int{"country_id": countryID}
		}
	}

	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetRegionList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetCityList получает список городов
func (hc *HotelController) GetCityList(c *gin.Context) {
	var data interface{}
	if regionIDStr := c.Query("region_id"); regionIDStr != "" {
		if regionID, err := strconv.Atoi(regionIDStr); err == nil {
			data = map[string]int{"region_id": regionID}
		}
	}

	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetCityList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetHotelTypeList получает список типов отелей
func (hc *HotelController) GetHotelTypeList(c *gin.Context) {
	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetHotelTypeList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetHotelList получает список отелей
func (hc *HotelController) GetHotelList(c *gin.Context) {
	var data interface{}
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	} else if hotelTypeIDStr := c.Query("hotel_type_id"); hotelTypeIDStr != "" {
		if hotelTypeID, err := strconv.Atoi(hotelTypeIDStr); err == nil {
			data = map[string]int{"hotel_type_id": hotelTypeID}
		}
	} else if mode := c.Query("mode"); mode != "" {
		data = map[string]string{"mode": mode}
	}

	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetHotelList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetHotelPhotoList получает фотографии отеля
func (hc *HotelController) GetHotelPhotoList(c *gin.Context) {
	var data interface{}
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	}

	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetHotelPhotoList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetHotelRoomTypeList получает типы номеров отеля
func (hc *HotelController) GetHotelRoomTypeList(c *gin.Context) {
	var data interface{}
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	}

	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetHotelRoomTypeList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetHotelRoomTypesPhotoList получает фотографии номеров
func (hc *HotelController) GetHotelRoomTypesPhotoList(c *gin.Context) {
	var data interface{}
	hotelIDStr := c.Query("hotel_id")
	roomTypeIDStr := c.Query("room_type_id")

	if hotelIDStr != "" && roomTypeIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			if roomTypeID, err := strconv.Atoi(roomTypeIDStr); err == nil {
				data = map[string]int{"hotel_id": hotelID, "room_type_id": roomTypeID}
			}
		}
	} else if hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	} else if roomTypeIDStr != "" {
		if roomTypeID, err := strconv.Atoi(roomTypeIDStr); err == nil {
			data = map[string]int{"room_type_id": roomTypeID}
		}
	}

	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetHotelRoomTypesPhotoList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetFacilityList получает список удобств
func (hc *HotelController) GetFacilityList(c *gin.Context) {
	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetFacilityList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetHotelFacilityList получает удобства отеля
func (hc *HotelController) GetHotelFacilityList(c *gin.Context) {
	var data interface{}
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	}

	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetHotelFacilityList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetEquipmentList получает список оборудования
func (hc *HotelController) GetEquipmentList(c *gin.Context) {
	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetEquipmentList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetRoomTypeEquipmentList получает оборудование номеров
func (hc *HotelController) GetRoomTypeEquipmentList(c *gin.Context) {
	var data interface{}
	hotelIDStr := c.Query("hotel_id")
	roomTypeIDStr := c.Query("room_type_id")

	if hotelIDStr != "" && roomTypeIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			if roomTypeID, err := strconv.Atoi(roomTypeIDStr); err == nil {
				data = map[string]int{"hotel_id": hotelID, "room_type_id": roomTypeID}
			}
		}
	} else if hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	} else if roomTypeIDStr != "" {
		if roomTypeID, err := strconv.Atoi(roomTypeIDStr); err == nil {
			data = map[string]int{"room_type_id": roomTypeID}
		}
	}

	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetRoomTypeEquipmentList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetPriceRange получает диапазон цен
func (hc *HotelController) GetPriceRange(c *gin.Context) {
	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetPriceRange", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetStarList получает список звезд отелей
func (hc *HotelController) GetStarList(c *gin.Context) {
	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetStarList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetNearbyPlacesTypeList получает список типов ближайших мест
func (hc *HotelController) GetNearbyPlacesTypeList(c *gin.Context) {
	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetNearbyPlacesTypeList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetHotelNearbyPlacesList получает ближайшие места отеля
func (hc *HotelController) GetHotelNearbyPlacesList(c *gin.Context) {
	var data interface{}
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	}

	raw, err := hc.hoteliosService.MakeHoteliosActionRequestRawNoCache("GetHotelNearbyPlacesList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	percent := getHotelSplitPercent()
	raw = addSplitPercentToResponse(raw, percent)
	c.Data(http.StatusOK, "application/json", raw)
}

// GetServicesInRoomList получает список услуг в номере
func (hc *HotelController) GetServicesInRoomList(c *gin.Context) {
	body, hit, err := hc.hoteliosService.MakeHoteliosActionRequestRaw("GetServicesInRoomList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if hit {
		c.Header("X-Cache", "HIT")
		c.Header("X-Source", "redis")
	} else {
		c.Header("X-Cache", "MISS")
		c.Header("X-Source", "hotelios")
	}
	percent := getHotelSplitPercent()
	body = addSplitPercentToResponse(body, percent)
	c.Data(http.StatusOK, "application/json", body)
}

// GetHotelServicesInRoomList получает услуги в номере отеля
func (hc *HotelController) GetHotelServicesInRoomList(c *gin.Context) {
	var data interface{}
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	}

	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetHotelServicesInRoomList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Добавляем процент split в ответ
	if percent := getHotelSplitPercent(); percent != nil {
		response["split_percent"] = *percent
	}
	c.JSON(http.StatusOK, response)
}

// GetBedTypeList получает список типов кроватей
func (hc *HotelController) GetBedTypeList(c *gin.Context) {
	body, hit, err := hc.hoteliosService.MakeHoteliosActionRequestRaw("GetBedTypeList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if hit {
		c.Header("X-Cache", "HIT")
		c.Header("X-Source", "redis")
	} else {
		c.Header("X-Cache", "MISS")
		c.Header("X-Source", "hotelios")
	}
	percent := getHotelSplitPercent()
	body = addSplitPercentToResponse(body, percent)
	c.Data(http.StatusOK, "application/json", body)
}

// GetCurrencyList получает список валют
func (hc *HotelController) GetCurrencyList(c *gin.Context) {
	body, hit, err := hc.hoteliosService.MakeHoteliosActionRequestRaw("GetCurrencyList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if hit {
		c.Header("X-Cache", "HIT")
		c.Header("X-Source", "redis")
	} else {
		c.Header("X-Cache", "MISS")
		c.Header("X-Source", "hotelios")
	}
	percent := getHotelSplitPercent()
	body = addSplitPercentToResponse(body, percent)
	c.Data(http.StatusOK, "application/json", body)
}

// ===== МЕТОДЫ BOOKING-FLOW (v1.1.0) =====

// BookingFlowSearch выполняет поиск через новый API
func (hc *HotelController) BookingFlowSearch(c *gin.Context) {
	raw, _ := c.GetRawData()
	raw = ensureBookingFlowCredentials(raw, hc.hoteliosService)
	respBody, status, err := hc.hoteliosService.MakeBookingFlowRequestRaw("POST", "/api/v1/booking-flow/search", raw, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Добавляем процент split в ответ
	percent := getHotelSplitPercent()
	respBody = addSplitPercentToResponse(respBody, percent)
	c.Data(status, "application/json", respBody)
}

// BookingFlowQuote получает актуальные цены
func (hc *HotelController) BookingFlowQuote(c *gin.Context) {
	raw, _ := c.GetRawData()
	raw = ensureBookingFlowCredentials(raw, hc.hoteliosService)
	respBody, status, err := hc.hoteliosService.MakeBookingFlowRequestRaw("POST", "/api/v1/booking-flow/quote", raw, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Добавляем процент split в ответ
	percent := getHotelSplitPercent()
	respBody = addSplitPercentToResponse(respBody, percent)
	c.Data(status, "application/json", respBody)
}

// BookingFlowCreate создает бронирование через новый API
func (hc *HotelController) BookingFlowCreate(c *gin.Context) {
	raw, _ := c.GetRawData()
	raw = ensureBookingFlowCredentials(raw, hc.hoteliosService)
	respBody, status, err := hc.hoteliosService.MakeBookingFlowRequestRaw("POST", "/api/v1/booking-flow/booking/create", raw, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Добавляем процент split в ответ
	percent := getHotelSplitPercent()
	respBody = addSplitPercentToResponse(respBody, percent)
	c.Data(status, "application/json", respBody)
}

// BookingFlowConfirm подтверждает бронирование через новый API
func (hc *HotelController) BookingFlowConfirm(c *gin.Context) {
	raw, _ := c.GetRawData()
	raw = ensureBookingFlowCredentials(raw, hc.hoteliosService)
	respBody, status, err := hc.hoteliosService.MakeBookingFlowRequestRaw("POST", "/api/v1/booking-flow/booking/confirm", raw, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Добавляем процент split в ответ
	percent := getHotelSplitPercent()
	respBody = addSplitPercentToResponse(respBody, percent)
	c.Data(status, "application/json", respBody)
}

// BookingFlowCancel отменяет бронирование через новый API
func (hc *HotelController) BookingFlowCancel(c *gin.Context) {
	raw, _ := c.GetRawData()
	raw = ensureBookingFlowCredentials(raw, hc.hoteliosService)
	respBody, status, err := hc.hoteliosService.MakeBookingFlowRequestRaw("POST", "/api/v1/booking-flow/booking/cancel", raw, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Добавляем процент split в ответ
	percent := getHotelSplitPercent()
	respBody = addSplitPercentToResponse(respBody, percent)
	c.Data(status, "application/json", respBody)
}

// BookingFlowRead получает детали бронирования через новый API
func (hc *HotelController) BookingFlowRead(c *gin.Context) {
	raw, _ := c.GetRawData()
	raw = ensureBookingFlowCredentials(raw, hc.hoteliosService)
	respBody, status, err := hc.hoteliosService.MakeBookingFlowRequestRaw("POST", "/api/v1/booking-flow/booking/read", raw, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Добавляем процент split в ответ
	percent := getHotelSplitPercent()
	respBody = addSplitPercentToResponse(respBody, percent)
	c.Data(status, "application/json", respBody)
}
