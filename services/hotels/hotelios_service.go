package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type HoteliosService struct {
	BaseURL    string
	Login      string
	Password   string
	AccessKey  string
	HTTPClient *http.Client
}

type HoteliosRequest struct {
	Login     string      `json:"login"`
	Password  string      `json:"password"`
	AccessKey string      `json:"access_key"`
	Action    string      `json:"action"`
	Version   int         `json:"version"`
	Data      interface{} `json:"data,omitempty"`
}

type HoteliosResponse struct {
	Success     bool        `json:"success"`
	Action      string      `json:"action"`
	Version     int         `json:"version"`
	Data        interface{} `json:"data,omitempty"`
	Description string      `json:"description,omitempty"`
	ErrorCode   int         `json:"errorCode,omitempty"`
}

func NewHoteliosService() *HoteliosService {
	// Временные значения для тестирования
	baseURL := os.Getenv("HOTELIOS_API_URL")
	if baseURL == "" {
		baseURL = "https://staging-api.hotelios.uz/api"
	}

	login := os.Getenv("HOTELIOS_LOGIN")
	if login == "" {
		login = "api-0002-001"
	}

	password := os.Getenv("HOTELIOS_PASSWORD")
	if password == "" {
		password = "d5f12e53a182c062b6bf30c1445153faff12269a"
	}

	accessKey := os.Getenv("HOTELIOS_ACCESS_KEY")
	if accessKey == "" {
		accessKey = "377edeb5-f452-4b10-a24d-67b977892ea9"
	}

	return &HoteliosService{
		BaseURL:    baseURL,
		Login:      login,
		Password:   password,
		AccessKey:  accessKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *HoteliosService) MakeRequest(action string, data interface{}) (*HoteliosResponse, error) {
	request := HoteliosRequest{
		Login:     s.Login,
		Password:  s.Password,
		AccessKey: s.AccessKey,
		Action:    action,
		Version:   1,
		Data:      data,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// Определяем эндпоинт в зависимости от действия
	endpoint := "/hotel"
	if action == "SearchHotels" || action == "GetAvailableRoomsByHotel" ||
		action == "MakeReservation" || action == "GetReservationStatus" ||
		action == "GetReservationDetails" || action == "ConfirmReservation" ||
		action == "CancelReservation" {
		endpoint = "/reservation"
	}

	url := s.BaseURL + endpoint
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var response HoteliosResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &response, nil
}

// Справочники
func (s *HoteliosService) GetCountryList() (*HoteliosResponse, error) {
	return s.MakeRequest("GetCountryList", nil)
}

func (s *HoteliosService) GetRegionList(countryID *int) (*HoteliosResponse, error) {
	var data interface{}
	if countryID != nil {
		data = map[string]int{"country_id": *countryID}
	}
	return s.MakeRequest("GetRegionList", data)
}

func (s *HoteliosService) GetCityList(regionID *int) (*HoteliosResponse, error) {
	var data interface{}
	if regionID != nil {
		data = map[string]int{"region_id": *regionID}
	}
	return s.MakeRequest("GetCityList", data)
}

func (s *HoteliosService) GetHotelTypeList() (*HoteliosResponse, error) {
	return s.MakeRequest("GetHotelTypeList", nil)
}

func (s *HoteliosService) GetFacilityList() (*HoteliosResponse, error) {
	return s.MakeRequest("GetFacilityList", nil)
}

func (s *HoteliosService) GetEquipmentList() (*HoteliosResponse, error) {
	return s.MakeRequest("GetEquipmentList", nil)
}

func (s *HoteliosService) GetNearbyPlacesList() (*HoteliosResponse, error) {
	return s.MakeRequest("GetNearbyPlacesTypeList", nil)
}

func (s *HoteliosService) GetFreeServicesList() (*HoteliosResponse, error) {
	return s.MakeRequest("GetServicesInRoomList", nil)
}

func (s *HoteliosService) GetBedTypeList() (*HoteliosResponse, error) {
	return s.MakeRequest("GetBedTypeList", nil)
}

func (s *HoteliosService) GetHotelStarList() (*HoteliosResponse, error) {
	return s.MakeRequest("GetStarList", nil)
}

func (s *HoteliosService) GetCurrencyList() (*HoteliosResponse, error) {
	return s.MakeRequest("GetCurrencyList", nil)
}

func (s *HoteliosService) GetMinMaxPrice() (*HoteliosResponse, error) {
	return s.MakeRequest("GetPriceRange", nil)
}

// Информация об отелях
func (s *HoteliosService) GetHotelList(hotelID *int, hotelTypeID *int) (*HoteliosResponse, error) {
	var data interface{}
	if hotelID != nil {
		data = map[string]int{"hotel_id": *hotelID}
	} else if hotelTypeID != nil {
		data = map[string]int{"hotel_type_id": *hotelTypeID}
	}
	return s.MakeRequest("GetHotelList", data)
}

func (s *HoteliosService) GetHotelPhotosList(hotelID *int) (*HoteliosResponse, error) {
	var data interface{}
	if hotelID != nil {
		data = map[string]int{"hotel_id": *hotelID}
	}
	return s.MakeRequest("GetHotelPhotoList", data)
}

func (s *HoteliosService) GetHotelFacilitiesList(hotelID *int) (*HoteliosResponse, error) {
	var data interface{}
	if hotelID != nil {
		data = map[string]int{"hotel_id": *hotelID}
	}
	return s.MakeRequest("GetHotelFacilityList", data)
}

func (s *HoteliosService) GetHotelServicesInRoomList(hotelID *int) (*HoteliosResponse, error) {
	var data interface{}
	if hotelID != nil {
		data = map[string]int{"hotel_id": *hotelID}
	}
	return s.MakeRequest("GetHotelServicesInRoomList", data)
}

func (s *HoteliosService) GetHotelNearbyPlacesList(hotelID *int) (*HoteliosResponse, error) {
	var data interface{}
	if hotelID != nil {
		data = map[string]int{"hotel_id": *hotelID}
	}
	return s.MakeRequest("GetHotelNearbyPlacesList", data)
}

// Информация о номерах
func (s *HoteliosService) GetHotelRoomTypeList(hotelID *int) (*HoteliosResponse, error) {
	var data interface{}
	if hotelID != nil {
		data = map[string]int{"hotel_id": *hotelID}
	}
	return s.MakeRequest("GetHotelRoomTypeList", data)
}

func (s *HoteliosService) GetHotelRoomTypesPhotoList(hotelID *int, roomTypeID *int) (*HoteliosResponse, error) {
	var data interface{}
	if hotelID != nil && roomTypeID != nil {
		data = map[string]int{"hotel_id": *hotelID, "room_type_id": *roomTypeID}
	} else if hotelID != nil {
		data = map[string]int{"hotel_id": *hotelID}
	} else if roomTypeID != nil {
		data = map[string]int{"room_type_id": *roomTypeID}
	}
	return s.MakeRequest("GetHotelRoomTypesPhotoList", data)
}

func (s *HoteliosService) GetRoomTypeEquipmentList(hotelID *int, roomTypeID *int) (*HoteliosResponse, error) {
	var data interface{}
	if hotelID != nil && roomTypeID != nil {
		data = map[string]int{"hotel_id": *hotelID, "room_type_id": *roomTypeID}
	} else if hotelID != nil {
		data = map[string]int{"hotel_id": *hotelID}
	} else if roomTypeID != nil {
		data = map[string]int{"room_type_id": *roomTypeID}
	}
	return s.MakeRequest("GetRoomTypeEquipmentList", data)
}

// Резервация
func (s *HoteliosService) SearchHotels(searchData interface{}) (*HoteliosResponse, error) {
	return s.MakeRequest("SearchHotels", searchData)
}

func (s *HoteliosService) GetAvailableRoomsByHotel(hotelID int, isGroup bool, searchToken int64) (*HoteliosResponse, error) {
	data := map[string]interface{}{
		"hotel_id":     hotelID,
		"is_group":     isGroup,
		"search_token": searchToken,
	}
	return s.MakeRequest("GetAvailableRoomsByHotel", data)
}

func (s *HoteliosService) MakeReservation(reservationData interface{}) (*HoteliosResponse, error) {
	return s.MakeRequest("MakeReservation", reservationData)
}

func (s *HoteliosService) GetReservationStatus(externalID string) (*HoteliosResponse, error) {
	data := map[string]string{"external_id": externalID}
	return s.MakeRequest("GetReservationStatus", data)
}

func (s *HoteliosService) GetReservationDetails(reservationID int) (*HoteliosResponse, error) {
	data := map[string]int{"reservation_id": reservationID}
	return s.MakeRequest("GetReservationDetails", data)
}

func (s *HoteliosService) ConfirmReservation(reservationID int) (*HoteliosResponse, error) {
	data := map[string]int{"reservation_id": reservationID}
	return s.MakeRequest("ConfirmReservation", data)
}

func (s *HoteliosService) CancelReservation(reservationID int) (*HoteliosResponse, error) {
	data := map[string]int{"reservation_id": reservationID}
	return s.MakeRequest("CancelReservation", data)
}
