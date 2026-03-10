package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
	SessionID    string      `json:"session_id,omitempty"`
	Owner        *bool       `json:"owner,omitempty"`
	Vehicle      interface{} `json:"vehicle,omitempty"`
	Person       interface{} `json:"person,omitempty"`
	Organization interface{} `json:"organization,omitempty"`
	Errors       []string    `json:"errors,omitempty"`
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
			"vehicle":      response.Vehicle,
			"person":       response.Person,
			"organization": response.Organization,
			"owner":        response.Owner,
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
	SessionID         string   `json:"session_id" binding:"required"`
	PeriodID          int      `json:"period_id" binding:"required"` // 1 = 12 месяцев, 2 = 6 месяцев, 3 = 20 дней (только EuroAsia, Apex, Trust)
	DriverRestriction bool     `json:"driver_restriction"`           // true = ограничено, false = неограничено
	Drivers           []Driver `json:"drivers,omitempty"`            // опционально, если driver_restriction = true
}

// Driver - данные водителя
type Driver struct {
	PassportSeries string `json:"passport_series"`
	PassportNumber string `json:"passport_number"`
	Birthdate      string `json:"birthdate"` // YYYY-MM-DD
	LicenseSeries  string `json:"license_series,omitempty"`
	LicenseNumber  string `json:"license_number,omitempty"`
	Relative       int    `json:"relative,omitempty"` // 0-10, по умолчанию 0
}

// CalculateResponse - ответ с расчетами от всех провайдеров
type CalculateResponse struct {
	Neo      interface{}    `json:"neo,omitempty"`
	Apex     interface{}    `json:"apex,omitempty"`
	Euroasia interface{}    `json:"euroasia,omitempty"`
	Gross    interface{}    `json:"gross,omitempty"`
	Trust    interface{}    `json:"trust,omitempty"`
	Inson    interface{}    `json:"inson,omitempty"`
	Premiums map[string]int `json:"premiums,omitempty"` // только суммы премий по провайдерам (UZS), удобно для обработки
	Errors   []string       `json:"errors,omitempty"`
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

// extractNestedValue - безопасное извлечение interface{} из вложенной структуры
func extractNestedValue(data interface{}, path ...string) interface{} {
	current := data
	for _, key := range path {
		if current == nil {
			return nil
		}
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[key]
		case []interface{}:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil
			}
			current = v[idx]
		default:
			return nil
		}
	}
	return current
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

// extractActiveDocumentFromPerson — возвращает серию и номер активного документа владельца из documents[]:
// приоритет IDMS_RECV_MVD_IDCARD_CITIZEN (биометрический ID), затем IDMS_RECV_CITIZ_DOCUMENTS,
// затем любой другой документ; если ничего нет — из поля passport.
// Именно этот документ ожидают Apex и Neo (не поле passport, которое содержит старый паспорт).
func extractActiveDocumentFromPerson(personData interface{}) (series, number string) {
	pm, ok := personData.(map[string]interface{})
	if !ok {
		return "", ""
	}
	docs, _ := pm["documents"].([]interface{})
	// 1. Биометрический ID (приоритет)
	for _, d := range docs {
		doc, _ := d.(map[string]interface{})
		if doc == nil {
			continue
		}
		dt := asString(doc["document_type"])
		if dt != "IDMS_RECV_MVD_IDCARD_CITIZEN" {
			continue
		}
		s := asString(doc["series"])
		n := asString(doc["number"])
		if s != "" && n != "" {
			return s, n
		}
	}
	// 2. Гражданский паспорт (книжка)
	for _, d := range docs {
		doc, _ := d.(map[string]interface{})
		if doc == nil {
			continue
		}
		dt := asString(doc["document_type"])
		if dt != "IDMS_RECV_CITIZ_DOCUMENTS" {
			continue
		}
		s := asString(doc["series"])
		n := asString(doc["number"])
		if s != "" && n != "" {
			return s, n
		}
	}
	// 3. Любой документ из массива
	for _, d := range docs {
		doc, _ := d.(map[string]interface{})
		if doc == nil {
			continue
		}
		s := asString(doc["series"])
		n := asString(doc["number"])
		if s != "" && n != "" {
			return s, n
		}
	}
	// 4. Fallback: поле passport
	if passport, ok := pm["passport"].(map[string]interface{}); ok {
		s := asString(passport["series"])
		n := asString(passport["number"])
		if s != "" && n != "" {
			return s, n
		}
	}
	return "", ""
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
	case "inson":
		// response: {"data": {"insurancePremium": 192000, ...}}
		if dataObj, _ := m["data"].(map[string]interface{}); dataObj != nil {
			return toInt(dataObj["insurancePremium"])
		}
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

	ownerType := extractNestedString(sessionData.Vehicle, "data", "owner", "type")
	isLegalEntity := ownerType == "organization"
	// Для юрлица OSAGO только через Trust; Trust не поддерживает период 20 дней
	if isLegalEntity && req.PeriodID == 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "для юридического лица доступен только Trust, период 20 дней не поддерживается"})
		return
	}

	response := CalculateResponse{
		Errors: []string{},
	}

	// Ожидаемые провайдеры: при period_id=3 Trust и Inson не вызываем (только 6 и 12 мес)
	expectedProviders := []string{"neo", "apex", "euroasia", "gross"}
	if req.PeriodID != 3 {
		expectedProviders = append(expectedProviders, "trust", "inson")
	}

	var mu sync.Mutex
	hasResponse := func(name string) bool {
		mu.Lock()
		defer mu.Unlock()
		switch name {
		case "neo":
			return response.Neo != nil
		case "apex":
			return response.Apex != nil
		case "euroasia":
			return response.Euroasia != nil
		case "gross":
			return response.Gross != nil
		case "trust":
			return response.Trust != nil
		case "inson":
			return response.Inson != nil
		}
		return false
	}

	// Вызов одного провайдера с одной повторной попыткой при отсутствии ответа
	callOne := func(providerName string, calculateFunc func() (interface{}, error)) {
		result, err := calculateFunc()
		if result == nil || err != nil {
			result, err = calculateFunc()
		}
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			response.Errors = append(response.Errors, providerName+": "+err.Error())
		} else if result != nil {
			switch providerName {
			case "neo":
				response.Neo = result
			case "apex":
				response.Apex = result
			case "euroasia":
				response.Euroasia = result
			case "gross":
				response.Gross = result
			case "trust":
				response.Trust = result
			case "inson":
				response.Inson = result
			}
		}
	}

	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var wg sync.WaitGroup
		for _, name := range expectedProviders {
			if hasResponse(name) {
				continue
			}
			wg.Add(1)
			go func(providerName string) {
				defer wg.Done()
				switch providerName {
				case "neo":
					callOne("neo", func() (interface{}, error) { return oc.calculateNeo(sessionData, &req) })
				case "apex":
					callOne("apex", func() (interface{}, error) { return oc.calculateApex(sessionData, &req) })
				case "euroasia":
					callOne("euroasia", func() (interface{}, error) { return oc.calculateEuroAsia(sessionData, &req) })
				case "gross":
					callOne("gross", func() (interface{}, error) { return oc.calculateGross(sessionData, &req) })
				case "trust":
					callOne("trust", func() (interface{}, error) { return oc.calculateTrust(sessionData, &req) })
				case "inson":
					callOne("inson", func() (interface{}, error) { return oc.calculateInson(sessionData, &req) })
				}
			}(name)
		}
		wg.Wait()

		allOk := true
		for _, name := range expectedProviders {
			if !hasResponse(name) {
				allOk = false
				break
			}
		}
		// Для юрлица обязательно нужен Trust
		if isLegalEntity && response.Trust == nil {
			allOk = false
		}
		if allOk {
			break
		}
	}

	// Для юрлица без ответа Trust возвращаем ошибку
	if isLegalEntity && response.Trust == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":  "для юридического лица необходим ответ от Trust; ответ не получен после нескольких попыток",
			"errors": response.Errors,
		})
		return
	}

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
	if a := extractPremiumFromResponse("inson", response.Inson); a >= 0 {
		response.Premiums["inson"] = a
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

// calculateNeo - расчет для Neo Insurance.
func (oc *OsagoAllController) calculateNeo(session *SessionData, req *CalculateRequest) (interface{}, error) {
	ownerType := extractNestedString(session.Vehicle, "data", "owner", "type")
	if ownerType == "organization" {
		return nil, fmt.Errorf("Neo Insurance не поддерживает юридические лица")
	}
	if req.PeriodID == 3 {
		return nil, fmt.Errorf("Neo Insurance не поддерживает период 20 дней")
	}

	vehicleData := session.Vehicle
	gosNumber  := extractNestedString(vehicleData, "data", "license_plate")
	techSeries := extractNestedString(vehicleData, "data", "tech_passport", "series")
	techNumber := extractNestedString(vehicleData, "data", "tech_passport", "number")

	// Логируем что именно извлекли из сессии
	log.Printf("[Neo calc] extracted from session: gos_number=%q tech_sery=%q tech_number=%q", gosNumber, techSeries, techNumber)

	if gosNumber == "" || techSeries == "" || techNumber == "" {
		return nil, fmt.Errorf("недостаточно данных о машине")
	}

	periodIDStr := strconv.Itoa(req.PeriodID)
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

	reqJSON, _ := json.Marshal(requestBody)
	log.Printf("[Neo calc] → sending: %s", reqJSON)

	url := oc.cfg.NeoBaseURL + "/api/osago-neo/get-calc-osago"
	result, err := oc.makeProviderRequest("POST", url, requestBody, oc.cfg.NeoLogin, oc.cfg.NeoPassword, "")

	respJSON, _ := json.Marshal(result)
	if err != nil {
		log.Printf("[Neo calc] ← error: %v", err)
	} else {
		log.Printf("[Neo calc] ← response: %s", respJSON)
	}
	return result, err
}

// calculateApex - расчет для Apex Insurance.
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
		ownerPersonObj := extractNestedValue(vehicleData, "data", "owner", "person")
		// Используем активный документ из documents[] (ID-карта), а не устаревшее поле passport
		passportSeries, passportNumber := extractActiveDocumentFromPerson(ownerPersonObj)
		if passportSeries == "" || passportNumber == "" {
			// fallback: поле passport
			passportSeries = extractNestedString(vehicleData, "data", "owner", "person", "passport", "series")
			passportNumber = extractNestedString(vehicleData, "data", "owner", "person", "passport", "number")
		}

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
			repPinfl, _ = oc.euroasiaLookupPinflByPassport(formatDateYYYYMMDD(rep.Birthdate), rep.PassportSeries, rep.PassportNumber)
		}
		if repPinfl == "" {
			return nil, fmt.Errorf("для Apex (юрлицо) не удалось получить PINFL представителя (drivers[0]); укажите паспорт и дату рождения и выполните find по человеку")
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

	// Территория использования — только из сессии Find (без дефолтов)
	useTerritoryID := extractNestedString(vehicleData, "data", "use_territory_region", "external_id")
	if useTerritoryID == "" {
		useTerritoryID = extractNestedString(vehicleData, "data", "use_territory_region", "id")
	}
	if useTerritoryID == "" {
		return nil, fmt.Errorf("в сессии нет территории использования (use_territory_region); выполните find с полными данными по ТС")
	}
	// Логируем что именно извлекли из сессии
	{
		_s, _n := extractActiveDocumentFromPerson(extractNestedValue(vehicleData, "data", "owner", "person"))
		_pf := extractNestedString(vehicleData, "data", "owner", "person", "pinfls", "0")
		if _pf == "" {
			_pf = extractNestedString(vehicleData, "data", "owner", "person", "external_id")
		}
		log.Printf("[Apex calc] session: gos=%q tech_seria=%q tech_number=%q territory=%q pinfl=%q doc_seria=%q doc_number=%q",
			gosNumber, techSeries, techNumber, useTerritoryID, _pf, _s, _n)
	}

	// Период — коды Apex из конфига (period_id от пользователя)
	var contractTermID, seasonalInsuranceID string
	switch req.PeriodID {
	case 1:
		contractTermID = oc.cfg.ApexContractTerm12
		seasonalInsuranceID = oc.cfg.ApexSeasonalID12
	case 2:
		contractTermID = oc.cfg.ApexContractTerm6
		seasonalInsuranceID = oc.cfg.ApexSeasonalID6
	case 3:
		contractTermID = oc.cfg.ApexContractTerm6
		seasonalInsuranceID = oc.cfg.ApexSeasonalID20
	default:
		contractTermID = oc.cfg.ApexContractTerm12
		seasonalInsuranceID = oc.cfg.ApexSeasonalID12
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
					return nil, fmt.Errorf("не удалось получить PINFL водителя (серия %s, номер %s); выполните find по человеку или укажите дату рождения", driver.PassportSeries, driver.PassportNumber)
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
		// Неограничено водителей: Apex требует непустой массив; передаём владельца как единственного водителя
		ownerPersonObj := extractNestedValue(vehicleData, "data", "owner", "person")
		ownerDocSery, ownerDocNum := extractActiveDocumentFromPerson(ownerPersonObj)
		if ownerDocSery == "" {
			ownerDocSery = extractNestedString(vehicleData, "data", "owner", "person", "passport", "series")
		}
		if ownerDocNum == "" {
			ownerDocNum = extractNestedString(vehicleData, "data", "owner", "person", "passport", "number")
		}
		ownerPinflFull := extractNestedString(vehicleData, "data", "owner", "person", "pinfls", "0")
		if ownerPinflFull == "" {
			ownerPinflFull = extractNestedString(vehicleData, "data", "owner", "person", "external_id")
		}
		if ownerDocSery == "" || ownerDocNum == "" || ownerPinflFull == "" {
			return nil, fmt.Errorf("для Apex (неограниченный полис) в сессии должны быть данные владельца: паспорт и PINFL; выполните find по ТС и владельцу")
		}
		drivers = []map[string]interface{}{
			{
				"passportData": map[string]interface{}{
					"pinfl":  ownerPinflFull,
					"seria":  ownerDocSery,
					"number": ownerDocNum,
				},
			},
		}
	}

	// Apex calc: согласно документации — НЕ передавать typeId/vehicleTypeId в calc (только в create)
	requestBody := map[string]interface{}{
		"owner": owner,
		"details": map[string]interface{}{
			"driverNumberRestriction": req.DriverRestriction,
		},
		"cost": map[string]interface{}{
			"contractTermConclusionId": contractTermID,
			"useTerritoryId":           useTerritoryID,
			"seasonalInsuranceId":      seasonalInsuranceID,
			"foreignVehicleId":         oc.cfg.ApexForeignVehicleID,
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

	reqJSON, _ := json.Marshal(requestBody)
	log.Printf("[Apex calc] → sending: %s", reqJSON)

	url := oc.cfg.ApexBaseURL + "/osago_calculation"
	result, err := oc.makeProviderRequest("POST", url, requestBody, oc.cfg.ApexLogin, oc.cfg.ApexPassword, "")

	respJSON, _ := json.Marshal(result)
	if err != nil {
		log.Printf("[Apex calc] ← error: %v", err)
	} else {
		log.Printf("[Apex calc] ← response: %s", respJSON)
	}
	return result, err
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

	// Период — из конфига (UUID сезонности EuroAsia)
	var seasonalInsuranceID string
	switch req.PeriodID {
	case 1:
		seasonalInsuranceID = oc.cfg.EuroasiaSeasonalID12
	case 2:
		seasonalInsuranceID = oc.cfg.EuroasiaSeasonalID6
	case 3:
		seasonalInsuranceID = oc.cfg.EuroasiaSeasonalID20
	default:
		seasonalInsuranceID = oc.cfg.EuroasiaSeasonalID12
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
					"passport_series":    ownerPassSeries,
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

	// Тип ТС — только из сессии Find (маппинг external_id → Gross: 2→1, 6→2, 9→3, 15→4)
	vehicleTypeExternalID := extractNestedInt(vehicleData, "data", "vehicle_type", "external_id")
	if vehicleTypeExternalID == 0 {
		vehicleTypeExternalID = extractNestedInt(vehicleData, "data", "vehicle_type", "id")
	}
	if vehicleTypeExternalID == 0 {
		return nil, fmt.Errorf("в сессии нет типа ТС (vehicle_type); выполните find с полными данными по ТС")
	}
	var vehicleTypeID int
	switch vehicleTypeExternalID {
	case 2:
		vehicleTypeID = 1 // легковые
	case 6:
		vehicleTypeID = 2 // грузовые
	case 9:
		vehicleTypeID = 3 // автобусы
	case 15:
		vehicleTypeID = 4 // мотоциклы
	default:
		return nil, fmt.Errorf("тип ТС из сессии (vehicle_type.external_id=%d) не поддерживается Gross; ожидается 2, 6, 9 или 15", vehicleTypeExternalID)
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

	// Тип ТС — только из сессии Find (маппинг external_id → Trust: 2→1, 6→6, 9→9, 15→15)
	vehicleTypeExternalID := extractNestedInt(vehicleData, "data", "vehicle_type", "external_id")
	if vehicleTypeExternalID == 0 {
		vehicleTypeExternalID = extractNestedInt(vehicleData, "data", "vehicle_type", "id")
	}
	if vehicleTypeExternalID == 0 {
		return nil, fmt.Errorf("в сессии нет типа ТС (vehicle_type); выполните find с полными данными по ТС")
	}
	vehicleTypeID := vehicleTypeExternalID
	if vehicleTypeExternalID == 2 {
		vehicleTypeID = 1 // легковые в Find = external_id 2, в Trust = 1
	}

	// Территория: Find возвращает 1–3 категории (1=Ташкент и обл., 2=Другие регионы, 3=Для иностранцев),
	// Trust ожидает 1–14 регионов (1=город Ташкент, 2=Ташкентская обл., 3–14=остальные). Маппинг через конфиг.
	useTerritoryExternalID := extractNestedInt(vehicleData, "data", "use_territory_region", "external_id")
	if useTerritoryExternalID == 0 {
		useTerritoryExternalID = extractNestedInt(vehicleData, "data", "use_territory_region", "id")
	}
	if useTerritoryExternalID <= 0 {
		return nil, fmt.Errorf("в сессии нет территории использования (use_territory_region); выполните find с полными данными по ТС")
	}
	var useTerritoryID int
	switch useTerritoryExternalID {
	case 1:
		useTerritoryID = oc.cfg.TrustTerritoryFind1 // Ташкент и обл. → по умолчанию 1 (город Ташкент)
	case 2:
		useTerritoryID = oc.cfg.TrustTerritoryFind2 // Другие регионы → по умолчанию 10 (Самаркандская)
	case 3:
		useTerritoryID = oc.cfg.TrustTerritoryFind3 // Для иностранцев
	default:
		if useTerritoryExternalID >= 4 && useTerritoryExternalID <= 14 {
			useTerritoryID = useTerritoryExternalID
		} else {
			return nil, fmt.Errorf("территория из сессии (use_territory_region.external_id=%d) не поддерживается Trust; ожидается 1–3 (Find) или 4–14", useTerritoryExternalID)
		}
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
					"pinfl":       ownerPinfl,
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
		// Неограничено (driver_limit=0): пустой массив, как в Trust Create — иначе Trust возвращает завышенную премию
		drivers = []map[string]interface{}{}
	}

	requestBody := map[string]interface{}{
		"vehicle": map[string]interface{}{
			"vehicle":        vehicleTypeID,
			"renumber":       gosNumber,
			"foreignVehicle": false,
		},
		"period":        periodID,
		"use_territory": useTerritoryID,
		"driver_limit":  driverLimit,
		"discount":      oc.cfg.TrustDefaultDiscountID,
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

	trustBodyRaw, _ := json.Marshal(requestBody)
	log.Printf("[Trust calc] → request body (raw JSON): %s", trustBodyRaw)

	url := oc.cfg.TrustBaseURL + "/api/osgo/v2/calc-prem"
	return oc.makeProviderRequest("POST", url, requestBody, oc.cfg.TrustLogin, oc.cfg.TrustPassword, "")
}

// calculateInson - расчет ОСАГО для Inson Insurance.
// Inson принимает vehicleTypeId, period (6/12), governmentNumber, drivers (массив PINFL) и owner (для физлиц).
// Auth: HTTP Basic.
func (oc *OsagoAllController) calculateInson(session *SessionData, req *CalculateRequest) (interface{}, error) {
	if req.PeriodID == 3 {
		return nil, fmt.Errorf("Inson не поддерживает период 20 дней")
	}

	vehicleData := session.Vehicle
	gosNumber := extractNestedString(vehicleData, "data", "license_plate")
	if gosNumber == "" {
		return nil, fmt.Errorf("не найден госномер автомобиля")
	}

	// period: 1→12 мес, 2→6 мес (в месяцах для API)
	period := 12
	if req.PeriodID == 2 {
		period = 6
	}

	// Тип ТС — только из сессии Find (Inson: 1=легковой, 6, 9, 15; в Find легковые часто external_id=2)
	vehicleTypeExternalID := extractNestedInt(vehicleData, "data", "vehicle_type", "external_id")
	if vehicleTypeExternalID == 0 {
		vehicleTypeExternalID = extractNestedInt(vehicleData, "data", "vehicle_type", "id")
	}
	if vehicleTypeExternalID == 0 {
		return nil, fmt.Errorf("в сессии нет типа ТС (vehicle_type); выполните find с полными данными по ТС")
	}
	vehicleTypeID := vehicleTypeExternalID
	if vehicleTypeExternalID == 2 {
		vehicleTypeID = 1 // легковые
	}
	if vehicleTypeID != 1 && vehicleTypeID != 6 && vehicleTypeID != 9 && vehicleTypeID != 15 {
		return nil, fmt.Errorf("тип ТС из сессии (vehicle_type=%d) не поддерживается Inson; ожидается 1, 6, 9 или 15", vehicleTypeID)
	}

	ownerType := extractNestedString(vehicleData, "data", "owner", "type")

	log.Printf("[Inson calc] extracted from session: gos_number=%q owner_type=%q period_id=%d period_sent=%d driver_restriction=%v", gosNumber, ownerType, req.PeriodID, period, req.DriverRestriction)

	// Список водителей (PINFLs). По документации Inson: при driverNumberRestricted=false массив drivers должен быть пустым [].
	var driverPinfls []string
	if req.DriverRestriction {
		for _, driver := range req.Drivers {
			pinfl := findPinflByPassport(driver.PassportSeries, driver.PassportNumber, session)
			if pinfl == "" && driver.Birthdate != "" {
				if p, err := oc.euroasiaLookupPinflByPassport(formatDateYYYYMMDD(driver.Birthdate), driver.PassportSeries, driver.PassportNumber); err == nil {
					pinfl = p
				}
			}
			if pinfl != "" {
				driverPinfls = append(driverPinfls, pinfl)
			}
		}
	}
	// Иначе driverPinfls остаётся nil → в JSON "null"; Inson принимает и [], и отсутствие. Явно передаём [] для соответствия доке.
	if driverPinfls == nil {
		driverPinfls = []string{}
	}

	requestBody := map[string]interface{}{
		"vehicleTypeId":          vehicleTypeID,
		"period":                 period,
		"governmentNumber":       gosNumber,
		"drivers":                driverPinfls,
		"driverNumberRestricted": req.DriverRestriction,
	}

	// owner: только для физлиц; для юрлиц не передаётся
	if ownerType == "person" {
		ownerPinfl := extractOwnerPinfl(session)
		ownerPersonObj := extractNestedValue(vehicleData, "data", "owner", "person")
		passportSeries, passportNumber := extractActiveDocumentFromPerson(ownerPersonObj)
		if passportSeries == "" {
			passportSeries = extractNestedString(vehicleData, "data", "owner", "person", "passport", "series")
			passportNumber = extractNestedString(vehicleData, "data", "owner", "person", "passport", "number")
		}
		if ownerPinfl != "" && passportSeries != "" && passportNumber != "" {
			requestBody["owner"] = map[string]interface{}{
				"pinfl":          ownerPinfl,
				"passportSeries": passportSeries,
				"passportNumber": passportNumber,
			}
		}
	}

	insonBodyRaw, _ := json.Marshal(requestBody)
	log.Printf("[Inson calc] → request body (raw JSON): %s", insonBodyRaw)

	url := oc.cfg.InsonBaseURL + "/api/v2/osago/calculator"
	result, err := oc.makeProviderRequest("POST", url, requestBody, oc.cfg.InsonLogin, oc.cfg.InsonPassword, "")

	if err != nil {
		log.Printf("[Inson calc] ← error: %v", err)
	} else {
		respJSON, _ := json.Marshal(result)
		log.Printf("[Inson calc] ← response: %s", respJSON)
	}
	return result, err
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
