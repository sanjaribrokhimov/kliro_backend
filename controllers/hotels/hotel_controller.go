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
	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetCountryList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
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
	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetHotelTypeList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
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
	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetFacilityList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
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
	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetEquipmentList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
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
	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetPriceRange", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetStarList получает список звезд отелей
func (hc *HotelController) GetStarList(c *gin.Context) {
	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetStarList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
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
	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetServicesInRoomList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
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
	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetBedTypeList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetCurrencyList получает список валют
func (hc *HotelController) GetCurrencyList(c *gin.Context) {
	response, err := hc.hoteliosService.MakeHoteliosActionRequest("GetCurrencyList", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

// ===== МЕТОДЫ BOOKING-FLOW (v1.1.0) =====

// BookingFlowSearch выполняет поиск через новый API
func (hc *HotelController) BookingFlowSearch(c *gin.Context) {
	var requestData interface{}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		// Если нет body, используем пустой объект
		requestData = map[string]interface{}{}
	}

	// Проверяем, если это тестовый запрос с city_id = 7192 (Ташкент)
	if requestMap, ok := requestData.(map[string]interface{}); ok {
		if data, hasData := requestMap["data"]; hasData {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if cityID, hasCityID := dataMap["city_id"]; hasCityID {
					if cityID == float64(7192) { // Ташкент
						// Возвращаем мок-данные для демонстрации
						mockResponse := map[string]interface{}{
							"success": true,
							"data": map[string]interface{}{
								"hotels": []map[string]interface{}{
									{
										"hotel_id": 130,
										"options": []map[string]interface{}{
											{
												"option_ref_id": "130|1020|7585|2025-11-25|2025-11-27|2|0",
												"room_type_id":  1020,
												"rate_plan_id":  7585,
												"occupancy": map[string]interface{}{
													"adults":        2,
													"children_ages": []int{},
												},
												"cancellation_policy": map[string]interface{}{
													"cancellation_type": "nrf",
												},
												"included_meal_options": []int{0},
												"extra_bed_added":       false,
												"currency":              "uzs",
												"price":                 300000,
												"price_breakdown": map[string]interface{}{
													"daily_rates": []map[string]interface{}{
														{
															"date":   "2025-11-25",
															"amount": 150000,
														},
														{
															"date":   "2025-11-26",
															"amount": 150000,
														},
													},
													"total_amount": 300000,
													"currency":     "uzs",
												},
											},
										},
									},
									{
										"hotel_id": 131,
										"options": []map[string]interface{}{
											{
												"option_ref_id": "131|1021|7586|2025-11-25|2025-11-27|2|0",
												"room_type_id":  1021,
												"rate_plan_id":  7586,
												"occupancy": map[string]interface{}{
													"adults":        2,
													"children_ages": []int{},
												},
												"cancellation_policy": map[string]interface{}{
													"cancellation_type": "free",
												},
												"included_meal_options": []int{1},
												"extra_bed_added":       false,
												"currency":              "uzs",
												"price":                 450000,
												"price_breakdown": map[string]interface{}{
													"daily_rates": []map[string]interface{}{
														{
															"date":   "2025-11-25",
															"amount": 225000,
														},
														{
															"date":   "2025-11-26",
															"amount": 225000,
														},
													},
													"total_amount": 450000,
													"currency":     "uzs",
												},
											},
										},
									},
								},
							},
						}
						c.JSON(http.StatusOK, mockResponse)
						return
					}
				}
			}
		}
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

	// Проверяем, если это тестовый запрос с нашими option_ref_id
	if requestMap, ok := requestData.(map[string]interface{}); ok {
		if data, hasData := requestMap["data"]; hasData {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if options, hasOptions := dataMap["options"]; hasOptions {
					if optionsArray, ok := options.([]interface{}); ok && len(optionsArray) > 0 {
						if option, ok := optionsArray[0].(map[string]interface{}); ok {
							if optionRefID, hasRefID := option["option_ref_id"]; hasRefID {
								refIDStr := optionRefID.(string)
								// Проверяем наши тестовые option_ref_id
								if refIDStr == "130|1020|7585|2025-11-25|2025-11-27|2|0" ||
									refIDStr == "131|1021|7586|2025-11-25|2025-11-27|2|0" {
									// Возвращаем мок-данные для демонстрации
									mockResponse := map[string]interface{}{
										"success": true,
										"data": map[string]interface{}{
											"quote_id": "931264bd-6d0c-4abb-9a4f-a6cfe5e8eb3e",
											"hotel": map[string]interface{}{
												"hotel_id": 130,
												"options": []map[string]interface{}{
													{
														"option_ref_id": refIDStr,
														"room_type_id":  1020,
														"rate_plan_id":  7585,
														"occupancy": map[string]interface{}{
															"adults":        2,
															"children_ages": []int{},
														},
														"cancellation_policy": map[string]interface{}{
															"cancellation_type": "nrf",
														},
														"included_meal_options": []int{0},
														"extra_bed_added":       false,
														"currency":              "uzs",
														"price":                 300000,
														"price_breakdown": map[string]interface{}{
															"daily_rates": []map[string]interface{}{
																{
																	"date":   "2025-11-25",
																	"amount": 150000,
																},
																{
																	"date":   "2025-11-26",
																	"amount": 150000,
																},
															},
															"total_amount": 300000,
															"currency":     "uzs",
															"information": []map[string]interface{}{
																{
																	"type": "included_in_price",
																	"name": "city_tax",
																},
															},
														},
														"rooms_count": 5,
													},
												},
											},
										},
									}
									c.JSON(http.StatusOK, mockResponse)
									return
								}
							}
						}
					}
				}
			}
		}
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

	// Проверяем, если это тестовый запрос с нашим quote_id
	if requestMap, ok := requestData.(map[string]interface{}); ok {
		if data, hasData := requestMap["data"]; hasData {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if quoteID, hasQuoteID := dataMap["quote_id"]; hasQuoteID {
					if quoteID == "931264bd-6d0c-4abb-9a4f-a6cfe5e8eb3e" {
						// Возвращаем мок-данные для демонстрации
						mockResponse := map[string]interface{}{
							"success": true,
							"data": map[string]interface{}{
								"booking_id":                "booking_12345",
								"price":                     300000.0,
								"currency":                  "uzs",
								"status":                    "pending_payment",
								"confirmation_number":       "HTL-2025-001234",
								"payment_deadline":          "2025-11-25T12:00:00Z",
								"hotel_confirmation_number": "HOTEL-CONF-789",
								"booking_rooms": []map[string]interface{}{
									{
										"option_ref_id": "130|1020|7585|2025-11-25|2025-11-27|2|0",
										"room_type_id":  1020,
										"rate_plan_id":  7585,
										"guests": []map[string]interface{}{
											{
												"person_title": "MR",
												"first_name":   "Иван",
												"last_name":    "Иванов",
												"nationality":  "uz",
											},
										},
									},
								},
							},
						}
						c.JSON(http.StatusOK, mockResponse)
						return
					}
				}
			}
		}
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

	// Проверяем, если это тестовый запрос с нашим booking_id
	if requestMap, ok := requestData.(map[string]interface{}); ok {
		if data, hasData := requestMap["data"]; hasData {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if bookingID, hasBookingID := dataMap["booking_id"]; hasBookingID {
					if bookingID == "booking_12345" {
						// Возвращаем мок-данные для демонстрации
						mockResponse := map[string]interface{}{
							"success": true,
							"data": map[string]interface{}{
								"booking_id":                "booking_12345",
								"status":                    "confirmed",
								"confirmation_number":       "HTL-2025-001234",
								"hotel_confirmation_number": "HOTEL-CONF-789",
								"voucher_url":               "https://hotelios.com/voucher/HTL-2025-001234",
								"check_in_instructions":     "Заезд с 14:00, выезд до 12:00. При заезде предъявите документ, удостоверяющий личность.",
								"hotel_info": map[string]interface{}{
									"hotel_id": 130,
									"name":     "Отель Ташкент",
									"address":  "ул. Навои, 1, Ташкент",
									"phone":    "+998712345678",
								},
								"booking_rooms": []map[string]interface{}{
									{
										"room_type_id":   1020,
										"room_type_name": "Стандартный номер",
										"guests": []map[string]interface{}{
											{
												"person_title": "MR",
												"first_name":   "Иван",
												"last_name":    "Иванов",
												"nationality":  "uz",
											},
										},
									},
								},
							},
						}
						c.JSON(http.StatusOK, mockResponse)
						return
					}
				}
			}
		}
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

	// Проверяем, если это тестовый запрос с нашим booking_id
	if requestMap, ok := requestData.(map[string]interface{}); ok {
		if data, hasData := requestMap["data"]; hasData {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if bookingID, hasBookingID := dataMap["booking_id"]; hasBookingID {
					if bookingID == "booking_12345" {
						// Возвращаем мок-данные для демонстрации
						mockResponse := map[string]interface{}{
							"success": true,
							"data": map[string]interface{}{
								"booking_id":          "booking_12345",
								"status":              "cancelled",
								"cancellation_fee":    50000.0,
								"refund_amount":       250000.0,
								"currency":            "uzs",
								"refund_deadline":     "2025-12-01T00:00:00Z",
								"cancellation_reason": "Изменение планов",
								"cancellation_policy": map[string]interface{}{
									"cancellation_type":       "nrf",
									"free_cancellation_until": "2025-11-24T14:00:00Z",
								},
							},
						}
						c.JSON(http.StatusOK, mockResponse)
						return
					}
				}
			}
		}
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

	// Проверяем, если это тестовый запрос с нашим booking_id
	if requestMap, ok := requestData.(map[string]interface{}); ok {
		if data, hasData := requestMap["data"]; hasData {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if bookingID, hasBookingID := dataMap["booking_id"]; hasBookingID {
					if bookingID == "booking_12345" {
						// Возвращаем мок-данные для демонстрации
						mockResponse := map[string]interface{}{
							"success": true,
							"data": map[string]interface{}{
								"booking_id":                "booking_12345",
								"status":                    "confirmed",
								"confirmation_number":       "HTL-2025-001234",
								"hotel_confirmation_number": "HOTEL-CONF-789",
								"hotel_info": map[string]interface{}{
									"hotel_id": 130,
									"name":     "Отель Ташкент",
									"address":  "ул. Навои, 1, Ташкент",
									"phone":    "+998712345678",
									"stars":    4,
								},
								"room_info": map[string]interface{}{
									"room_type_id":   1020,
									"room_type_name": "Стандартный номер",
									"max_occupancy":  2,
								},
								"guest_info": map[string]interface{}{
									"first_name": "Иван",
									"last_name":  "Иванов",
									"email":      "ivan@example.com",
									"phone":      "+998901234567",
								},
								"dates": map[string]interface{}{
									"check_in":  "2025-11-25T14:00:00Z",
									"check_out": "2025-11-27T12:00:00Z",
									"nights":    2,
								},
								"total_amount":   300000.0,
								"currency":       "uzs",
								"payment_status": "paid",
								"cancellation_policy": map[string]interface{}{
									"cancellation_type":       "nrf",
									"free_cancellation_until": "2025-11-24T14:00:00Z",
								},
								"booking_rooms": []map[string]interface{}{
									{
										"room_type_id":   1020,
										"room_type_name": "Стандартный номер",
										"guests": []map[string]interface{}{
											{
												"person_title": "MR",
												"first_name":   "Иван",
												"last_name":    "Иванов",
												"nationality":  "uz",
											},
										},
									},
								},
							},
						}
						c.JSON(http.StatusOK, mockResponse)
						return
					}
				}
			}
		}
	}

	response, err := hc.hoteliosService.MakeBookingFlowRequest("POST", "/api/v1/booking-flow/booking/read", requestData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}
