package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// HoteliosService сервис для работы с Hotelios API
type HoteliosService struct {
	baseURL    string
	login      string
	password   string
	accessKey  string
	httpClient *http.Client
}

// NewHoteliosService создает новый экземпляр сервиса
func NewHoteliosService() *HoteliosService {
	// Получаем данные из переменных окружения
	baseURL := os.Getenv("HOTELIOS_API_URL")
	if baseURL == "" {
		baseURL = "https://staging-api.hotelios.uz/api" // значение по умолчанию
	}

	login := os.Getenv("HOTELIOS_LOGIN")
	if login == "" {
		log.Fatal("HOTELIOS_LOGIN не установлен в .env файле")
	}

	password := os.Getenv("HOTELIOS_PASSWORD")
	if password == "" {
		log.Fatal("HOTELIOS_PASSWORD не установлен в .env файле")
	}

	accessKey := os.Getenv("HOTELIOS_ACCESS_KEY")
	if accessKey == "" {
		log.Fatal("HOTELIOS_ACCESS_KEY не установлен в .env файле")
	}

	// Логируем загруженные данные
	log.Printf("DEBUG: HoteliosService создан с данными - baseURL: %s, login: %s, password: %s, accessKey: %s", baseURL, login, password, accessKey)

	return &HoteliosService{
		baseURL:   baseURL,
		login:     login,
		password:  password,
		accessKey: accessKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// MakeRequest выполняет запрос к Hotelios API (простое проксирование)
func (s *HoteliosService) MakeRequest(method, endpoint string, body interface{}) (map[string]interface{}, error) {
	url := s.baseURL + endpoint

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %v", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return response, nil
}

// MakeHoteliosRequest выполняет запрос к Hotelios API - простое перенаправление
func (s *HoteliosService) MakeHoteliosRequest(method, endpoint string, body interface{}) (map[string]interface{}, error) {
	return s.MakeRequest(method, endpoint, body)
}

// MakeHoteliosActionRequest выполняет запрос к Hotelios API с action
func (s *HoteliosService) MakeHoteliosActionRequest(action string, data interface{}) (map[string]interface{}, error) {
	requestData := map[string]interface{}{
		"login":      s.login,
		"password":   s.password,
		"access_key": s.accessKey,
		"action":     action,
		"version":    1,
		"data":       data,
	}

	// Определяем эндпоинт в зависимости от действия
	endpoint := "/hotel"
	if action == "SearchHotels" || action == "GetAvailableRoomsByHotel" ||
		action == "MakeReservation" || action == "GetReservationStatus" ||
		action == "GetReservationDetails" || action == "ConfirmReservation" ||
		action == "CancelReservation" {
		endpoint = "/reservation"
	}

	return s.MakeRequest("POST", endpoint, requestData)
}

// GetLogin возвращает логин
func (s *HoteliosService) GetLogin() string {
	return s.login
}

// GetPassword возвращает пароль
func (s *HoteliosService) GetPassword() string {
	return s.password
}

// GetAccessKey возвращает access key
func (s *HoteliosService) GetAccessKey() string {
	return s.accessKey
}

// MakeBookingFlowRequest выполняет запрос к новому Booking-Flow API
func (s *HoteliosService) MakeBookingFlowRequest(method, endpoint string, body interface{}) (map[string]interface{}, error) {
	// Для нового API используем другой базовый URL
	baseURL := os.Getenv("HOTELIOS_BOOKING_FLOW_URL")
	if baseURL == "" {
		baseURL = "https://integration-staging.hotelios.uz"
	}

	url := baseURL + endpoint

	// Всегда добавляем credentials к запросу (согласно OpenAPI спецификации)
	requestData := map[string]interface{}{
		"login":      s.login,
		"password":   s.password,
		"access_key": s.accessKey,
	}

	// Добавляем данные из body
	if body != nil {
		if bodyMap, ok := body.(map[string]interface{}); ok {
			// Если body уже содержит data, используем его
			if data, hasData := bodyMap["data"]; hasData {
				requestData["data"] = data
			} else {
				// Иначе весь body становится data
				requestData["data"] = bodyMap
			}
		} else {
			requestData["data"] = body
		}
	}

	var reqBody io.Reader
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	reqBody = bytes.NewBuffer(jsonData)

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json;charset=UTF-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return response, nil
}
