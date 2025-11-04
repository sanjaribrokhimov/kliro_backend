package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Multicard - сервис для работы с платежным шлюзом Multicard
type Multicard struct {
	baseURL     string
	appID       string
	secret      string
	storeID     string
	token       string
	tokenExpiry time.Time
	client      *http.Client
}

// NewMulticard создает новый сервис Multicard
func NewMulticard() *Multicard {
	return &Multicard{
		baseURL: getEnv("MULTICARD_BASE_URL", "https://dev-mesh.multicard.uz"),
		appID:   getEnv("MULTICARD_APPLICATION_ID", ""),
		secret:  getEnv("MULTICARD_SECRET", ""),
		storeID: getEnv("MULTICARD_STORE_ID", ""),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// getEnv получает переменную окружения
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getToken получает токен авторизации
func (m *Multicard) getToken() error {
	// Проверяем, есть ли действующий токен
	if m.token != "" && time.Now().Before(m.tokenExpiry.Add(-5*time.Minute)) {
		return nil
	}

	url := fmt.Sprintf("%s/auth", m.baseURL)
	data := map[string]string{
		"application_id": m.appID,
		"secret":         m.secret,
	}

	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth failed: %s", string(body))
	}

	var result struct {
		Token  string `json:"token"`
		Expiry string `json:"expiry"`
	}

	json.NewDecoder(resp.Body).Decode(&result)
	m.token = result.Token

	// Парсим время истечения
	if expiry, err := time.Parse("2006-01-02 15:04:05", result.Expiry); err == nil {
		m.tokenExpiry = expiry
	} else {
		m.tokenExpiry = time.Now().Add(1 * time.Hour)
	}

	log.Printf("Multicard: получили токен, действует до %s", m.tokenExpiry.Format("2006-01-02 15:04:05"))
	return nil
}

// request делает запрос к Multicard API
func (m *Multicard) request(method, endpoint string, body interface{}) (*http.Response, error) {
	if err := m.getToken(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s%s", m.baseURL, endpoint)
	var reqBody io.Reader

	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.token)

	return m.client.Do(req)
}

// CreatePayment создает платеж
func (m *Multicard) CreatePayment(paymentMethod string, amount int64, invoiceID string, cardToken *string, ofd []map[string]interface{}) (map[string]interface{}, error) {
	storeIDInt, _ := strconv.Atoi(m.storeID)

	payload := map[string]interface{}{
		"amount":     amount,
		"store_id":   storeIDInt,
		"invoice_id": invoiceID,
	}

	// Для оплаты картой используем поле card, иначе payment_system
	if paymentMethod == "card" && cardToken != nil {
		payload["card"] = map[string]string{"token": *cardToken}
	} else {
		payload["payment_system"] = paymentMethod
	}

	// Добавляем OFD данные если есть
	if len(ofd) > 0 {
		payload["ofd"] = ofd
	}

	resp, err := m.request("POST", "/payment", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Details string `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		errMsg := "unknown error"
		if result.Error != nil {
			errMsg = result.Error.Details
		}
		return nil, fmt.Errorf("payment creation failed: %s", errMsg)
	}

	return result.Data, nil
}

// GetPaymentStatus получает статус платежа
func (m *Multicard) GetPaymentStatus(uuid string) (map[string]interface{}, error) {
	resp, err := m.request("GET", fmt.Sprintf("/payment/%s", uuid), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Details string `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		errMsg := "unknown error"
		if result.Error != nil {
			errMsg = result.Error.Details
		}
		return nil, fmt.Errorf("failed to get status: %s", errMsg)
	}

	return result.Data, nil
}

// ConfirmPayment подтверждает платеж с OTP
func (m *Multicard) ConfirmPayment(uuid, otp string) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"otp":             otp,
		"debit_available": false,
	}

	resp, err := m.request("PUT", fmt.Sprintf("/payment/%s", uuid), payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Details string `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		errMsg := "unknown error"
		if result.Error != nil {
			errMsg = result.Error.Details
		}
		return nil, fmt.Errorf("confirmation failed: %s", errMsg)
	}

	return result.Data, nil
}

// CancelPayment отменяет платеж
func (m *Multicard) CancelPayment(uuid string) error {
	resp, err := m.request("DELETE", fmt.Sprintf("/payment/%s", uuid), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Success bool `json:"success"`
		Error   *struct {
			Code    string `json:"code"`
			Details string `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		errMsg := "unknown error"
		if result.Error != nil {
			errMsg = result.Error.Details
		}
		return fmt.Errorf("cancellation failed: %s", errMsg)
	}

	return nil
}

// BindCard создает сессию для привязки карты
func (m *Multicard) BindCard(phone, returnURL string) (map[string]interface{}, error) {
	storeIDInt, _ := strconv.Atoi(m.storeID)

	payload := map[string]interface{}{
		"store_id":             storeIDInt,
		"redirect_url":         returnURL,
		"redirect_decline_url": returnURL,
	}

	if phone != "" {
		payload["phone"] = phone
	}

	callbackURL := getEnv("MULTICARD_CALLBACK_URL", "")
	if callbackURL != "" {
		payload["callback_url"] = callbackURL + "/cards/bind"
	}

	resp, err := m.request("POST", "/payment/card/bind", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Details string `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		errMsg := "unknown error"
		if result.Error != nil {
			errMsg = result.Error.Details
		}
		return nil, fmt.Errorf("card binding failed: %s", errMsg)
	}

	return result.Data, nil
}

// GetCardBindingStatus проверяет статус привязки карты
func (m *Multicard) GetCardBindingStatus(sessionID string) (map[string]interface{}, error) {
	resp, err := m.request("GET", fmt.Sprintf("/payment/card/bind/%s", sessionID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
		Error   *struct {
			Code    string `json:"code"`
			Details string `json:"details"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Success {
		errMsg := "unknown error"
		if result.Error != nil {
			errMsg = result.Error.Details
		}
		return nil, fmt.Errorf("failed to get card status: %s", errMsg)
	}

	return result.Data, nil
}
