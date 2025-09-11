package hotels

import (
	"net/http"
	"strconv"

	hotels "kliro/services/hotels"

	"github.com/gin-gonic/gin"
)

type HotelController struct {
	hoteliosService *hotels.HoteliosService
}

func NewHotelController() *HotelController {
	return &HotelController{
		hoteliosService: hotels.NewHoteliosService(),
	}
}

// Справочники
func (hc *HotelController) GetCountryList(c *gin.Context) {
	response, err := hc.hoteliosService.GetCountryList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetRegionList(c *gin.Context) {
	var countryID *int
	if countryIDStr := c.Query("country_id"); countryIDStr != "" {
		if id, err := strconv.Atoi(countryIDStr); err == nil {
			countryID = &id
		}
	}

	response, err := hc.hoteliosService.GetRegionList(countryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetCityList(c *gin.Context) {
	var regionID *int
	if regionIDStr := c.Query("region_id"); regionIDStr != "" {
		if id, err := strconv.Atoi(regionIDStr); err == nil {
			regionID = &id
		}
	}

	response, err := hc.hoteliosService.GetCityList(regionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetHotelTypeList(c *gin.Context) {
	response, err := hc.hoteliosService.GetHotelTypeList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetFacilityList(c *gin.Context) {
	response, err := hc.hoteliosService.GetFacilityList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetEquipmentList(c *gin.Context) {
	response, err := hc.hoteliosService.GetEquipmentList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetNearbyPlacesList(c *gin.Context) {
	response, err := hc.hoteliosService.GetNearbyPlacesList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetFreeServicesList(c *gin.Context) {
	response, err := hc.hoteliosService.GetFreeServicesList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetBedTypeList(c *gin.Context) {
	response, err := hc.hoteliosService.GetBedTypeList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetHotelStarList(c *gin.Context) {
	response, err := hc.hoteliosService.GetHotelStarList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetCurrencyList(c *gin.Context) {
	response, err := hc.hoteliosService.GetCurrencyList()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetMinMaxPrice(c *gin.Context) {
	response, err := hc.hoteliosService.GetMinMaxPrice()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

// Информация об отелях
func (hc *HotelController) GetHotelList(c *gin.Context) {
	var hotelID *int
	var hotelTypeID *int

	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if id, err := strconv.Atoi(hotelIDStr); err == nil {
			hotelID = &id
		}
	}

	if hotelTypeIDStr := c.Query("hotel_type_id"); hotelTypeIDStr != "" {
		if id, err := strconv.Atoi(hotelTypeIDStr); err == nil {
			hotelTypeID = &id
		}
	}

	response, err := hc.hoteliosService.GetHotelList(hotelID, hotelTypeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetHotelPhotosList(c *gin.Context) {
	var hotelID *int
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if id, err := strconv.Atoi(hotelIDStr); err == nil {
			hotelID = &id
		}
	}

	response, err := hc.hoteliosService.GetHotelPhotosList(hotelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetHotelFacilitiesList(c *gin.Context) {
	var hotelID *int
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if id, err := strconv.Atoi(hotelIDStr); err == nil {
			hotelID = &id
		}
	}

	response, err := hc.hoteliosService.GetHotelFacilitiesList(hotelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetHotelServicesInRoomList(c *gin.Context) {
	var hotelID *int
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if id, err := strconv.Atoi(hotelIDStr); err == nil {
			hotelID = &id
		}
	}

	response, err := hc.hoteliosService.GetHotelServicesInRoomList(hotelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetHotelNearbyPlacesList(c *gin.Context) {
	var hotelID *int
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if id, err := strconv.Atoi(hotelIDStr); err == nil {
			hotelID = &id
		}
	}

	response, err := hc.hoteliosService.GetHotelNearbyPlacesList(hotelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

// Информация о номерах
func (hc *HotelController) GetHotelRoomTypeList(c *gin.Context) {
	var hotelID *int
	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if id, err := strconv.Atoi(hotelIDStr); err == nil {
			hotelID = &id
		}
	}

	response, err := hc.hoteliosService.GetHotelRoomTypeList(hotelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetHotelRoomTypesPhotoList(c *gin.Context) {
	var hotelID *int
	var roomTypeID *int

	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if id, err := strconv.Atoi(hotelIDStr); err == nil {
			hotelID = &id
		}
	}

	if roomTypeIDStr := c.Query("room_type_id"); roomTypeIDStr != "" {
		if id, err := strconv.Atoi(roomTypeIDStr); err == nil {
			roomTypeID = &id
		}
	}

	response, err := hc.hoteliosService.GetHotelRoomTypesPhotoList(hotelID, roomTypeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetRoomTypeEquipmentList(c *gin.Context) {
	var hotelID *int
	var roomTypeID *int

	if hotelIDStr := c.Query("hotel_id"); hotelIDStr != "" {
		if id, err := strconv.Atoi(hotelIDStr); err == nil {
			hotelID = &id
		}
	}

	if roomTypeIDStr := c.Query("room_type_id"); roomTypeIDStr != "" {
		if id, err := strconv.Atoi(roomTypeIDStr); err == nil {
			roomTypeID = &id
		}
	}

	response, err := hc.hoteliosService.GetRoomTypeEquipmentList(hotelID, roomTypeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

// Резервация
func (hc *HotelController) SearchHotels(c *gin.Context) {
	var searchData interface{}
	if err := c.ShouldBindJSON(&searchData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := hc.hoteliosService.SearchHotels(searchData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetAvailableRoomsByHotel(c *gin.Context) {
	var requestData struct {
		HotelID     int   `json:"hotel_id" binding:"required"`
		IsGroup     bool  `json:"is_group"`
		SearchToken int64 `json:"search_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := hc.hoteliosService.GetAvailableRoomsByHotel(requestData.HotelID, requestData.IsGroup, requestData.SearchToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) MakeReservation(c *gin.Context) {
	var reservationData interface{}
	if err := c.ShouldBindJSON(&reservationData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := hc.hoteliosService.MakeReservation(reservationData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetReservationStatus(c *gin.Context) {
	var requestData struct {
		ExternalID string `json:"external_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := hc.hoteliosService.GetReservationStatus(requestData.ExternalID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) GetReservationDetails(c *gin.Context) {
	var requestData struct {
		ReservationID int `json:"reservation_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := hc.hoteliosService.GetReservationDetails(requestData.ReservationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) ConfirmReservation(c *gin.Context) {
	var requestData struct {
		ReservationID int `json:"reservation_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := hc.hoteliosService.ConfirmReservation(requestData.ReservationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}

func (hc *HotelController) CancelReservation(c *gin.Context) {
	var requestData struct {
		ReservationID int `json:"reservation_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := hc.hoteliosService.CancelReservation(requestData.ReservationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": response})
}
