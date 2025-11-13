package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"context"

	"kliro/utils"
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

// buildURL корректно собирает конечный URL с учетом наличия/отсутствия /api/v1 в baseURL и endpoint
func (s *HoteliosService) buildURL(endpoint string) string {
	base := strings.TrimRight(s.baseURL, "/")
	path := endpoint
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	baseHasV1 := strings.HasSuffix(base, "/api/v1")
	pathHasV1 := strings.HasPrefix(path, "/api/v1/")

	// Если base уже содержит /api/v1 и path тоже начинается с /api/v1 — убираем дублирование
	if baseHasV1 && pathHasV1 {
		path = strings.TrimPrefix(path, "/api/v1")
	}
	// Если base не содержит /api/v1 и path не содержит — добавляем, если это известные API-пути
	if !baseHasV1 && !pathHasV1 {
		if strings.HasPrefix(path, "/hotel") || strings.HasPrefix(path, "/reservation") || strings.HasPrefix(path, "/booking-flow/") {
			path = "/api/v1" + path
		}
	}
	return base + path
}

// MakeRequest выполняет запрос к Hotelios API (простое проксирование)
func (s *HoteliosService) MakeRequest(method, endpoint string, body interface{}) (map[string]interface{}, error) {
	url := s.buildURL(endpoint)

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

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

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

// MakeRequestRaw выполняет запрос к Hotelios API и возвращает сырые байты без изменений
func (s *HoteliosService) MakeRequestRaw(method, endpoint string, body interface{}) ([]byte, error) {
	url := s.buildURL(endpoint)

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

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	return responseBody, nil
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

	// Redis cache key per action and data
	cacheKey := "hotels:action:" + action
	if data != nil {
		if b, err := json.Marshal(data); err == nil {
			cacheKey = cacheKey + ":" + string(b)
		}
	}

	// Try cache first
	rdb := utils.GetRedis()
	if rdb != nil {
		if cached, err := rdb.Get(context.Background(), cacheKey).Bytes(); err == nil && len(cached) > 0 {
			var response map[string]interface{}
			if err := json.Unmarshal(cached, &response); err == nil {
				log.Printf("[HOTELIOS CACHE] HIT action=%s", action)
				return response, nil
			}
		}
	}

	resp, err := s.MakeRequest("POST", endpoint, requestData)
	if err != nil {
		return nil, err
	}

	// Store raw in Redis for 24h
	if rdb != nil {
		if raw, err := json.Marshal(resp); err == nil {
			_ = rdb.Set(context.Background(), cacheKey, raw, 24*time.Hour).Err()
			log.Printf("[HOTELIOS CACHE] MISS action=%s stored", action)
		}
	}

	return resp, nil
}

// MakeHoteliosActionRequestRaw делает запрос action и возвращает сырые байты, используя Redis-кэш на 24 часа
func (s *HoteliosService) MakeHoteliosActionRequestRaw(action string, data interface{}) ([]byte, bool, error) {
	requestData := map[string]interface{}{
		"login":      s.login,
		"password":   s.password,
		"access_key": s.accessKey,
		"action":     action,
		"version":    1,
		"data":       data,
	}

	endpoint := "/hotel"
	// Для некоторых action нужен эндпоинт /reservation
	if action == "SearchHotels" || action == "GetAvailableRoomsByHotel" ||
		action == "MakeReservation" || action == "GetReservationStatus" ||
		action == "GetReservationDetails" || action == "ConfirmReservation" ||
		action == "CancelReservation" {
		endpoint = "/reservation"
	}

	body, err := s.MakeRequestRaw("POST", endpoint, requestData)
	if err != nil {
		return nil, false, err
	}

	return body, false, nil
}

// MakeHoteliosActionRequestRawNoCache — прямой вызов без Redis для action-методов
func (s *HoteliosService) MakeHoteliosActionRequestRawNoCache(action string, data interface{}) ([]byte, error) {
	requestData := map[string]interface{}{
		"login":      s.login,
		"password":   s.password,
		"access_key": s.accessKey,
		"action":     action,
		"version":    1,
	}
	if data != nil {
		requestData["data"] = data
	}

	endpoint := "/hotel"
	if action == "SearchHotels" || action == "GetAvailableRoomsByHotel" ||
		action == "MakeReservation" || action == "GetReservationStatus" ||
		action == "GetReservationDetails" || action == "ConfirmReservation" ||
		action == "CancelReservation" {
		endpoint = "/reservation"
	}

	return s.MakeRequestRaw("POST", endpoint, requestData)
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
	// Используем единый базовый URL из HOTELIOS_API_URL
	url := s.buildURL(endpoint)

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

// MakeBookingFlowRequestRaw выполняет запрос к Booking-Flow API и возвращает статус + сырые байты без изменений.
// Тело запроса пробрасывается как есть (не модифицируем JSON).
func (s *HoteliosService) MakeBookingFlowRequestRaw(method, endpoint string, rawBody []byte, extraHeaders map[string]string) ([]byte, int, error) {
	url := s.buildURL(endpoint)

	var reqBody io.Reader
	if rawBody != nil {
		reqBody = bytes.NewBuffer(rawBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %v", err)
	}

	// Если явно не указан Content-Type — ставим дефолтный
	if extraHeaders == nil || extraHeaders["Content-Type"] == "" {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %v", err)
	}

	return responseBody, resp.StatusCode, nil
}
