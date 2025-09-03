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

	"kliro/models"
)

// BukharaService сервис для работы с Bukhara API
type BukharaService struct {
	baseURL     string
	email       string
	password    string
	accessToken string
	tokenExpiry time.Time
	httpClient  *http.Client
}

// NewBukharaService создает новый экземпляр сервиса
func NewBukharaService() *BukharaService {
	// Получаем данные из переменных окружения
	baseURL := os.Getenv("BUKHARA_BASE_URL")
	if baseURL == "" {
		baseURL = "https://avia-api-test.bookhara.uz" // значение по умолчанию
	}

	email := os.Getenv("BUKHARA_EMAIL")
	if email == "" {
		log.Fatal("BUKHARA_EMAIL не установлен в .env файле")
	}

	password := os.Getenv("BUKHARA_PASSWORD")
	if password == "" {
		log.Fatal("BUKHARA_PASSWORD не установлен в .env файле")
	}

	return &BukharaService{
		baseURL:  baseURL,
		email:    email,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// EnsureTokenValid проверяет и обновляет токен если нужно
func (bs *BukharaService) EnsureTokenValid() error {
	if bs.accessToken == "" || time.Now().After(bs.tokenExpiry) {
		return bs.refreshToken()
	}
	return nil
}

// refreshToken обновляет токен авторизации
func (bs *BukharaService) refreshToken() error {
	url := fmt.Sprintf("%s/api/v1/accounts/tokens", bs.baseURL)

	requestBody := map[string]string{
		"email":       bs.email,
		"password":    bs.password,
		"access_type": "avia",
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга запроса: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := bs.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp models.BukharaErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return fmt.Errorf("ошибка авторизации, статус: %d, тело: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("ошибка авторизации: %s (код: %d)", errorResp.Message, errorResp.ErrorCode)
	}

	var tokenResp models.BukharaTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("ошибка парсинга ответа: %v", err)
	}

	bs.accessToken = tokenResp.Data.Token
	// Токен действует 29 дней, устанавливаем срок действия на 28 дней для безопасности
	bs.tokenExpiry = time.Now().Add(28 * 24 * time.Hour)

	log.Printf("Токен Bukhara API обновлен, действует до: %s", bs.tokenExpiry.Format("2006-01-02 15:04:05"))
	return nil
}

// makeRequest выполняет HTTP запрос к Bukhara API (использует makeRequestWithAuth для авторизованных запросов)
func (bs *BukharaService) makeRequest(method, endpoint string, body interface{}) ([]byte, error) {
	return bs.makeRequestWithAuth(method, endpoint, body)
}

// SearchFlights выполняет поиск авиабилетов
func (bs *BukharaService) SearchFlights(searchReq models.FlightSearchRequest) (*models.FlightSearchResponse, error) {
	// Формируем query параметры для GET запроса
	queryParams := fmt.Sprintf("?directions[0][departure_airport]=%s&directions[0][arrival_airport]=%s&directions[0][date]=%s",
		searchReq.Directions[0].DepartureAirport,
		searchReq.Directions[0].ArrivalAirport,
		searchReq.Directions[0].Date)

	// Добавляем параметры для обратного направления если есть
	if len(searchReq.Directions) > 1 {
		queryParams += fmt.Sprintf("&directions[1][departure_airport]=%s&directions[1][arrival_airport]=%s&directions[1][date]=%s",
			searchReq.Directions[1].DepartureAirport,
			searchReq.Directions[1].ArrivalAirport,
			searchReq.Directions[1].Date)
	}

	// Добавляем остальные параметры
	queryParams += fmt.Sprintf("&service_class=%s&adults=%d&children=%d&infants=%d&infants_with_seat=%d",
		searchReq.ServiceClass,
		searchReq.Adults,
		searchReq.Children,
		searchReq.Infants,
		searchReq.InfantsWithSeat)

	endpoint := "/api/v1/offers" + queryParams

	responseBody, err := bs.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var baseResp models.BukharaBaseResponse
	if err := json.Unmarshal(responseBody, &baseResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %v", err)
	}

	// Преобразуем data в FlightSearchResponse
	searchResp := &models.FlightSearchResponse{}

	// Парсим data как массив FlightOffer
	if offersData, ok := baseResp.Data.([]interface{}); ok {
		for _, offerData := range offersData {
			offerBytes, err := json.Marshal(offerData)
			if err != nil {
				continue
			}

			var offer models.FlightOffer
			if err := json.Unmarshal(offerBytes, &offer); err != nil {
				continue
			}

			searchResp.Offers = append(searchResp.Offers, offer)
		}
	}

	return searchResp, nil
}

// CreateBooking создает бронирование
func (bs *BukharaService) CreateBooking(offerID string, bookingReq models.BookingRequest) (*models.BookingResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/offers/%s/booking", offerID)

	responseBody, err := bs.makeRequest("POST", endpoint, bookingReq)
	if err != nil {
		return nil, err
	}

	var baseResp models.BukharaBaseResponse
	if err := json.Unmarshal(responseBody, &baseResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %v", err)
	}

	// Преобразуем data в BookingResponse
	bookingResp := &models.BookingResponse{}

	bookingBytes, err := json.Marshal(baseResp.Data)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга данных бронирования: %v", err)
	}

	if err := json.Unmarshal(bookingBytes, bookingResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга данных бронирования: %v", err)
	}

	return bookingResp, nil
}

// PayBooking оплачивает бронирование
func (bs *BukharaService) PayBooking(bookingID string) (*models.PaymentResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/booking/%s/payment", bookingID)

	responseBody, err := bs.makeRequest("POST", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var baseResp models.BukharaBaseResponse
	if err := json.Unmarshal(responseBody, &baseResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %v", err)
	}

	// Преобразуем data в PaymentResponse
	paymentResp := &models.PaymentResponse{}

	paymentBytes, err := json.Marshal(baseResp.Data)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга данных оплаты: %v", err)
	}

	if err := json.Unmarshal(paymentBytes, paymentResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга данных оплаты: %v", err)
	}

	return paymentResp, nil
}

// GetBookingInfo получает информацию о бронировании
func (bs *BukharaService) GetBookingInfo(bookingID string) (*models.BookingResponse, error) {
	endpoint := fmt.Sprintf("/api/v1/booking/%s", bookingID)

	responseBody, err := bs.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var baseResp models.BukharaBaseResponse
	if err := json.Unmarshal(responseBody, &baseResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %v", err)
	}

	// Преобразуем data в BookingResponse
	bookingResp := &models.BookingResponse{}

	bookingBytes, err := json.Marshal(baseResp.Data)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга данных бронирования: %v", err)
	}

	if err := json.Unmarshal(bookingBytes, bookingResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга данных бронирования: %v", err)
	}

	return bookingResp, nil
}

// CancelBooking отменяет бронирование
func (bs *BukharaService) CancelBooking(bookingID string) error {
	endpoint := fmt.Sprintf("/api/v1/booking/%s/cancel", bookingID)

	_, err := bs.makeRequest("POST", endpoint, nil)
	return err
}

// GetFareRules получает правила тарифа
func (bs *BukharaService) GetFareRules(offerID string) (interface{}, error) {
	endpoint := fmt.Sprintf("/api/v1/offers/%s/rules", offerID)

	responseBody, err := bs.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var baseResp models.BukharaBaseResponse
	if err := json.Unmarshal(responseBody, &baseResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %v", err)
	}

	return baseResp.Data, nil
}

// UpdateOffer обновляет информацию об оффере
func (bs *BukharaService) UpdateOffer(offerID string) (*models.FlightOffer, error) {
	endpoint := fmt.Sprintf("/api/v1/offers/%s", offerID)

	responseBody, err := bs.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var baseResp models.BukharaBaseResponse
	if err := json.Unmarshal(responseBody, &baseResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %v", err)
	}

	// Преобразуем data в FlightOffer
	offer := &models.FlightOffer{}

	offerBytes, err := json.Marshal(baseResp.Data)
	if err != nil {
		return nil, fmt.Errorf("ошибка маршалинга данных оффера: %v", err)
	}

	if err := json.Unmarshal(offerBytes, offer); err != nil {
		return nil, fmt.Errorf("ошибка парсинга данных оффера: %v", err)
	}

	return offer, nil
}

// ExtractAirportsFromOffer извлекает аэропорты из оффера для кэширования
func (bs *BukharaService) ExtractAirportsFromOffer(offer *models.FlightOffer) []*models.Airport {
	var airports []*models.Airport

	// Извлекаем аэропорты из направлений
	for _, direction := range offer.Directions {
		// Аэропорт вылета
		if direction.Departure.Airport.Code != "" {
			airports = append(airports, &direction.Departure.Airport)
		}

		// Аэропорт прибытия
		if direction.Arrival.Airport.Code != "" {
			airports = append(airports, &direction.Arrival.Airport)
		}

		// Аэропорты из сегментов
		for _, segment := range direction.Segments {
			if segment.Departure.Airport.Code != "" {
				airports = append(airports, &segment.Departure.Airport)
			}
			if segment.Arrival.Airport.Code != "" {
				airports = append(airports, &segment.Arrival.Airport)
			}
		}
	}

	return airports
}

// GetAirportHints получает подсказки аэропортов от Bukhara API (открытый API без токена)
func (bs *BukharaService) GetAirportHints(phrase string, limit int) ([]map[string]interface{}, error) {
	// Для airport-hints используем продакшен URL, так как в тестовой среде не работает
	productionURL := "https://api.bookhara.uz"

	// Создаем HTTP клиент напрямую для этого запроса
	req, err := http.NewRequest("GET", productionURL+"/api/avia/airport-hints", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Добавляем query параметры
	q := req.URL.Query()
	q.Add("phrase", phrase)
	q.Add("limit", fmt.Sprintf("%d", limit))
	req.URL.RawQuery = q.Encode()

	// Устанавливаем заголовки (без авторизации для открытого API)
	req.Header.Set("Content-Type", "application/json")

	// Отладочная информация
	fullURL := productionURL + "/api/avia/airport-hints?" + q.Encode()
	log.Printf("DEBUG: Отправляем запрос к Bukhara PRODUCTION API: %s", fullURL)
	log.Printf("DEBUG: Заголовки: %v", req.Header)

	// Выполняем запрос
	resp, err := bs.httpClient.Do(req)
	if err != nil {
		log.Printf("DEBUG: Ошибка выполнения запроса: %v", err)
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("DEBUG: Получен ответ, статус: %d", resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("DEBUG: Ошибка чтения ответа: %v", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("DEBUG: Тело ответа: %s", string(responseBody))

	if resp.StatusCode != http.StatusOK {
		var errorResp models.BukharaErrorResponse
		if err := json.Unmarshal(responseBody, &errorResp); err != nil {
			return nil, fmt.Errorf("API error, status: %d, body: %s", resp.StatusCode, string(responseBody))
		}
		return nil, fmt.Errorf("API error: %s (код: %d)", errorResp.Message, errorResp.ErrorCode)
	}

	// Парсим ответ Bukhara API - он возвращает объект с data.airports
	var bukharaResponse struct {
		Data struct {
			Airports []map[string]interface{} `json:"airports"`
		} `json:"data"`
	}

	if err := json.Unmarshal(responseBody, &bukharaResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bukhara response: %w", err)
	}

	return bukharaResponse.Data.Airports, nil
}

// makeRequestWithAuth выполняет HTTP запрос к Bukhara API с авторизацией
func (bs *BukharaService) makeRequestWithAuth(method, endpoint string, body interface{}) ([]byte, error) {
	// Проверяем и обновляем токен если нужно
	if err := bs.EnsureTokenValid(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s%s", bs.baseURL, endpoint)

	var req *http.Request
	var err error

	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("ошибка маршалинга тела запроса: %v", err)
		}
		req, err = http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+bs.accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := bs.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp models.BukharaErrorResponse
		if err := json.Unmarshal(responseBody, &errorResp); err != nil {
			return nil, fmt.Errorf("ошибка API, статус: %d, тело: %s", resp.StatusCode, string(responseBody))
		}
		return nil, fmt.Errorf("ошибка API: %s (код: %d)", errorResp.Message, errorResp.ErrorCode)
	}

	return responseBody, nil
}
