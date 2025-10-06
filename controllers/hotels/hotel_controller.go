package hotels

import (
	"net/http"
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

// ===== СПРАВОЧНЫЕ МЕТОДЫ (v1.0) =====

// GetCountryList получает список стран
func (hc *HotelController) GetCountryList(c *gin.Context) {
	body, hit, err := hc.hoteliosService.MakeHoteliosActionRequestRaw("GetCountryList", nil)
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
	c.Data(http.StatusOK, "application/json", body)
}

// GetRegionList получает список регионов
func (hc *HotelController) GetRegionList(c *gin.Context) {
	var data interface{}
	if countryIDStr := c.Query("country_id"); countryIDStr != "" {
		if countryID, err := strconv.Atoi(countryIDStr); err == nil {
			data = map[string]int{"country_id": countryID}
		}
	}

	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetRegionList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetCityList получает список городов
func (hc *HotelController) GetCityList(c *gin.Context) {
	var data interface{}
	if regionIDStr := c.Query("region_id"); regionIDStr != "" {
		if regionID, err := strconv.Atoi(regionIDStr); err == nil {
			data = map[string]int{"region_id": regionID}
		}
	}

	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetCityList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetHotelTypeList получает список типов отелей
func (hc *HotelController) GetHotelTypeList(c *gin.Context) {
	body, hit, err := hc.hoteliosService.MakeHoteliosActionRequestRaw("GetHotelTypeList", nil)
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
	c.Data(http.StatusOK, "application/json", body)
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

	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetHotelList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetHotelPhotoList получает фотографии отеля
func (hc *HotelController) GetHotelPhotoList(c *gin.Context) {
	var data interface{}
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	}

	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetHotelPhotoList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetHotelRoomTypeList получает типы номеров отеля
func (hc *HotelController) GetHotelRoomTypeList(c *gin.Context) {
	var data interface{}
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	}

	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetHotelRoomTypeList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
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

	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetHotelRoomTypesPhotoList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetFacilityList получает список удобств
func (hc *HotelController) GetFacilityList(c *gin.Context) {
	body, hit, err := hc.hoteliosService.MakeHoteliosActionRequestRaw("GetFacilityList", nil)
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
	c.Data(http.StatusOK, "application/json", body)
}

// GetHotelFacilityList получает удобства отеля
func (hc *HotelController) GetHotelFacilityList(c *gin.Context) {
	var data interface{}
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	}

	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetHotelFacilityList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetEquipmentList получает список оборудования
func (hc *HotelController) GetEquipmentList(c *gin.Context) {
	body, hit, err := hc.hoteliosService.MakeHoteliosActionRequestRaw("GetEquipmentList", nil)
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
	c.Data(http.StatusOK, "application/json", body)
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

	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetRoomTypeEquipmentList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetPriceRange получает диапазон цен
func (hc *HotelController) GetPriceRange(c *gin.Context) {
	body, hit, err := hc.hoteliosService.MakeHoteliosActionRequestRaw("GetPriceRange", nil)
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
	c.Data(http.StatusOK, "application/json", body)
}

// GetStarList получает список звезд отелей
func (hc *HotelController) GetStarList(c *gin.Context) {
	body, hit, err := hc.hoteliosService.MakeHoteliosActionRequestRaw("GetStarList", nil)
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
	c.Data(http.StatusOK, "application/json", body)
}

// GetNearbyPlacesTypeList получает список типов ближайших мест
func (hc *HotelController) GetNearbyPlacesTypeList(c *gin.Context) {
	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetNearbyPlacesTypeList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetHotelNearbyPlacesList получает ближайшие места отеля
func (hc *HotelController) GetHotelNearbyPlacesList(c *gin.Context) {
	var data interface{}
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if hotelID, err := strconv.Atoi(hotelIDStr); err == nil {
			data = map[string]int{"hotel_id": hotelID}
		}
	}

	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetHotelNearbyPlacesList", data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
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
	c.Data(http.StatusOK, "application/json", body)
}

// ===== МЕТОДЫ BOOKING-FLOW (v1.1.0) =====

// BookingFlowSearch выполняет поиск через новый API
func (hc *HotelController) BookingFlowSearch(c *gin.Context) {
	var requestData interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		// Если нет body, используем пустой объект
		requestData = map[string]interface{}{}
	}

	response, err := hc.hoteliosService.MakeBookingFlowRequest("POST", "/api/v1/booking-flow/search", requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// BookingFlowQuote получает актуальные цены
func (hc *HotelController) BookingFlowQuote(c *gin.Context) {
	var requestData interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		requestData = map[string]interface{}{}
	}

	response, err := hc.hoteliosService.MakeBookingFlowRequest("POST", "/api/v1/booking-flow/quote", requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// BookingFlowCreate создает бронирование через новый API
func (hc *HotelController) BookingFlowCreate(c *gin.Context) {
	var requestData interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		requestData = map[string]interface{}{}
	}

	response, err := hc.hoteliosService.MakeBookingFlowRequest("POST", "/api/v1/booking-flow/booking/create", requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// BookingFlowConfirm подтверждает бронирование через новый API
func (hc *HotelController) BookingFlowConfirm(c *gin.Context) {
	var requestData interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		requestData = map[string]interface{}{}
	}

	response, err := hc.hoteliosService.MakeBookingFlowRequest("POST", "/api/v1/booking-flow/booking/confirm", requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// BookingFlowCancel отменяет бронирование через новый API
func (hc *HotelController) BookingFlowCancel(c *gin.Context) {
	var requestData interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		requestData = map[string]interface{}{}
	}

	response, err := hc.hoteliosService.MakeBookingFlowRequest("POST", "/api/v1/booking-flow/booking/cancel", requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// BookingFlowRead получает детали бронирования через новый API
func (hc *HotelController) BookingFlowRead(c *gin.Context) {
	var requestData interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		requestData = map[string]interface{}{}
	}

	response, err := hc.hoteliosService.MakeBookingFlowRequest("POST", "/api/v1/booking-flow/booking/read", requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}
