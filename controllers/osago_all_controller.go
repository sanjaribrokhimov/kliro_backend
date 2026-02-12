package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"kliro/config"
	"kliro/utils"
)

// asString converts common JSON-unmarshaled types to string (string/float64/json.Number/int/etc).
// Useful when external APIs sometimes return numeric IDs that we need as strings (e.g., PINFL).
func asString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		// JSON numbers become float64 when unmarshaled into interface{}
		// PINFL is an integer-like identifier, so we keep 0 decimals.
		return strings.TrimSpace(strconv.FormatFloat(t, 'f', 0, 64))
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return strings.TrimSpace(t.String())
	default:
		return ""
	}
}

type OsagoAllController struct {
	cfg *config.Config
	cl  *http.Client
}

func NewOsagoAllController(cfg *config.Config) *OsagoAllController {
	return &OsagoAllController{
		cfg: cfg,
		cl:  &http.Client{Timeout: 30 * time.Second},
	}
}

// FindRequest - структура запроса для поиска
type FindRequest struct {
	// Поля для поиска машины
	LicensePlate       *string `json:"license_plate,omitempty"`
	TechPassportNumber *string `json:"tech_passport_number,omitempty"`
	TechPassportSeries *string `json:"tech_passport_series,omitempty"`

	// Поля для поиска человека
	Birthdate      *string `json:"birthdate,omitempty"`
	PassportNumber *string `json:"passport_number,omitempty"`
	PassportSeries *string `json:"passport_series,omitempty"`
	Pinfl          *string `json:"pinfl,omitempty"`

	// Поля для поиска организации
	Inn *string `json:"inn,omitempty"`
}

// FindResponse - структура ответа
type FindResponse struct {
	SessionID     string      `json:"session_id,omitempty"`
	Owner         *bool       `json:"owner,omitempty"`
	Vehicle       interface{} `json:"vehicle,omitempty"`
	Person        interface{} `json:"person,omitempty"`
	Organization  interface{} `json:"organization,omitempty"`
	Errors        []string    `json:"errors,omitempty"`
}

func (oc *OsagoAllController) makeExternalRequest(method, path string, bodyData interface{}) ([]byte, int, error) {
	url := oc.cfg.EuroasiaAllBaseURL + path

	var body io.Reader
	if bodyData != nil {
		jsonData, err := json.Marshal(bodyData)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("Accept-Language", "ru")
	req.Header.Set("Authorization", oc.cfg.EuroasiaAllAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := oc.cl.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return respBody, resp.StatusCode, nil
}

// Find - универсальный метод для поиска машины и/или человека.
// Единственный endpoint в группе /osago-all (кроме этого контроллера).
func (oc *OsagoAllController) Find(c *gin.Context) {
	var req FindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	response := FindResponse{
		Errors: []string{},
	}

	// Проверяем, есть ли данные для поиска машины
	hasVehicleData := req.LicensePlate != nil || req.TechPassportNumber != nil || req.TechPassportSeries != nil

	// Проверяем, есть ли данные для поиска человека/организации
	hasPersonData := req.Birthdate != nil || req.PassportNumber != nil || req.PassportSeries != nil || req.Pinfl != nil
	hasOrganizationData := req.Inn != nil

	// Не допускаем одновременный поиск person и organization в одном запросе
	if hasPersonData && hasOrganizationData {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите либо данные person (birthdate/passport/pinfl), либо inn для organization"})
		return
	}

	if !hasVehicleData && !hasPersonData && !hasOrganizationData {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one search parameter is required"})
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Поиск машины (параллельно)
	if hasVehicleData {
		wg.Add(1)
		go func() {
			defer wg.Done()

			vehicleReq := map[string]string{}
			if req.LicensePlate != nil {
				vehicleReq["license_plate"] = *req.LicensePlate
			}
			if req.TechPassportNumber != nil {
				vehicleReq["tech_passport_number"] = *req.TechPassportNumber
			}
			if req.TechPassportSeries != nil {
				vehicleReq["tech_passport_series"] = *req.TechPassportSeries
			}

			vehicleBody, statusCode, err := oc.makeExternalRequest(
				http.MethodPost,
				"/api/v1/insurance/vehicles/find",
				vehicleReq,
			)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				response.Errors = append(response.Errors, "vehicle search error: "+err.Error())
				return
			}

			var vehicleData interface{}
			if err := json.Unmarshal(vehicleBody, &vehicleData); err != nil {
				response.Errors = append(response.Errors, "vehicle response parse error: "+err.Error())
				return
			}

			if statusCode == http.StatusOK {
				response.Vehicle = vehicleData
			} else {
				response.Errors = append(response.Errors, "vehicle search failed with status "+strconv.Itoa(statusCode))
			}
		}()
	}

	// Поиск человека (параллельно)
	if hasPersonData {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var personPath string
			personReq := map[string]string{}

			// Если есть ПИНФЛ, используем find-by-pinfl
			if req.Pinfl != nil {
				personPath = "/api/v1/insurance/persons/find-by-pinfl"
				personReq["pinfl"] = *req.Pinfl
				if req.PassportNumber != nil {
					personReq["passport_number"] = *req.PassportNumber
				}
				if req.PassportSeries != nil {
					personReq["passport_series"] = *req.PassportSeries
				}
			} else {
				// Иначе используем find-by-birthdate
				personPath = "/api/v1/insurance/persons/find-by-birthdate"
				if req.Birthdate != nil {
					personReq["birthdate"] = *req.Birthdate
				}
				if req.PassportNumber != nil {
					personReq["passport_number"] = *req.PassportNumber
				}
				if req.PassportSeries != nil {
					personReq["passport_series"] = *req.PassportSeries
				}
			}

			personBody, statusCode, err := oc.makeExternalRequest(
				http.MethodPost,
				personPath,
				personReq,
			)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				response.Errors = append(response.Errors, "person search error: "+err.Error())
				return
			}

			var personData interface{}
			if err := json.Unmarshal(personBody, &personData); err != nil {
				response.Errors = append(response.Errors, "person response parse error: "+err.Error())
				return
			}

			if statusCode == http.StatusOK {
				response.Person = personData
			} else {
				response.Errors = append(response.Errors, "person search failed with status "+strconv.Itoa(statusCode))
			}
		}()
	}

	// Поиск организации (параллельно)
	if hasOrganizationData {
		wg.Add(1)
		go func() {
			defer wg.Done()

			orgReq := map[string]string{
				"inn": strings.TrimSpace(*req.Inn),
			}

			orgBody, statusCode, err := oc.makeExternalRequest(
				http.MethodPost,
				"/api/v1/insurance/organizations/find-by-inn",
				orgReq,
			)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				response.Errors = append(response.Errors, "organization search error: "+err.Error())
				return
			}

			var orgData interface{}
			if err := json.Unmarshal(orgBody, &orgData); err != nil {
				response.Errors = append(response.Errors, "organization response parse error: "+err.Error())
				return
			}

			if statusCode == http.StatusOK {
				response.Organization = orgData
			} else {
				response.Errors = append(response.Errors, "organization search failed with status "+strconv.Itoa(statusCode))
			}
		}()
	}

	// Ждем завершения всех запросов
	wg.Wait()

	// Проверяем, является ли найденный субъект владельцем машины
	if response.Vehicle != nil && response.Person != nil {
		owner := oc.checkOwnerPerson(response.Vehicle, response.Person)
		response.Owner = &owner
	} else if response.Vehicle != nil && response.Organization != nil {
		owner := oc.checkOwnerOrganization(response.Vehicle, response.Organization)
		response.Owner = &owner
	}

	// Генерируем session_id и сохраняем данные в Redis
	sessionID := uuid.New().String()
	response.SessionID = sessionID

	// Сохраняем все данные в Redis
	rdb := utils.GetRedis()
	if rdb != nil {
		ctx := context.Background()
		redisKey := "osago_all:session:" + sessionID

		sessionData := map[string]interface{}{
			"vehicle": response.Vehicle,
			"person":  response.Person,
			"organization": response.Organization,
			"owner":   response.Owner,
		}

		sessionDataJSON, err := json.Marshal(sessionData)
		if err == nil {
			rdb.Set(ctx, redisKey, sessionDataJSON, 30*time.Minute)
		}
	}

	// Возвращаем ответ
	if len(response.Errors) > 0 && response.Vehicle == nil && response.Person == nil && response.Organization == nil {
		c.JSON(http.StatusBadGateway, response)
		return
	}

	c.JSON(http.StatusOK, response)
}

// checkOwnerPerson проверяет, является ли человек владельцем машины по паспортным данным
func (oc *OsagoAllController) checkOwnerPerson(vehicleData, personData interface{}) bool {
	// Преобразуем в map для доступа к данным
	vehicleMap, ok := vehicleData.(map[string]interface{})
	if !ok {
		return false
	}

	personMap, ok := personData.(map[string]interface{})
	if !ok {
		return false
	}

	// Извлекаем паспортные данные из person
	var personPassportNumber, personPassportSeries string

	if data, ok := personMap["data"].(map[string]interface{}); ok {
		if passport, ok := data["passport"].(map[string]interface{}); ok {
			if number, ok := passport["number"].(string); ok {
				personPassportNumber = number
			}
			if series, ok := passport["series"].(string); ok {
				personPassportSeries = series
			}
		}
	}

	// Извлекаем паспортные данные из vehicle.owner.person
	var vehicleOwnerPassportNumber, vehicleOwnerPassportSeries string

	if data, ok := vehicleMap["data"].(map[string]interface{}); ok {
		if owner, ok := data["owner"].(map[string]interface{}); ok {
			if ownerType, ok := owner["type"].(string); ok && ownerType == "person" {
				if person, ok := owner["person"].(map[string]interface{}); ok {
					if passport, ok := person["passport"].(map[string]interface{}); ok {
						if number, ok := passport["number"].(string); ok {
							vehicleOwnerPassportNumber = number
						}
						if series, ok := passport["series"].(string); ok {
							vehicleOwnerPassportSeries = series
						}
					}
				}
			}
		}
	}

	// Сравниваем паспортные данные
	if personPassportNumber != "" && personPassportSeries != "" &&
		vehicleOwnerPassportNumber != "" && vehicleOwnerPassportSeries != "" {
		return personPassportNumber == vehicleOwnerPassportNumber &&
			personPassportSeries == vehicleOwnerPassportSeries
	}

	return false
}

// checkOwnerOrganization проверяет, является ли организация владельцем машины по ИНН.
// Ожидаемые структуры (сырой ответ):
// - vehicle.data.owner.organization.inn
// - organization.data.inn
func (oc *OsagoAllController) checkOwnerOrganization(vehicleData, organizationData interface{}) bool {
	vehicleMap, ok := vehicleData.(map[string]interface{})
	if !ok {
		return false
	}
	orgMap, ok := organizationData.(map[string]interface{})
	if !ok {
		return false
	}

	// inn организации из organization response
	var orgInn string
	if data, ok := orgMap["data"].(map[string]interface{}); ok {
		orgInn = asString(data["inn"])
	} else {
		orgInn = asString(orgMap["inn"])
	}

	// inn владельца из vehicle response
	var ownerInn string
	if data, ok := vehicleMap["data"].(map[string]interface{}); ok {
		if owner, ok := data["owner"].(map[string]interface{}); ok {
			if ownerType, ok := owner["type"].(string); ok && ownerType == "organization" {
				if org, ok := owner["organization"].(map[string]interface{}); ok {
					ownerInn = asString(org["inn"])
				}
			}
		}
	}

	if orgInn == "" || ownerInn == "" {
		return false
	}
	return orgInn == ownerInn
}

// CalculateRequest - единый запрос на расчет для всех провайдеров
type CalculateRequest struct {
	SessionID        string   `json:"session_id" binding:"required"`
	PeriodID         int      `json:"period_id" binding:"required"` // 1 = 12 месяцев, 2 = 6 месяцев, 3 = 20 дней (только EuroAsia, Apex, Trust)
	DriverRestriction bool    `json:"driver_restriction"`           // true = ограничено, false = неограничено
	Drivers          []Driver `json:"drivers,omitempty"`            // опционально, если driver_restriction = true
}

// Driver - данные водителя
type Driver struct {
	PassportSeries  string `json:"passport_series"`
	PassportNumber  string `json:"passport_number"`
	Birthdate       string `json:"birthdate"` // YYYY-MM-DD
	LicenseSeries   string `json:"license_series,omitempty"`
	LicenseNumber   string `json:"license_number,omitempty"`
	Relative        int    `json:"relative,omitempty"` // 0-10, по умолчанию 0
}

// CalculateResponse - ответ с расчетами от всех провайдеров
type CalculateResponse struct {
	Neo      interface{}       `json:"neo,omitempty"`
	Apex     interface{}       `json:"apex,omitempty"`
	Euroasia interface{}       `json:"euroasia,omitempty"`
	Gross    interface{}       `json:"gross,omitempty"`
	Trust    interface{}       `json:"trust,omitempty"`
	Premiums map[string]int    `json:"premiums,omitempty"` // только суммы премий по провайдерам (UZS), удобно для обработки
	Errors   []string          `json:"errors,omitempty"`
}

// SessionData - структура данных из Redis session
type SessionData struct {
	Vehicle      interface{} `json:"vehicle"`
	Person       interface{} `json:"person"`
	Organization interface{} `json:"organization"`
	Owner        *bool       `json:"owner"`
}

// getSessionData - получение данных из Redis по session_id
func (oc *OsagoAllController) getSessionData(sessionID string) (*SessionData, error) {
	rdb := utils.GetRedis()
	if rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}

	ctx := context.Background()
	redisKey := "osago_all:session:" + sessionID

	val, err := rdb.Get(ctx, redisKey).Result()
	if err != nil {
		return nil, fmt.Errorf("session not found: %v", err)
	}

	var sessionData SessionData
	if err := json.Unmarshal([]byte(val), &sessionData); err != nil {
		return nil, fmt.Errorf("failed to parse session data: %v", err)
	}

	return &sessionData, nil
}

// extractNestedString - безопасное извлечение строки из вложенной структуры
func extractNestedString(data interface{}, path ...string) string {
	current := data
	for _, key := range path {
		if current == nil {
			return ""
		}
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[key]
		case []interface{}:
			// allow numeric indexing like "0"
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(v) {
				return ""
			}
			current = v[idx]
		default:
			return ""
		}
	}
	return asString(current)
}

// extractNestedInt - безопасное извлечение int из вложенной структуры
func extractNestedInt(data interface{}, path ...string) int {
	current := data
	for _, key := range path {
		if current == nil {
			return 0
		}
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[key]
		case []interface{}:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(v) {
				return 0
			}
			current = v[idx]
		default:
			return 0
		}
	}
	
	switch v := current.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return 0
}

// extractOwnerPinfl - извлечение PINFL владельца
func extractOwnerPinfl(session *SessionData) string {
	vehicleData := session.Vehicle
	if vehicleData == nil {
		return ""
	}

	ownerType := extractNestedString(vehicleData, "data", "owner", "type")
	if ownerType == "person" {
		pinfls := extractNestedString(vehicleData, "data", "owner", "person", "pinfls", "0")
		if pinfls == "" {
			// Попробовать через external_id
			pinfls = extractNestedString(vehicleData, "data", "owner", "person", "external_id")
		}
		return pinfls
	}
	return ""
}

// formatDateDDMMYYYY - конвертация даты из ISO8601 в DD.MM.YYYY
func formatDateDDMMYYYY(isoDate string) string {
	if isoDate == "" {
		return ""
	}
	// Убрать время если есть
	parts := strings.Split(isoDate, "T")
	if len(parts) > 0 {
		dateParts := strings.Split(parts[0], "-")
		if len(dateParts) == 3 {
			return fmt.Sprintf("%s.%s.%s", dateParts[2], dateParts[1], dateParts[0])
		}
	}
	return isoDate
}

// formatDateYYYYMMDD - конвертация даты из ISO8601 в YYYY-MM-DD
func formatDateYYYYMMDD(isoDate string) string {
	if isoDate == "" {
		return ""
	}
	parts := strings.Split(isoDate, "T")
	if len(parts) > 0 {
		return parts[0]
	}
	return isoDate
}

// euroasiaLookupPinflByPassport использует EuroAsia Find API чтобы получить PINFL по паспорту+дате рождения.
// Это нужно для провайдеров (например Trust/Apex), где в расчёте требуется PINFL, а в session юрлица PINFL отсутствует.
func (oc *OsagoAllController) euroasiaLookupPinflByPassport(birthdateYYYYMMDD, passportSeries, passportNumber string) (string, error) {
	reqBody := map[string]string{
		"birthdate":       strings.TrimSpace(birthdateYYYYMMDD),
		"passport_series": strings.TrimSpace(passportSeries),
		"passport_number": strings.TrimSpace(passportNumber),
	}

	body, status, err := oc.makeExternalRequest(http.MethodPost, "/api/v1/insurance/persons/find-by-birthdate", reqBody)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("EuroAsia persons find-by-birthdate failed: HTTP %d", status)
	}

	var resp interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse EuroAsia person response: %v", err)
	}

	pinfl := extractNestedString(resp, "data", "pinfls", "0")
	if pinfl == "" {
		pinfl = extractNestedString(resp, "data", "external_id")
	}
	return pinfl, nil
}

// extractPremiumFromResponse извлекает сумму премии (UZS) из сырого ответа провайдера. Возвращает -1 если не удалось.
func extractPremiumFromResponse(provider string, data interface{}) int {
	if data == nil {
		return -1
	}
	m, ok := data.(map[string]interface{})
	if !ok {
		return -1
	}
	switch provider {
	case "neo":
		if resp, _ := m["response"].(map[string]interface{}); resp != nil {
			return toInt(resp["amount_uzs"])
		}
	case "apex":
		// "insurance_premium": "384 000,00 UZS"
		if s, _ := m["insurance_premium"].(string); s != "" {
			s = strings.TrimSpace(s)
			if i := strings.Index(s, " UZS"); i >= 0 {
				s = s[:i]
			}
			s = strings.ReplaceAll(s, " ", "")
			s = strings.ReplaceAll(s, "\u00a0", "")
			if idx := strings.Index(s, ","); idx >= 0 {
				s = s[:idx]
			}
			return toInt(s)
		}
	case "euroasia":
		if dataObj, _ := m["data"].(map[string]interface{}); dataObj != nil {
			if prem, _ := dataObj["premium"].(map[string]interface{}); prem != nil {
				return toInt(prem["amount"])
			}
		}
	case "gross":
		if resp, _ := m["response"].(map[string]interface{}); resp != nil {
			return toInt(resp["amount_uzs"])
		}
	case "trust":
		return toInt(m["insurance_premium"])
	}
	return -1
}

// toInt конвертирует число из JSON (float64, int, string) в int
func toInt(v interface{}) int {
	if v == nil {
		return -1
	}
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	case string:
		s := strings.TrimSpace(strings.ReplaceAll(x, " ", ""))
		n, _ := strconv.Atoi(s)
		return n
	}
	return -1
}

// Calculate - единый метод для расчета OSAGO от всех провайдеров
func (oc *OsagoAllController) Calculate(c *gin.Context) {
	var req CalculateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	if req.PeriodID < 1 || req.PeriodID > 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period_id must be 1 (12 мес), 2 (6 мес) or 3 (20 дней)"})
		return
	}
	if req.DriverRestriction && len(req.Drivers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "для ограниченной страховки (driver_restriction: true) обязательно указать drivers (минимум 1 водитель)"})
		return
	}

	// Получаем данные из session
	sessionData, err := oc.getSessionData(req.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found", "details": err.Error()})
		return
	}

	// Проверяем наличие данных о машине
	if sessionData.Vehicle == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vehicle data not found in session"})
		return
	}

	response := CalculateResponse{
		Errors: []string{},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Neo Insurance
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := oc.calculateNeo(sessionData, &req)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			response.Errors = append(response.Errors, "neo: "+err.Error())
		} else {
			response.Neo = result
		}
	}()

	// Apex Insurance
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := oc.calculateApex(sessionData, &req)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			response.Errors = append(response.Errors, "apex: "+err.Error())
		} else {
			response.Apex = result
		}
	}()

	// EuroAsia Insurance
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := oc.calculateEuroAsia(sessionData, &req)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			response.Errors = append(response.Errors, "euroasia: "+err.Error())
		} else {
			response.Euroasia = result
		}
	}()

	// Gross Insurance
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := oc.calculateGross(sessionData, &req)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			response.Errors = append(response.Errors, "gross: "+err.Error())
		} else {
			response.Gross = result
		}
	}()

	// Trust Insurance
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := oc.calculateTrust(sessionData, &req)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			response.Errors = append(response.Errors, "trust: "+err.Error())
		} else {
			response.Trust = result
		}
	}()

	wg.Wait()

	// Собираем отдельный объект только с суммами премий (UZS) для удобной обработки
	response.Premiums = make(map[string]int)
	if a := extractPremiumFromResponse("neo", response.Neo); a >= 0 {
		response.Premiums["neo"] = a
	}
	if a := extractPremiumFromResponse("apex", response.Apex); a >= 0 {
		response.Premiums["apex"] = a
	}
	if a := extractPremiumFromResponse("euroasia", response.Euroasia); a >= 0 {
		response.Premiums["euroasia"] = a
	}
	if a := extractPremiumFromResponse("gross", response.Gross); a >= 0 {
		response.Premiums["gross"] = a
	}
	if a := extractPremiumFromResponse("trust", response.Trust); a >= 0 {
		response.Premiums["trust"] = a
	}

	// Сохраняем в сессию параметры и результаты calculate — Create возьмёт оттуда period_id, driver_restriction, drivers, amount_uzs
	if rdb := utils.GetRedis(); rdb != nil {
		ctx := context.Background()
		redisKey := "osago_all:session:" + req.SessionID
		val, err := rdb.Get(ctx, redisKey).Result()
		if err == nil {
			var sessionMap map[string]interface{}
			if json.Unmarshal([]byte(val), &sessionMap) == nil {
				sessionMap["calculate_snapshot"] = map[string]interface{}{
					"period_id":          req.PeriodID,
					"driver_restriction": req.DriverRestriction,
					"drivers":            req.Drivers,
					"premiums":           response.Premiums,
				}
				if b, err := json.Marshal(sessionMap); err == nil {
					rdb.Set(ctx, redisKey, b, 30*time.Minute)
				}
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

// calculateNeo - расчет для Neo Insurance
func (oc *OsagoAllController) calculateNeo(session *SessionData, req *CalculateRequest) (interface{}, error) {
	// Проверка: Neo не поддерживает юридические лица
	ownerType := extractNestedString(session.Vehicle, "data", "owner", "type")
	if ownerType == "organization" {
		return nil, fmt.Errorf("Neo Insurance не поддерживает юридические лица")
	}
	if req.PeriodID == 3 {
		return nil, fmt.Errorf("Neo Insurance не поддерживает период 20 дней")
	}

	vehicleData := session.Vehicle
	gosNumber := extractNestedString(vehicleData, "data", "license_plate")
	techSeries := extractNestedString(vehicleData, "data", "tech_passport", "series")
	techNumber := extractNestedString(vehicleData, "data", "tech_passport", "number")

	if gosNumber == "" || techSeries == "" || techNumber == "" {
		return nil, fmt.Errorf("недостаточно данных о машине")
	}

	// Конвертация period_id: 1→"1", 2→"2" (string!)
	periodIDStr := strconv.Itoa(req.PeriodID)
	
	// Конвертация driver_restriction: true→"4", false→"1" (string!)
	numberDriversID := "1"
	if req.DriverRestriction {
		numberDriversID = "4"
	}

	requestBody := map[string]interface{}{
		"gos_number":        gosNumber,
		"tech_sery":         techSeries,
		"tech_number":       techNumber,
		"period_id":         periodIDStr,
		"number_drivers_id": numberDriversID,
	}

	url := oc.cfg.NeoBaseURL + "/api/osago-neo/get-calc-osago"
	return oc.makeProviderRequest("POST", url, requestBody, oc.cfg.NeoLogin, oc.cfg.NeoPassword, "")
}

// calculateApex - расчет для Apex Insurance
func (oc *OsagoAllController) calculateApex(session *SessionData, req *CalculateRequest) (interface{}, error) {
	vehicleData := session.Vehicle
	
	// Owner
	ownerType := extractNestedString(vehicleData, "data", "owner", "type")
	var owner map[string]interface{}
	if ownerType == "person" {
		pinfl := extractNestedString(vehicleData, "data", "owner", "person", "pinfls", "0")
		if pinfl == "" {
			pinfl = extractNestedString(vehicleData, "data", "owner", "person", "external_id")
		}
		passportSeries := extractNestedString(vehicleData, "data", "owner", "person", "passport", "series")
		passportNumber := extractNestedString(vehicleData, "data", "owner", "person", "passport", "number")

		owner = map[string]interface{}{
			"person": map[string]interface{}{
				"passportData": map[string]interface{}{
					"pinfl":  pinfl,
					"seria":  passportSeries,
					"number": passportNumber,
				},
			},
		}
	} else {
		// Юрлицо: Apex в calc ждёт owner.person.passportData.
		// Берём представителя из drivers[0] (передайте хотя бы 1 водителя для юрлица).
		if len(req.Drivers) == 0 {
			return nil, fmt.Errorf("для Apex (юрлицо) укажите drivers[] (минимум 1 водитель) с паспортом и датой рождения")
		}
		rep := req.Drivers[0]
		repPinfl := findPinflByPassport(rep.PassportSeries, rep.PassportNumber, session)
		if repPinfl == "" && rep.Birthdate != "" {
			var err error
			repPinfl, err = oc.euroasiaLookupPinflByPassport(formatDateYYYYMMDD(rep.Birthdate), rep.PassportSeries, rep.PassportNumber)
			if err != nil {
				// как fallback используем нули (Apex допускает), но лучше иметь реальный PINFL
				repPinfl = "00000000000000"
			}
		}

		inn := extractNestedString(vehicleData, "data", "owner", "organization", "inn")
		owner = map[string]interface{}{
			"person": map[string]interface{}{
				"passportData": map[string]interface{}{
					"pinfl":  repPinfl,
					"seria":  rep.PassportSeries,
					"number": rep.PassportNumber,
				},
			},
			"organization": map[string]interface{}{
				"inn": inn,
			},
		}
	}

	// Vehicle
	techSeries := extractNestedString(vehicleData, "data", "tech_passport", "series")
	techNumber := extractNestedString(vehicleData, "data", "tech_passport", "number")
	gosNumber := extractNestedString(vehicleData, "data", "license_plate")

	// Use territory
	useTerritoryID := extractNestedString(vehicleData, "data", "use_territory_region", "external_id")
	if useTerritoryID == "" {
		useTerritoryID = "1" // по умолчанию
	}

	// Apex: contractTermConclusionId 1=год, 2=сезон; seasonalInsuranceId 7=1год, 1=6мес, 8=20дней
	var contractTermID, seasonalInsuranceID string
	switch req.PeriodID {
	case 1:
		contractTermID = "1"
		seasonalInsuranceID = "7" // 1 год
	case 2:
		contractTermID = "2"
		seasonalInsuranceID = "1" // 6 месяцев
	case 3:
		contractTermID = "2"
		seasonalInsuranceID = "8" // 20 дней
	default:
		contractTermID = "1"
		seasonalInsuranceID = "7"
	}

	// Drivers
	var drivers []map[string]interface{}
	if req.DriverRestriction {
		if len(req.Drivers) == 0 {
			// Использовать владельца (только для физлица)
			ownerPinfl := extractOwnerPinfl(session)
			ownerPassSeries := extractNestedString(vehicleData, "data", "owner", "person", "passport", "series")
			ownerPassNumber := extractNestedString(vehicleData, "data", "owner", "person", "passport", "number")
			drivers = []map[string]interface{}{
				{
					"passportData": map[string]interface{}{
						"pinfl":  ownerPinfl,
						"seria":  ownerPassSeries,
						"number": ownerPassNumber,
					},
				},
			}
		} else {
			for _, driver := range req.Drivers {
				// Найти PINFL из session
				pinfl := findPinflByPassport(driver.PassportSeries, driver.PassportNumber, session)
				if pinfl == "" && driver.Birthdate != "" {
					if p, err := oc.euroasiaLookupPinflByPassport(formatDateYYYYMMDD(driver.Birthdate), driver.PassportSeries, driver.PassportNumber); err == nil {
						pinfl = p
					}
				}
				if pinfl == "" {
					pinfl = "00000000000000"
				}
				drivers = append(drivers, map[string]interface{}{
					"passportData": map[string]interface{}{
						"pinfl":  pinfl,
						"seria":  driver.PassportSeries,
						"number": driver.PassportNumber,
					},
				})
			}
		}
	} else {
		// Неограничено - заглушка
		drivers = []map[string]interface{}{
			{
				"passportData": map[string]interface{}{
					"pinfl":  "00000000000000",
					"seria":  "AA",
					"number": "0000000",
				},
			},
		}
	}

	requestBody := map[string]interface{}{
		"owner": owner,
		"details": map[string]interface{}{
			"driverNumberRestriction": req.DriverRestriction,
		},
		"cost": map[string]interface{}{
			"contractTermConclusionId": contractTermID,
			"useTerritoryId":           useTerritoryID,
			"seasonalInsuranceId":      seasonalInsuranceID,
			"foreignVehicleId":         "2",
		},
		"vehicle": map[string]interface{}{
			"techPassport": map[string]interface{}{
				"number": techNumber,
				"seria":  techSeries,
			},
			"govNumber": gosNumber,
		},
		"drivers": drivers,
	}

	url := oc.cfg.ApexBaseURL + "/osago_calculation"
	return oc.makeProviderRequest("POST", url, requestBody, oc.cfg.ApexLogin, oc.cfg.ApexPassword, "")
}

// calculateEuroAsia - расчет для EuroAsia Insurance
func (oc *OsagoAllController) calculateEuroAsia(session *SessionData, req *CalculateRequest) (interface{}, error) {
	vehicleData := session.Vehicle
	
	// Проверка UUID
	useTerritoryID := extractNestedString(vehicleData, "data", "use_territory_region", "id")
	vehicleGroupID := extractNestedString(vehicleData, "data", "vehicle_type", "vehicle_group")
	
	if useTerritoryID == "" || vehicleGroupID == "" {
		return nil, fmt.Errorf("недостаточно UUID данных для EuroAsia")
	}

	// Period UUID: 1→365 дней, 2→180 дней, 3→20 дней
	var seasonalInsuranceID string
	switch req.PeriodID {
	case 1:
		seasonalInsuranceID = "8465a831-850f-4445-a995-ef71195094ab" // 365 дней
	case 2:
		seasonalInsuranceID = "9848096e-cc12-4dbd-893b-41f2cdfc9a0e" // 180 дней
	case 3:
		seasonalInsuranceID = "0d546748-0ba6-43bc-9ce2-1b977ad9e494" // 20 дней
	default:
		seasonalInsuranceID = "8465a831-850f-4445-a995-ef71195094ab"
	}

	// Drivers
	var drivers []map[string]interface{}
	if req.DriverRestriction {
		if len(req.Drivers) == 0 {
			// Использовать владельца
			ownerBirthdate := extractNestedString(vehicleData, "data", "owner", "person", "birthdate")
			ownerPassSeries := extractNestedString(vehicleData, "data", "owner", "person", "passport", "series")
			ownerPassNumber := extractNestedString(vehicleData, "data", "owner", "person", "passport", "number")
			drivers = []map[string]interface{}{
				{
					"passport_birthdate": formatDateYYYYMMDD(ownerBirthdate),
					"passport_number":    ownerPassNumber,
					"passport_series":   ownerPassSeries,
				},
			}
		} else {
			for _, driver := range req.Drivers {
				drivers = append(drivers, map[string]interface{}{
					"passport_birthdate": formatDateYYYYMMDD(driver.Birthdate),
					"passport_number":    driver.PassportNumber,
					"passport_series":    driver.PassportSeries,
				})
			}
		}
	}

	requestBody := map[string]interface{}{
		"driver_restriction":      req.DriverRestriction,
		"drivers":                 drivers,
		"seasonal_insurance_id":   seasonalInsuranceID,
		"use_territory_region_id": useTerritoryID,
		"vehicle_group_id":        vehicleGroupID,
	}

	url := oc.cfg.EuroasiaAllBaseURL + "/api/v1/insurance/osago/calculate"
	return oc.makeProviderRequest("POST", url, requestBody, "", "", oc.cfg.EuroasiaAllAPIKey)
}

// calculateGross - расчет для Gross Insurance
func (oc *OsagoAllController) calculateGross(session *SessionData, req *CalculateRequest) (interface{}, error) {
	if req.PeriodID == 3 {
		return nil, fmt.Errorf("Gross Insurance не поддерживает период 20 дней")
	}
	vehicleData := session.Vehicle
	
	gosNumber := extractNestedString(vehicleData, "data", "license_plate")
	techSeries := extractNestedString(vehicleData, "data", "tech_passport", "series")
	techNumber := extractNestedString(vehicleData, "data", "tech_passport", "number")

	if gosNumber == "" || techSeries == "" || techNumber == "" {
		return nil, fmt.Errorf("недостаточно данных о машине")
	}

	// Vehicle type mapping: external_id=2 (легковые) → 1
	vehicleTypeExternalID := extractNestedInt(vehicleData, "data", "vehicle_type", "external_id")
	vehicleTypeID := 1 // по умолчанию легковые
	if vehicleTypeExternalID == 6 {
		vehicleTypeID = 2 // грузовые
	} else if vehicleTypeExternalID == 9 {
		vehicleTypeID = 3 // автобусы
	} else if vehicleTypeExternalID == 15 {
		vehicleTypeID = 4 // мотоциклы
	}

	// Driver restriction: true→4, false→1
	numberDriversID := 1
	if req.DriverRestriction {
		numberDriversID = 4
	}

	requestBody := map[string]interface{}{
		"gos_number":        gosNumber,
		"tech_sery":         techSeries,
		"tech_number":       techNumber,
		"period_id":         req.PeriodID,
		"number_drivers_id": numberDriversID,
		"vehicleTypeId":     vehicleTypeID,
	}

	url := oc.cfg.GrossBaseURL + "/osago-gross/get-calc-osago"
	return oc.makeProviderRequest("POST", url, requestBody, oc.cfg.GrossLogin, oc.cfg.GrossPassword, "")
}

// calculateTrust - расчет для Trust Insurance
func (oc *OsagoAllController) calculateTrust(session *SessionData, req *CalculateRequest) (interface{}, error) {
	vehicleData := session.Vehicle
	ownerType := extractNestedString(vehicleData, "data", "owner", "type")
	
	gosNumber := extractNestedString(vehicleData, "data", "license_plate")
	
	// Vehicle type mapping: external_id=2 → 1, 6→6, 9→9, 15→15
	vehicleTypeExternalID := extractNestedInt(vehicleData, "data", "vehicle_type", "external_id")
	vehicleTypeID := 1 // по умолчанию
	if vehicleTypeExternalID == 6 {
		vehicleTypeID = 6
	} else if vehicleTypeExternalID == 9 {
		vehicleTypeID = 9
	} else if vehicleTypeExternalID == 15 {
		vehicleTypeID = 15
	}

	// Use territory: external_id → Trust ID (1-14)
	useTerritoryExternalID := extractNestedInt(vehicleData, "data", "use_territory_region", "external_id")
	useTerritoryID := 1 // по умолчанию
	if useTerritoryExternalID > 0 && useTerritoryExternalID <= 14 {
		useTerritoryID = useTerritoryExternalID
	}

	// Trust: 1=6мес, 2=12мес, 3=2мес, 4=15 и 20 дней
	var periodID int
	switch req.PeriodID {
	case 1:
		periodID = 2 // 12 месяцев
	case 2:
		periodID = 1 // 6 месяцев
	case 3:
		periodID = 4 // 15 и 20 дней
	default:
		periodID = 2
	}

	// Driver limit: true→1, false→0
	driverLimit := 0
	if req.DriverRestriction {
		driverLimit = 1
	}

	// Owner data
	var ownerPinfl string
	var ownerInn string
	if ownerType == "person" {
		ownerPinfl = extractOwnerPinfl(session)
		if ownerPinfl == "" {
			return nil, fmt.Errorf("не найден PINFL владельца")
		}
	} else {
		ownerInn = extractNestedString(vehicleData, "data", "owner", "organization", "inn")
		if ownerInn == "" {
			ownerInn = extractNestedString(session.Organization, "data", "inn")
		}
		// Для Trust (юрлицо) нужен хотя бы один водитель, чтобы получить PINFL и подставить его в расчёт.
		if len(req.Drivers) == 0 {
			return nil, fmt.Errorf("для Trust (юрлицо) укажите drivers[] (минимум 1 водитель) с паспортом и датой рождения")
		}
		// owner_pinfl используем как PINFL представителя (первого водителя)
		first := req.Drivers[0]
		ownerPinfl = findPinflByPassport(first.PassportSeries, first.PassportNumber, session)
		if ownerPinfl == "" && first.Birthdate != "" {
			if p, err := oc.euroasiaLookupPinflByPassport(formatDateYYYYMMDD(first.Birthdate), first.PassportSeries, first.PassportNumber); err == nil {
				ownerPinfl = p
			}
		}
		if ownerPinfl == "" {
			return nil, fmt.Errorf("не удалось получить PINFL представителя (drivers[0]) для Trust")
		}
	}

	// Drivers
	var drivers []map[string]interface{}
	if req.DriverRestriction {
		if len(req.Drivers) == 0 {
			drivers = []map[string]interface{}{
				{
					"pinfl":      ownerPinfl,
					"coefficient": 1,
				},
			}
		} else {
			for _, driver := range req.Drivers {
				pinfl := findPinflByPassport(driver.PassportSeries, driver.PassportNumber, session)
				if pinfl == "" && driver.Birthdate != "" {
					if p, err := oc.euroasiaLookupPinflByPassport(formatDateYYYYMMDD(driver.Birthdate), driver.PassportSeries, driver.PassportNumber); err == nil {
						pinfl = p
					}
				}
				if pinfl == "" {
					pinfl = ownerPinfl // fallback на представителя
				}
				drivers = append(drivers, map[string]interface{}{
					"pinfl":       pinfl,
					"coefficient": 1,
				})
			}
		}
	} else {
		// Неограничено - все равно нужен массив с владельцем
		drivers = []map[string]interface{}{
			{
				"pinfl":       ownerPinfl,
				"coefficient": 1,
			},
		}
	}

	requestBody := map[string]interface{}{
		"vehicle": map[string]interface{}{
			"vehicle":       vehicleTypeID,
			"renumber":      gosNumber,
			"foreignVehicle": false,
		},
		"period":       periodID,
		"use_territory": useTerritoryID,
		"driver_limit": driverLimit,
		"discount":     1, // по умолчанию без льгот
		"owner": map[string]interface{}{
			"owner_pinfl": ownerPinfl,
		},
		"drivers": drivers,
	}
	if ownerType != "person" && ownerInn != "" {
		// пробуем передать ИНН юрлица (Trust может использовать для юрлиц)
		if o, ok := requestBody["owner"].(map[string]interface{}); ok {
			o["owner_inn"] = ownerInn
			o["owner_fy"] = 1
		}
	}

	url := oc.cfg.TrustBaseURL + "/api/osgo/v2/calc-prem"
	return oc.makeProviderRequest("POST", url, requestBody, oc.cfg.TrustLogin, oc.cfg.TrustPassword, "")
}

// findPinflByPassport - поиск PINFL по паспортным данным
func findPinflByPassport(series, number string, session *SessionData) string {
	// Проверить session.person
	if session.Person != nil {
		personSeries := extractNestedString(session.Person, "data", "passport", "series")
		personNumber := extractNestedString(session.Person, "data", "passport", "number")
		if personSeries == series && personNumber == number {
			pinfl := extractNestedString(session.Person, "data", "pinfls", "0")
			if pinfl == "" {
				pinfl = extractNestedString(session.Person, "data", "external_id")
			}
			return pinfl
		}
	}

	// Проверить vehicle.owner.person
	if session.Vehicle != nil {
		ownerType := extractNestedString(session.Vehicle, "data", "owner", "type")
		if ownerType == "person" {
			ownerSeries := extractNestedString(session.Vehicle, "data", "owner", "person", "passport", "series")
			ownerNumber := extractNestedString(session.Vehicle, "data", "owner", "person", "passport", "number")
			if ownerSeries == series && ownerNumber == number {
				pinfl := extractNestedString(session.Vehicle, "data", "owner", "person", "pinfls", "0")
				if pinfl == "" {
					pinfl = extractNestedString(session.Vehicle, "data", "owner", "person", "external_id")
				}
				return pinfl
			}
		}
	}

	return ""
}

// makeProviderRequest - универсальный метод для запросов к провайдерам
func (oc *OsagoAllController) makeProviderRequest(method, url string, bodyData interface{}, login, password, apiKey string) (interface{}, error) {
	var body io.Reader
	if bodyData != nil {
		jsonData, err := json.Marshal(bodyData)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Авторизация
	if apiKey != "" {
		req.Header.Set("Authorization", apiKey)
	} else if login != "" && password != "" {
		auth := base64.StdEncoding.EncodeToString([]byte(login + ":" + password))
		req.Header.Set("Authorization", "Basic "+auth)
	}

	resp, err := oc.cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		// HTML или не-JSON ответ — не возвращаем сырой HTML в теле
		if len(respBody) > 0 && (respBody[0] == '<' || !json.Valid(respBody)) {
			return nil, fmt.Errorf("ожидался JSON, получен не-JSON ответ (возможно HTML)")
		}
		return string(respBody), nil
	}

	return result, nil
}

