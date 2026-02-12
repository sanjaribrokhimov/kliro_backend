package osagoCreate

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
	"time"

	"github.com/gin-gonic/gin"

	"kliro/config"
	"kliro/utils"
)

type OsagoCreateController struct {
	cfg *config.Config
	cl  *http.Client
}

func NewOsagoCreateController(cfg *config.Config) *OsagoCreateController {
	return &OsagoCreateController{
		cfg: cfg,
		cl: &http.Client{Timeout: 40 * time.Second},
	}
}

// CreateRequest - единый body для create у любого провайдера (neo|gross|euroasia|trust|apex).
// Один и тот же набор полей: backend сам маппит на API провайдера. Find/calculate не трогаем.
type CreateRequest struct {
	SessionID         string   `json:"session_id" binding:"required"`
	Provider          string   `json:"provider" binding:"required"` // neo|gross|euroasia|trust|apex
	PeriodID          int      `json:"period_id,omitempty"` // 1=12мес, 2=6мес, 3=20дней; для Neo можно 0 — возьмётся из сессии (после Calculate)
	DriverRestriction bool     `json:"driver_restriction"`
	Drivers           []Driver `json:"drivers,omitempty"`

	StartDate   string `json:"start_date,omitempty"`   // YYYY-MM-DD (обязательно для gross, euroasia, trust)
	PhoneNumber string `json:"phone_number,omitempty"` // обязательно для neo, gross, euroasia
	AmountUZS   int    `json:"amount_uzs,omitempty"`   // обязательно для neo, gross

	// Заявитель (для Neo и др.): если не передан — берётся владелец ТС из сессии
	ApplicantPassportSeries string `json:"applicant_passport_series,omitempty"`
	ApplicantPassportNumber string `json:"applicant_passport_number,omitempty"`
	ApplicantBirthdate      string `json:"applicant_birthdate,omitempty"` // YYYY-MM-DD
	ApplicantIsDriver       *bool  `json:"applicant_is_driver,omitempty"`  // для Neo: заявитель в списке водителей (передаётся в теле)

	// Только для provider=euroasia
	EuroasiaDistrictID     string `json:"euroasia_district_id,omitempty"`
	EuroasiaInsurantType   string `json:"euroasia_insurant_type,omitempty"`
	EuroasiaOwnerIsInsurant *bool `json:"euroasia_owner_is_insurant,omitempty"`

	// Опционально: если передан — подменяет собранный payload только для trust/apex (для обратной совместимости)
	ProviderPayload map[string]interface{} `json:"provider_payload,omitempty"`
}

type Driver struct {
	PassportSeries string `json:"passport_series"`
	PassportNumber string `json:"passport_number"`
	Birthdate      string `json:"birthdate"` // YYYY-MM-DD

	LicenseSeries    string `json:"license_series,omitempty"`
	LicenseNumber    string `json:"license_number,omitempty"`
	LicenseIssueDate string `json:"license_issue_date,omitempty"` // YYYY-MM-DD
	Relative         int    `json:"relative,omitempty"`           // 0-10
}

type CreateResponse struct {
	Neo      interface{} `json:"neo,omitempty"`
	Gross    interface{} `json:"gross,omitempty"`
	Euroasia interface{} `json:"euroasia,omitempty"`
	Trust    interface{} `json:"trust,omitempty"`
	Apex     interface{} `json:"apex,omitempty"`
	Errors   []string    `json:"errors,omitempty"`
}

// CalculateSnapshot — параметры и результаты calculate, сохраняются в сессию при вызове Calculate
type CalculateSnapshot struct {
	PeriodID          int               `json:"period_id"`
	DriverRestriction bool             `json:"driver_restriction"`
	Drivers           []Driver         `json:"drivers"`
	Premiums          map[string]int    `json:"premiums"` // neo, gross, ... -> amount_uzs
}

type SessionData struct {
	Vehicle           interface{}        `json:"vehicle"`
	Person            interface{}        `json:"person"`
	Organization      interface{}       `json:"organization"`
	Owner             *bool             `json:"owner"`
	CalculateSnapshot *CalculateSnapshot `json:"calculate_snapshot,omitempty"`
}

func (oc *OsagoCreateController) getSessionData(sessionID string) (*SessionData, error) {
	rdb := utils.GetRedis()
	if rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}
	ctx := context.Background()
	val, err := rdb.Get(ctx, "osago_all:session:"+sessionID).Result()
	if err != nil {
		return nil, fmt.Errorf("session not found: %v", err)
	}
	var s SessionData
	if err := json.Unmarshal([]byte(val), &s); err != nil {
		return nil, fmt.Errorf("failed to parse session data: %v", err)
	}
	return &s, nil
}

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
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(v) {
				return ""
			}
			current = v[idx]
		default:
			return ""
		}
	}
	switch x := current.(type) {
	case string:
		return x
	case float64:
		return strconv.Itoa(int(x))
	default:
		b, _ := json.Marshal(x)
		if string(b) == "null" {
			return ""
		}
		return strings.Trim(string(b), `"`)
	}
}

func extractNestedInt(data interface{}, path ...string) int {
	s := extractNestedString(data, path...)
	if s == "" {
		return 0
	}
	i, _ := strconv.Atoi(s)
	return i
}

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

// extractPersonPassportFromFindSession извлекает серию и номер паспорта из person из ответа Find:
// поддерживает и session.Person (data.passport / data.documents), и vehicle.data.owner.person (passport / documents без обёртки data).
func extractPersonPassportFromFindSession(person interface{}) (series, number string) {
	if person == nil {
		return "", ""
	}
	pm, ok := person.(map[string]interface{})
	if !ok {
		return "", ""
	}
	data, _ := pm["data"].(map[string]interface{})
	if data == nil {
		data = pm
	}
	if data == nil {
		return "", ""
	}
	// Сначала documents (ID card / гражданин) — как в ответе Find (neoFlow); затем passport
	docs, _ := data["documents"].([]interface{})
	for _, d := range docs {
		doc, _ := d.(map[string]interface{})
		if doc == nil {
			continue
		}
		dt := asString(doc["document_type"])
		if dt != "IDMS_RECV_MVD_IDCARD_CITIZEN" && dt != "IDMS_RECV_CITIZ_DOCUMENTS" {
			continue
		}
		s := asString(doc["series"])
		n := asString(doc["number"])
		if s != "" && n != "" {
			return s, n
		}
	}
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
	if passport, ok := data["passport"].(map[string]interface{}); ok {
		series = asString(passport["series"])
		number = asString(passport["number"])
		if series != "" && number != "" {
			return series, number
		}
	}
	return "", ""
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		return strconv.Itoa(int(x))
	default:
		return ""
	}
}

func formatDateDDMMYYYY(dateYYYYMMDD string) string {
	if dateYYYYMMDD == "" {
		return ""
	}
	// also accept ISO with time
	parts := strings.Split(dateYYYYMMDD, "T")
	d := parts[0]
	p := strings.Split(d, "-")
	if len(p) == 3 {
		return fmt.Sprintf("%s.%s.%s", p[2], p[1], p[0])
	}
	return dateYYYYMMDD
}

func relativeToEuroasiaUUID(rel int) string {
	// from euroAsiaFlow.txt (relatives list)
	switch rel {
	case 1:
		return "903da482-1fd9-4e90-a384-9e4a52b6545c" // Father
	case 2:
		return "df286690-0d72-4cce-95e0-f27c30624174" // Mother
	case 3:
		return "94531b36-f72d-43b6-9e21-a63b251e0858" // Husband
	case 4:
		return "07147dd2-1c8f-424a-86e1-f79a38a5465e" // Wife
	case 5:
		return "6f3cb0a3-463c-498f-a0ef-09543a7c36c8" // Son
	case 6:
		return "ce1ddb40-e938-40cb-8653-9881817ba5a7" // Daughter
	case 7:
		return "44da9d2a-dee9-49b8-ad49-66f8c51d5cc1" // Older Brother
	case 8:
		return "10b0ac96-1004-4c71-99a1-82b2ff10847d" // Younger Brother
	case 9:
		return "cee50656-1d4f-4b47-aa78-ffa259bf1776" // Older Sister
	case 10:
		return "3e29b1ea-e10e-45dd-a73c-2e77c6e62052" // Younger Sister
	default:
		return "ab3391d9-a5df-4b7d-ae85-79479e9ad10b" // Not relative
	}
}

func (oc *OsagoCreateController) euroasiaLookupPinflByPassport(birthdateYYYYMMDD, passportSeries, passportNumber string) (string, error) {
	url := oc.cfg.EuroasiaAllBaseURL + "/api/v1/insurance/persons/find-by-birthdate"
	reqBody := map[string]string{
		"birthdate":       strings.TrimSpace(birthdateYYYYMMDD),
		"passport_series": strings.TrimSpace(passportSeries),
		"passport_number": strings.TrimSpace(passportNumber),
	}
	resp, err := oc.makeProviderRequest("POST", url, reqBody, "", "", oc.cfg.EuroasiaAllAPIKey)
	if err != nil {
		return "", err
	}
	pinfl := extractNestedString(resp, "data", "pinfls", "0")
	if pinfl == "" {
		pinfl = extractNestedString(resp, "data", "external_id")
	}
	return pinfl, nil
}

// makeProviderRequest - универсальный запрос к провайдеру (Basic или Authorization token)
func (oc *OsagoCreateController) makeProviderRequest(method, url string, bodyData interface{}, login, password, apiKey string) (map[string]interface{}, error) {
	var body io.Reader
	if bodyData != nil {
		b, err := json.Marshal(bodyData)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(b)
		// Логируем фактический запрос create для отладки (URL + body)
		log.Printf("[OSAGO_CREATE] %s %s", method, url)
		bodyForLog, _ := json.MarshalIndent(bodyData, "", "  ")
		log.Printf("[OSAGO_CREATE] Request body:\n%s", string(bodyForLog))
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
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
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("expected JSON, got: %s", strings.TrimSpace(string(respBody)))
	}
	return out, nil
}

func (oc *OsagoCreateController) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body", "details": err.Error()})
		return
	}
	if req.PeriodID != 0 && (req.PeriodID < 1 || req.PeriodID > 3) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period_id must be 1 (12 мес), 2 (6 мес) or 3 (20 дней), или 0 — тогда возьмётся из сессии (после Calculate)"})
		return
	}
	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))

	session, err := oc.getSessionData(req.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found", "details": err.Error()})
		return
	}
	if session.Vehicle == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "vehicle data not found in session"})
		return
	}

	resp := CreateResponse{Errors: []string{}}

	switch req.Provider {
	case "neo":
		r, err := oc.createNeo(session, &req)
		if err != nil {
			resp.Errors = append(resp.Errors, "neo: "+err.Error())
		} else {
			resp.Neo = r
		}
	case "gross":
		r, err := oc.createGross(session, &req)
		if err != nil {
			resp.Errors = append(resp.Errors, "gross: "+err.Error())
		} else {
			resp.Gross = r
		}
	case "euroasia":
		r, err := oc.createEuroasia(session, &req)
		if err != nil {
			resp.Errors = append(resp.Errors, "euroasia: "+err.Error())
		} else {
			resp.Euroasia = r
		}
	case "trust":
		r, err := oc.createTrust(session, &req)
		if err != nil {
			resp.Errors = append(resp.Errors, "trust: "+err.Error())
		} else {
			resp.Trust = r
		}
	case "apex":
		r, err := oc.createApex(session, &req)
		if err != nil {
			resp.Errors = append(resp.Errors, "apex: "+err.Error())
		} else {
			resp.Apex = r
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider must be one of: neo, gross, euroasia, trust, apex"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ---------------- Provider implementations ----------------

func (oc *OsagoCreateController) createNeo(session *SessionData, req *CreateRequest) (interface{}, error) {
	ownerType := extractNestedString(session.Vehicle, "data", "owner", "type")
	if ownerType == "organization" {
		return nil, fmt.Errorf("Neo Insurance не поддерживает юридические лица")
	}
	// period_id, driver_restriction, amount_uzs — из сессии (calculate_snapshot), если был вызван Calculate; иначе из тела запроса
	periodID := req.PeriodID
	driverRestriction := req.DriverRestriction
	amountUZS := req.AmountUZS
	if snap := session.CalculateSnapshot; snap != nil {
		if snap.PeriodID >= 1 && snap.PeriodID <= 3 {
			periodID = snap.PeriodID
		}
		driverRestriction = snap.DriverRestriction
		if snap.Premiums != nil && snap.Premiums["neo"] > 0 {
			amountUZS = snap.Premiums["neo"]
		}
	}
	if periodID != 1 && periodID != 2 {
		return nil, fmt.Errorf("Neo Insurance требует period_id 1 или 2 (вызовите сначала Calculate с period_id 1/2 по этой сессии или передайте period_id в теле)")
	}
	if req.PhoneNumber == "" {
		return nil, fmt.Errorf("phone_number обязателен для Neo create")
	}
	if amountUZS <= 0 {
		return nil, fmt.Errorf("amount_uzs обязателен для Neo create (вызовите сначала Calculate по этой сессии или передайте amount_uzs в теле)")
	}

	v := session.Vehicle
	gos := strings.TrimSpace(strings.ToUpper(extractNestedString(v, "data", "license_plate")))
	techS := strings.TrimSpace(extractNestedString(v, "data", "tech_passport", "series"))
	techN := strings.TrimSpace(extractNestedString(v, "data", "tech_passport", "number"))
	if gos == "" || techS == "" || techN == "" {
		return nil, fmt.Errorf("недостаточно данных о машине")
	}

	// Владелец ТС — только из сессии Find: number/series из documents[] (IDMS_RECV_MVD_IDCARD_CITIZEN), как в neoFlow 553-562
	ownerPersonObj := extractNestedValue(v, "data", "owner", "person")
	ownerPassS, ownerPassN := "", ""
	if ownerPersonObj != nil {
		ownerPassS, ownerPassN = extractPersonPassportFromFindSession(ownerPersonObj)
	}
	if (ownerPassS == "" || ownerPassN == "") && session.Owner != nil && *session.Owner && session.Person != nil {
		ownerPassS, ownerPassN = extractPersonPassportFromFindSession(session.Person)
	}
	if ownerPassS == "" || ownerPassN == "" {
		ownerPassS = extractNestedString(v, "data", "owner", "person", "passport", "series")
		ownerPassN = extractNestedString(v, "data", "owner", "person", "passport", "number")
	}
	ownerBirth := extractNestedString(v, "data", "owner", "person", "birthdate")
	if ownerBirth == "" {
		ownerBirth = extractNestedString(session.Person, "data", "birthdate")
	}
	if ownerPassS == "" || ownerPassN == "" || ownerBirth == "" {
		return nil, fmt.Errorf("не хватает данных владельца (passport/birthdate) в сессии Find — проверьте vehicle.data.owner.person или person.data (passport/documents)")
	}

	// Заявитель: из тела запроса, иначе = владелец ТС (по neoFlow applicant задаётся в запросе)
	applicantPassS := strings.TrimSpace(req.ApplicantPassportSeries)
	applicantPassN := strings.TrimSpace(req.ApplicantPassportNumber)
	applicantBirth := strings.TrimSpace(req.ApplicantBirthdate)
	if applicantPassS == "" {
		applicantPassS = ownerPassS
	}
	if applicantPassN == "" {
		applicantPassN = ownerPassN
	}
	if applicantBirth == "" {
		applicantBirth = ownerBirth
	}

	// drivers — из сессии (calculate_snapshot), т.к. уже спрашиваются в Calculate; иначе из тела
	driversList := req.Drivers
	if snap := session.CalculateSnapshot; snap != nil && len(snap.Drivers) > 0 {
		driversList = snap.Drivers
	}

	numberDriversID := 1
	if driverRestriction {
		numberDriversID = 4
	}

	var drivers []map[string]interface{}
	if driverRestriction {
		for _, d := range driversList {
			licenseIssue := d.LicenseIssueDate
			if licenseIssue == "" {
				licenseIssue = "2020-01-01"
			}
			drivers = append(drivers, map[string]interface{}{
				"passport__seria":   d.PassportSeries,
				"passport__number":  d.PassportNumber,
				"driver_birthday":   formatDateDDMMYYYY(d.Birthdate),
				"licenseNumber":     d.LicenseNumber,
				"licenseSeria":      d.LicenseSeries,
				"licenseIssueDate":  formatDateDDMMYYYY(licenseIssue),
				"relative":          d.Relative,
			})
		}
	}

	// applicant_is_driver: из тела запроса (Neo); если не передан — по паспорту заявителя и drivers
	applicantIsDriver := false
	if req.ApplicantIsDriver != nil {
		applicantIsDriver = *req.ApplicantIsDriver
	} else {
		for _, d := range driversList {
			if strings.TrimSpace(d.PassportSeries) == applicantPassS && strings.TrimSpace(d.PassportNumber) == applicantPassN {
				applicantIsDriver = true
				break
			}
		}
	}

	body := map[string]interface{}{
		"gos_number":            gos,
		"tech_sery":             techS,
		"tech_number":           techN,
		"period_id":             periodID,
		"number_drivers_id":     numberDriversID,
		"owner__pass_seria":     ownerPassS,
		"owner__pass_number":    ownerPassN,
		"owner_birthday":        formatDateDDMMYYYY(ownerBirth),
		"applicant__pass_seria": applicantPassS,
		"applicant__pass_number": applicantPassN,
		"applicant__birthday":   formatDateDDMMYYYY(applicantBirth),
		"applicant_is_driver":   applicantIsDriver,
		"phone_number":         req.PhoneNumber,
		"drivers":               drivers,
		"amount_uzs":            amountUZS,
	}
	if req.StartDate != "" {
		body["startDate"] = formatDateDDMMYYYY(req.StartDate)
	}

	url := oc.cfg.NeoBaseURL + "/api/osago-neo/save-policy/v2"
	res, err := oc.makeProviderRequest("POST", url, body, oc.cfg.NeoLogin, oc.cfg.NeoPassword, "")
	if err != nil && strings.Contains(err.Error(), "Vehicle not found") {
		return nil, fmt.Errorf("%w (Neo ищет ТС в своей базе: убедитесь, что по этому госномеру/техпаспорту уже делали расчёт Neo, или что ТС есть в базе Neo)", err)
	}
	return res, err
}

func (oc *OsagoCreateController) createGross(session *SessionData, req *CreateRequest) (interface{}, error) {
	// period_id, driver_restriction, amount_uzs — из сессии (calculate_snapshot), если был вызван Calculate; иначе из тела запроса
	periodID := req.PeriodID
	driverRestriction := req.DriverRestriction
	amountUZS := req.AmountUZS
	if snap := session.CalculateSnapshot; snap != nil {
		if snap.PeriodID >= 1 && snap.PeriodID <= 3 {
			periodID = snap.PeriodID
		}
		driverRestriction = snap.DriverRestriction
		if snap.Premiums != nil && snap.Premiums["gross"] > 0 {
			amountUZS = snap.Premiums["gross"]
		}
	}
	if periodID == 3 {
		return nil, fmt.Errorf("Gross Insurance не поддерживает период 20 дней")
	}
	if req.StartDate == "" {
		return nil, fmt.Errorf("start_date обязателен для Gross create")
	}
	if req.PhoneNumber == "" {
		return nil, fmt.Errorf("phone_number обязателен для Gross create")
	}
	if amountUZS <= 0 {
		return nil, fmt.Errorf("amount_uzs обязателен для Gross create (вызовите сначала Calculate по этой сессии или передайте amount_uzs в теле)")
	}
	// drivers — из сессии (calculate_snapshot), т.к. уже спрашиваются в Calculate; иначе из тела
	driversList := req.Drivers
	if snap := session.CalculateSnapshot; snap != nil && len(snap.Drivers) > 0 {
		driversList = snap.Drivers
	}
	if len(driversList) == 0 {
		return nil, fmt.Errorf("drivers[] обязателен для Gross create (вызовите сначала Calculate с водителями по этой сессии или передайте drivers в теле)")
	}

	v := session.Vehicle
	gos := extractNestedString(v, "data", "license_plate")
	techS := extractNestedString(v, "data", "tech_passport", "series")
	techN := extractNestedString(v, "data", "tech_passport", "number")
	if gos == "" || techS == "" || techN == "" {
		return nil, fmt.Errorf("недостаточно данных о машине")
	}

	ownerType := extractNestedString(v, "data", "owner", "type")

	numberDriversID := 1
	if driverRestriction {
		numberDriversID = 4
	}

	rep := driversList[0]
	repPinfl := extractNestedString(session.Person, "data", "pinfls", "0")
	if repPinfl == "" {
		repPinfl = extractNestedString(session.Person, "data", "external_id")
	}
	if repPinfl == "" && rep.Birthdate != "" {
		if p, err := oc.euroasiaLookupPinflByPassport(rep.Birthdate, rep.PassportSeries, rep.PassportNumber); err == nil {
			repPinfl = p
		}
	}
	if repPinfl == "" {
		repPinfl = "00000000000000"
	}

	ownerObj := map[string]interface{}{}
	if ownerType == "organization" {
		inn := extractNestedString(v, "data", "owner", "organization", "inn")
		name := extractNestedString(v, "data", "owner", "organization", "name_short")
		if name == "" {
			name = extractNestedString(v, "data", "owner", "organization", "name")
		}
		ownerObj["organization"] = map[string]interface{}{
			"inn":  toInt(inn),
			"name": name,
		}
	}
	ownerObj["person"] = map[string]interface{}{
		"pass_seria":  rep.PassportSeries,
		"pass_number": rep.PassportNumber,
		"pinfl":       repPinfl,
	}

	applicantObj := map[string]interface{}{
		"pass_seria": rep.PassportSeries,
		"pass_number": rep.PassportNumber,
		"pinfl": repPinfl,
		"is_driver": false,
		"licenseSeria": rep.LicenseSeries,
		"licenseNumber": rep.LicenseNumber,
		"licenseIssueDate": formatDateDDMMYYYY(rep.LicenseIssueDate),
		"relative": rep.Relative,
	}

	var drivers []map[string]interface{}
	if driverRestriction {
		for _, d := range driversList {
			licenseIssue := d.LicenseIssueDate
			if licenseIssue == "" {
				licenseIssue = "2020-01-01"
			}
			drivers = append(drivers, map[string]interface{}{
				"passport__seria":  d.PassportSeries,
				"passport__number": d.PassportNumber,
				"driver_birthday":  formatDateDDMMYYYY(d.Birthdate),
				"licenseSeria":     d.LicenseSeries,
				"licenseNumber":    d.LicenseNumber,
				"licenseIssueDate": formatDateDDMMYYYY(licenseIssue),
				"relative":         d.Relative,
			})
		}
	} else {
		licenseIssue := rep.LicenseIssueDate
		if licenseIssue == "" {
			licenseIssue = "2020-01-01"
		}
		drivers = []map[string]interface{}{
			{
				"passport__seria":  rep.PassportSeries,
				"passport__number": rep.PassportNumber,
				"driver_birthday":  formatDateDDMMYYYY(rep.Birthdate),
				"licenseSeria":     rep.LicenseSeries,
				"licenseNumber":    rep.LicenseNumber,
				"licenseIssueDate": formatDateDDMMYYYY(licenseIssue),
				"relative":         rep.Relative,
			},
		}
	}

	body := map[string]interface{}{
		"details": map[string]interface{}{
			"start_date":        formatDateDDMMYYYY(req.StartDate),
			"period_id":         periodID,
			"number_drivers_id": numberDriversID,
			"phone_number":      req.PhoneNumber,
			"amount_uzs":        amountUZS,
		},
		"techPassport": map[string]interface{}{
			"govNumber":  gos,
			"tech_sery":  techS,
			"tech_number": techN,
		},
		"owner":     ownerObj,
		"applicant": applicantObj,
		"drivers":   drivers,
	}

	url := oc.cfg.GrossBaseURL + "/osago-gross/save-policy-manual"
	return oc.makeProviderRequest("POST", url, body, oc.cfg.GrossLogin, oc.cfg.GrossPassword, "")
}

func (oc *OsagoCreateController) createEuroasia(session *SessionData, req *CreateRequest) (interface{}, error) {
	if req.StartDate == "" {
		return nil, fmt.Errorf("start_date обязателен для EuroAsia create")
	}
	if req.PhoneNumber == "" {
		return nil, fmt.Errorf("phone_number обязателен для EuroAsia create")
	}
	// district_id: из запроса или автоматически из session (person.district.id из Find API)
	districtID := strings.TrimSpace(req.EuroasiaDistrictID)
	if districtID == "" && session.Person != nil {
		districtID = extractNestedString(session.Person, "data", "district", "id")
	}
	if districtID == "" {
		return nil, fmt.Errorf("euroasia_district_id обязателен для EuroAsia create (или получите session через find с паспортом/пинфл физлица — тогда район подставится из ответа автоматически)")
	}

	v := session.Vehicle
	license := extractNestedString(v, "data", "license_plate")
	techS := extractNestedString(v, "data", "tech_passport", "series")
	techN := extractNestedString(v, "data", "tech_passport", "number")
	if license == "" || techS == "" || techN == "" {
		return nil, fmt.Errorf("недостаточно данных о машине")
	}

	ownerType := extractNestedString(v, "data", "owner", "type") // person|organization

	// period -> seasonal_insurance_id UUID
	var seasonalInsuranceID string
	switch req.PeriodID {
	case 1:
		seasonalInsuranceID = "8465a831-850f-4445-a995-ef71195094ab" // 365
	case 2:
		seasonalInsuranceID = "9848096e-cc12-4dbd-893b-41f2cdfc9a0e" // 180
	case 3:
		seasonalInsuranceID = "0d546748-0ba6-43bc-9ce2-1b977ad9e494" // 20
	}

	// details drivers
	var detailsDrivers []map[string]interface{}
	if req.DriverRestriction {
		if len(req.Drivers) == 0 {
			return nil, fmt.Errorf("drivers[] обязателен если driver_restriction=true для EuroAsia create")
		}
		for _, d := range req.Drivers {
			detailsDrivers = append(detailsDrivers, map[string]interface{}{
				"passport_birthdate": d.Birthdate,
				"passport_number":    d.PassportNumber,
				"passport_series":    d.PassportSeries,
				"relative_id":        relativeToEuroasiaUUID(d.Relative),
			})
		}
	}

	// insurant
	insType := strings.TrimSpace(req.EuroasiaInsurantType)
	if insType == "" {
		insType = ownerType
		if insType == "" {
			insType = "person"
		}
	}
	insurant := map[string]interface{}{
		"district_id":  districtID,
		"phone_number": req.PhoneNumber,
		"type":         insType,
	}
	if insType == "organization" {
		inn := extractNestedString(v, "data", "owner", "organization", "inn")
		if inn == "" {
			inn = extractNestedString(session.Organization, "data", "inn")
		}
		insurant["organization"] = map[string]interface{}{"inn": inn}
	} else {
		// person insurant: берём из drivers[0] если есть, иначе из session.person или owner.person
		var ps, pn, bd string
		if len(req.Drivers) > 0 {
			ps, pn, bd = req.Drivers[0].PassportSeries, req.Drivers[0].PassportNumber, req.Drivers[0].Birthdate
		}
		if ps == "" || pn == "" {
			ps = extractNestedString(session.Person, "data", "passport", "series")
			pn = extractNestedString(session.Person, "data", "passport", "number")
		}
		if bd == "" {
			bd = extractNestedString(session.Person, "data", "birthdate")
		}
		if ps == "" || pn == "" || bd == "" {
			return nil, fmt.Errorf("не хватает данных insurant.person (passport/birthdate)")
		}
		insurant["person"] = map[string]interface{}{
			"passport_birthdate": bd,
			"passport_number":    pn,
			"passport_series":    ps,
		}
	}

	// owner
	ownerIsIns := true
	if req.EuroasiaOwnerIsInsurant != nil {
		ownerIsIns = *req.EuroasiaOwnerIsInsurant
	}
	owner := map[string]interface{}{
		"is_insurant": ownerIsIns,
		"type":        ownerType,
	}
	if ownerType == "organization" {
		inn := extractNestedString(v, "data", "owner", "organization", "inn")
		if inn == "" {
			inn = extractNestedString(session.Organization, "data", "inn")
		}
		owner["organization"] = map[string]interface{}{"inn": inn}
		// добавить представителя как person (если передан)
		if len(req.Drivers) > 0 {
			owner["person"] = map[string]interface{}{
				"passport_series": req.Drivers[0].PassportSeries,
				"passport_number": req.Drivers[0].PassportNumber,
			}
		}
	} else {
		ps := extractNestedString(v, "data", "owner", "person", "passport", "series")
		pn := extractNestedString(v, "data", "owner", "person", "passport", "number")
		if ps == "" || pn == "" {
			ps = extractNestedString(session.Person, "data", "passport", "series")
			pn = extractNestedString(session.Person, "data", "passport", "number")
		}
		owner["person"] = map[string]interface{}{
			"passport_series": ps,
			"passport_number": pn,
		}
	}

	body := map[string]interface{}{
		"details": map[string]interface{}{
			"driver_restriction":    req.DriverRestriction,
			"drivers":               detailsDrivers,
			"seasonal_insurance_id": seasonalInsuranceID,
			"start_at":              req.StartDate,
		},
		"insurant": insurant,
		"owner":    owner,
		"vehicle": map[string]interface{}{
			"license_number":        license,
			"tech_passport_number":  techN,
			"tech_passport_series":  techS,
		},
	}

	url := oc.cfg.EuroasiaAllBaseURL + "/api/v1/insurance/osago/create"
	return oc.makeProviderRequest("POST", url, body, "", "", oc.cfg.EuroasiaAllAPIKey)
}

func (oc *OsagoCreateController) createTrust(session *SessionData, req *CreateRequest) (interface{}, error) {
	if req.ProviderPayload != nil {
		url := oc.cfg.TrustBaseURL + "/api/osgo/create"
		return oc.makeProviderRequest("POST", url, req.ProviderPayload, oc.cfg.TrustLogin, oc.cfg.TrustPassword, "")
	}
	// Единый body: собираем payload из session + req
	if req.StartDate == "" {
		return nil, fmt.Errorf("start_date обязателен для Trust create")
	}
	v := session.Vehicle
	renumber := extractNestedString(v, "data", "license_plate")
	texpsery := extractNestedString(v, "data", "tech_passport", "series")
	texpnumber := extractNestedString(v, "data", "tech_passport", "number")
	vmodel := extractNestedString(v, "data", "model")
	year := extractNestedInt(v, "data", "manufacture_year")
	kuzov := extractNestedString(v, "data", "body_number")
	dvigatel := extractNestedString(v, "data", "engine_number")
	if renumber == "" || texpsery == "" || texpnumber == "" {
		return nil, fmt.Errorf("недостаточно данных о машине для Trust")
	}
	vehicleTypeID := extractNestedInt(v, "data", "vehicle_type", "external_id")
	if vehicleTypeID == 0 {
		vehicleTypeID = 1
	}
	useTerritory := extractNestedInt(v, "data", "use_territory_region", "external_id")
	if useTerritory <= 0 || useTerritory > 14 {
		useTerritory = 1
	}
	texpdate := formatDateDDMMYYYY(extractNestedString(v, "data", "tech_passport", "issue_date"))
	if texpdate == "" {
		texpdate = "01.01.2020"
	}
	if year == 0 {
		year = 2020
	}
	if vmodel == "" {
		vmodel = "N"
	}
	if kuzov == "" {
		kuzov = "N"
	}
	if dvigatel == "" {
		dvigatel = "N"
	}

	ownerType := extractNestedString(v, "data", "owner", "type")
	ownerFy := 0
	ownerPinfl := extractNestedString(v, "data", "owner", "person", "pinfls", "0")
	if ownerPinfl == "" {
		ownerPinfl = extractNestedString(v, "data", "owner", "person", "external_id")
	}
	ownerBirthdate := formatDateDDMMYYYY(extractNestedString(v, "data", "owner", "person", "birthdate"))
	if ownerBirthdate == "" {
		ownerBirthdate = formatDateDDMMYYYY(extractNestedString(session.Person, "data", "birthdate"))
	}
	ownerPaspSery := extractNestedString(v, "data", "owner", "person", "passport", "series")
	ownerPaspNum := extractNestedString(v, "data", "owner", "person", "passport", "number")
	ownerSurname := extractNestedString(session.Person, "data", "last_name")
	ownerName := extractNestedString(session.Person, "data", "first_name")
	ownerPatronym := extractNestedString(session.Person, "data", "middle_name")
	if ownerSurname == "" {
		ownerSurname = "N"
	}
	if ownerName == "" {
		ownerName = "N"
	}
	if ownerPatronym == "" {
		ownerPatronym = "N"
	}
	ownerInn := ""
	ownerOrgname := ""
	if ownerType == "organization" {
		ownerFy = 1
		ownerInn = extractNestedString(v, "data", "owner", "organization", "inn")
		if ownerInn == "" {
			ownerInn = extractNestedString(session.Organization, "data", "inn")
		}
		ownerOrgname = extractNestedString(v, "data", "owner", "organization", "name_short")
		if ownerOrgname == "" {
			ownerOrgname = extractNestedString(v, "data", "owner", "organization", "name")
		}
		if len(req.Drivers) == 0 {
			return nil, fmt.Errorf("для Trust (юрлицо) укажите drivers[] (минимум 1 представитель)")
		}
		rep := req.Drivers[0]
		ownerPinfl = findPinflCreate(oc, session, rep.PassportSeries, rep.PassportNumber, rep.Birthdate)
		if ownerPinfl == "" {
			ownerPinfl = "00000000000000"
		}
		ownerBirthdate = formatDateDDMMYYYY(rep.Birthdate)
		ownerPaspSery = rep.PassportSeries
		ownerPaspNum = rep.PassportNumber
		ownerSurname = "N"
		ownerName = "N"
		ownerPatronym = "N"
	}
	ownerPhone := req.PhoneNumber
	if ownerPhone == "" {
		ownerPhone = "998900000000"
	}

	periodTrust := 2
	switch req.PeriodID {
	case 1:
		periodTrust = 2
	case 2:
		periodTrust = 1
	case 3:
		periodTrust = 4
	default:
		periodTrust = 2
	}
	driverLimit := 0
	if req.DriverRestriction {
		driverLimit = 1
	}
	contractBegin := formatDateDDMMYYYY(req.StartDate)
	if contractBegin == "" {
		contractBegin = req.StartDate
	}

	var driversList []map[string]interface{}
	if req.DriverRestriction && len(req.Drivers) > 0 {
		for _, d := range req.Drivers {
			pinfl := findPinflCreate(oc, session, d.PassportSeries, d.PassportNumber, d.Birthdate)
			if pinfl == "" {
				pinfl = ownerPinfl
			}
			driversList = append(driversList, map[string]interface{}{
				"datebirth":  formatDateDDMMYYYY(d.Birthdate),
				"paspsery":   d.PassportSeries,
				"paspnumber": d.PassportNumber,
				"pinfl":      pinfl,
				"surname":    "N",
				"name":       "N",
				"patronym":   "N",
				"licnumber":  d.LicenseNumber,
				"licsery":    d.LicenseSeries,
				"licdate":    formatDateDDMMYYYY(d.LicenseIssueDate),
				"relative":   d.Relative,
				"resident":   1,
			})
		}
	} else {
		driversList = []map[string]interface{}{
			{
				"datebirth":  ownerBirthdate,
				"paspsery":   ownerPaspSery,
				"paspnumber": ownerPaspNum,
				"pinfl":      ownerPinfl,
				"surname":    ownerSurname,
				"name":       ownerName,
				"patronym":   ownerPatronym,
				"licnumber":  "",
				"licsery":    "",
				"licdate":    "",
				"relative":   0,
				"resident":   1,
			},
		}
	}

	body := map[string]interface{}{
		"renumber":           renumber,
		"texpsery":           texpsery,
		"texpnumber":         texpnumber,
		"vmodel":             vmodel,
		"type":               vehicleTypeID,
		"texpdate":           texpdate,
		"year":               year,
		"kuzov":              kuzov,
		"dvigatel":           dvigatel,
		"use_territory":      useTerritory,
		"owner_fy":           ownerFy,
		"owner_pinfl":        ownerPinfl,
		"owner_birthdate":    ownerBirthdate,
		"owner_pasp_sery":    ownerPaspSery,
		"owner_pasp_num":     ownerPaspNum,
		"owner_surname":      ownerSurname,
		"owner_name":         ownerName,
		"owner_patronym":     ownerPatronym,
		"owner_isdriver":     1,
		"owner_oblast":       1,
		"owner_rayon":        1001,
		"has_benefit":        1,
		"owner_phone":        ownerPhone,
		"applicant_isowner":  1,
		"driver_limit":      driverLimit,
		"contract_begin":     contractBegin,
		"period":             periodTrust,
		"drivers":            driversList,
	}
	if ownerFy == 1 {
		body["owner_inn"] = toInt(ownerInn)
		body["owner_orgname"] = ownerOrgname
	}

	url := oc.cfg.TrustBaseURL + "/api/osgo/create"
	return oc.makeProviderRequest("POST", url, body, oc.cfg.TrustLogin, oc.cfg.TrustPassword, "")
}

func findPinflCreate(oc *OsagoCreateController, session *SessionData, series, number, birthdate string) string {
	pinfl := extractNestedString(session.Person, "data", "pinfls", "0")
	if pinfl == "" {
		pinfl = extractNestedString(session.Person, "data", "external_id")
	}
	if pinfl != "" {
		return pinfl
	}
	v := session.Vehicle
	pinfl = extractNestedString(v, "data", "owner", "person", "pinfls", "0")
	if pinfl == "" {
		pinfl = extractNestedString(v, "data", "owner", "person", "external_id")
	}
	if pinfl != "" {
		return pinfl
	}
	if birthdate != "" {
		if p, err := oc.euroasiaLookupPinflByPassport(birthdate, series, number); err == nil && p != "" {
			return p
		}
	}
	return ""
}

func (oc *OsagoCreateController) createApex(session *SessionData, req *CreateRequest) (interface{}, error) {
	if req.ProviderPayload != nil {
		userID := oc.cfg.ApexUserID
		if userID == 0 {
			userID = 30541
		}
		url := oc.cfg.ApexBaseURL + "/osago?user_id=" + strconv.Itoa(userID)
		return oc.makeProviderRequest("POST", url, req.ProviderPayload, oc.cfg.ApexLogin, oc.cfg.ApexPassword, "")
	}
	// Единый body: собираем payload из session + req
	if req.StartDate == "" {
		return nil, fmt.Errorf("start_date обязателен для Apex create")
	}
	if req.AmountUZS <= 0 {
		return nil, fmt.Errorf("amount_uzs обязателен для Apex create")
	}
	if req.PhoneNumber == "" {
		return nil, fmt.Errorf("phone_number обязателен для Apex create")
	}
	v := session.Vehicle
	ownerType := extractNestedString(v, "data", "owner", "type")
	// Представитель / владелец
	var pinfl, passSery, passNum, birthDate string
	if ownerType == "organization" {
		if len(req.Drivers) == 0 {
			return nil, fmt.Errorf("для Apex (юрлицо) укажите drivers[] (минимум 1 представитель)")
		}
		d := req.Drivers[0]
		pinfl = findPinflCreate(oc, session, d.PassportSeries, d.PassportNumber, d.Birthdate)
		passSery, passNum, birthDate = d.PassportSeries, d.PassportNumber, d.Birthdate
		if pinfl == "" {
			pinfl = "00000000000000"
		}
	} else {
		pinfl = extractNestedString(v, "data", "owner", "person", "pinfls", "0")
		if pinfl == "" {
			pinfl = extractNestedString(v, "data", "owner", "person", "external_id")
		}
		passSery = extractNestedString(v, "data", "owner", "person", "passport", "series")
		passNum = extractNestedString(v, "data", "owner", "person", "passport", "number")
		birthDate = extractNestedString(v, "data", "owner", "person", "birthdate")
		if birthDate == "" {
			birthDate = extractNestedString(session.Person, "data", "birthdate")
		}
	}
	firstName := extractNestedString(session.Person, "data", "first_name")
	lastName := extractNestedString(session.Person, "data", "last_name")
	middleName := extractNestedString(session.Person, "data", "middle_name")
	if firstName == "" {
		firstName = "N"
	}
	if lastName == "" {
		lastName = "N"
	}
	if middleName == "" {
		middleName = "N"
	}
	issueDate := extractNestedString(v, "data", "owner", "person", "passport", "issued_at")
	if issueDate == "" {
		issueDate = "2022-01-01"
	}
	issueDate = strings.TrimSpace(strings.Split(issueDate, "T")[0])

	// endDate from start_date + period
	endDate := req.StartDate
	if req.PeriodID == 1 {
		// +12 months - 1 day, simple
		endDate = addMonthsToDate(req.StartDate, 12)
	} else if req.PeriodID == 2 {
		endDate = addMonthsToDate(req.StartDate, 6)
	} else {
		endDate = addDaysToDate(req.StartDate, 20)
	}

	contractTermID := "1"
	seasonalID := 7
	switch req.PeriodID {
	case 1:
		contractTermID = "1"
		seasonalID = 7
	case 2:
		contractTermID = "2"
		seasonalID = 1
	case 3:
		contractTermID = "2"
		seasonalID = 8
	}
	useTerritoryID := extractNestedString(v, "data", "use_territory_region", "external_id")
	if useTerritoryID == "" {
		useTerritoryID = "1"
	}
	vehicleTypeID := extractNestedInt(v, "data", "vehicle_type", "external_id")
	if vehicleTypeID == 0 {
		vehicleTypeID = 1
	}
	issueYear := extractNestedInt(v, "data", "manufacture_year")
	if issueYear == 0 {
		issueYear = 2020
	}

	applicant := map[string]interface{}{
		"person": map[string]interface{}{
			"passportData": map[string]interface{}{
				"pinfl":       pinfl,
				"seria":      passSery,
				"number":     passNum,
				"issuedBy":   "N",
				"issueDate":  issueDate,
			},
			"fullName": map[string]interface{}{
				"firstname":  firstName,
				"lastname":   lastName,
				"middlename": middleName,
			},
			"phoneNumber":   req.PhoneNumber,
			"gender":        "m",
			"birthDate":     birthDate,
			"regionId":      10,
			"districtId":    1005,
		},
		"address":        "N",
		"residentOfUzb":  1,
		"citizenshipId":  210,
	}
	owner := map[string]interface{}{
		"person": map[string]interface{}{
			"passportData": map[string]interface{}{
				"pinfl":      pinfl,
				"seria":     passSery,
				"number":    passNum,
				"issuedBy":  "N",
				"issueDate": issueDate + "T00:00:00",
			},
			"fullName": map[string]interface{}{
				"firstname":  firstName,
				"lastname":   lastName,
				"middlename": middleName,
			},
		},
		"applicantIsOwner": true,
	}
	if ownerType == "organization" {
		inn := extractNestedString(v, "data", "owner", "organization", "inn")
		owner["organization"] = map[string]interface{}{"inn": inn}
	}
	techSery := extractNestedString(v, "data", "tech_passport", "series")
	techNum := extractNestedString(v, "data", "tech_passport", "number")
	gosNumber := extractNestedString(v, "data", "license_plate")
	modelName := extractNestedString(v, "data", "model")
	if modelName == "" {
		modelName = "N"
	}
	bodyNumber := extractNestedString(v, "data", "body_number")
	if bodyNumber == "" {
		bodyNumber = "N"
	}
	engineNumber := extractNestedString(v, "data", "engine_number")
	if engineNumber == "" {
		engineNumber = "N"
	}

	var driversList []map[string]interface{}
	if req.DriverRestriction && len(req.Drivers) > 0 {
		for _, d := range req.Drivers {
			dpinfl := findPinflCreate(oc, session, d.PassportSeries, d.PassportNumber, d.Birthdate)
			if dpinfl == "" {
				dpinfl = pinfl
			}
			driversList = append(driversList, map[string]interface{}{
				"passportData": map[string]interface{}{
					"pinfl":      dpinfl,
					"seria":     d.PassportSeries,
					"number":    d.PassportNumber,
					"issuedBy":  "N",
					"issueDate": toYYYYMMDD(d.LicenseIssueDate),
				},
				"fullName": map[string]interface{}{
					"firstname":  "N",
					"lastname":   "N",
					"middlename": "N",
				},
				"licenseNumber":     d.LicenseNumber,
				"licenseSeria":      d.LicenseSeries,
				"relative":          d.Relative,
				"birthDate":         d.Birthdate,
				"licenseIssueDate":  d.LicenseIssueDate,
				"residentOfUzb":     1,
			})
		}
	} else {
		driversList = []map[string]interface{}{
			{
				"passportData": map[string]interface{}{
					"pinfl":     "00000000000000",
					"seria":    "AA",
					"number":   "0000000",
					"issuedBy": "N",
					"issueDate": "2020-01-01",
				},
				"fullName": map[string]interface{}{
					"firstname":  "N",
					"lastname":   "N",
					"middlename": "N",
				},
				"licenseNumber":    "",
				"licenseSeria":    "",
				"relative":        0,
				"birthDate":       birthDate,
				"licenseIssueDate": "2020-01-01",
				"residentOfUzb":   1,
			},
		}
	}

	body := map[string]interface{}{
		"applicant": applicant,
		"owner":     owner,
		"details": map[string]interface{}{
			"startDate":               req.StartDate,
			"issueDate":               req.StartDate,
			"endDate":                 endDate,
			"driverNumberRestriction": req.DriverRestriction,
		},
		"cost": map[string]interface{}{
			"discountId":                    1,
			"discountSum":                   "0",
			"insurancePremium":              req.AmountUZS,
			"sumInsured":                    "80000000",
			"contractTermConclusionId":     contractTermID,
			"useTerritoryId":                useTerritoryID,
			"commission":                    "0",
			"insurancePremiumPaidToInsurer": req.AmountUZS,
			"seasonalInsuranceId":           seasonalID,
		},
		"vehicle": map[string]interface{}{
			"techPassport": map[string]interface{}{
				"seria":  techSery,
				"number": techNum,
			},
			"modelCustomName": modelName,
			"engineNumber":    engineNumber,
			"typeId":          vehicleTypeID,
			"issueYear":       issueYear,
			"govNumber":       gosNumber,
			"bodyNumber":      bodyNumber,
			"regionId":        10,
		},
		"drivers": driversList,
	}

	userID := oc.cfg.ApexUserID
	if userID == 0 {
		userID = 30541
	}
	url := oc.cfg.ApexBaseURL + "/osago?user_id=" + strconv.Itoa(userID)
	return oc.makeProviderRequest("POST", url, body, oc.cfg.ApexLogin, oc.cfg.ApexPassword, "")
}

func toYYYYMMDD(s string) string {
	s = strings.TrimSpace(strings.Split(s, "T")[0])
	if len(s) == 10 && s[4] == '-' {
		return s
	}
	// DD.MM.YYYY -> YYYY-MM-DD
	parts := strings.Split(s, ".")
	if len(parts) == 3 {
		return fmt.Sprintf("%s-%s-%s", parts[2], parts[1], parts[0])
	}
	return s
}

func addMonthsToDate(ymd string, months int) string {
	ymd = toYYYYMMDD(ymd)
	t, err := time.Parse("2006-01-02", ymd)
	if err != nil {
		return ymd
	}
	t = t.AddDate(0, months, 0)
	return t.Format("2006-01-02")
}

func addDaysToDate(ymd string, days int) string {
	t, err := time.Parse("2006-01-02", ymd)
	if err != nil {
		return ymd
	}
	t = t.AddDate(0, 0, days)
	return t.Format("2006-01-02")
}

func toInt(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case float64:
		return int(x)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(x))
		return i
	default:
		return 0
	}
}

