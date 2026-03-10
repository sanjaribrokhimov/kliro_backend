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
	"sync"
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

// CreateRequest - единый body для create у любого провайдера (neo|gross|euroasia|trust|apex|inson).
// Один и тот же набор полей: backend сам маппит на API провайдера. Find/calculate не трогаем.
type CreateRequest struct {
	SessionID         string   `json:"session_id" binding:"required"`
	Provider          string   `json:"provider" binding:"required"` // neo|gross|euroasia|trust|apex|inson
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
	Inson    interface{} `json:"inson,omitempty"`
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

// Единые адаптеры для relative (степень родства) — наша система: 0=не родственник, 1=отец, 2=мать, 3=муж, 4=жена, 5=сын, 6=дочь, 7=старший брат, 8=младший брат, 9=старшая сестра, 10=младшая сестра

// relativeToNeo конвертирует наше значение relative в формат Neo (0-10, без изменений)
func relativeToNeo(rel int) int {
	return rel
}

// relativeToGross конвертирует наше значение relative в формат Gross.
// Gross порядок: 1=Отец, 2=Старший брат, 3=Младший брат, 4=Жена, 5=Мать, 6=Муж, 7=Сын, 8=Дочь, 9=Старшая сестра, 10=Младшая сестра
func relativeToGross(rel int) int {
	switch rel {
	case 0:
		return 0 // не родственник
	case 1:
		return 1 // Отец
	case 2:
		return 5 // Мать
	case 3:
		return 6 // Муж
	case 4:
		return 4 // Жена
	case 5:
		return 7 // Сын
	case 6:
		return 8 // Дочь
	case 7:
		return 2 // Старший брат
	case 8:
		return 3 // Младший брат
	case 9:
		return 9 // Старшая сестра
	case 10:
		return 10 // Младшая сестра
	default:
		return 0
	}
}

// relativeToEuroasiaUUID конвертирует наше значение relative в UUID формат EuroAsia
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

// relativeToTrust конвертирует наше значение relative в формат Trust (0-10, без изменений)
func relativeToTrust(rel int) int {
	return rel
}

// relativeToApex конвертирует наше значение relative в формат Apex (0-10, без изменений)
func relativeToApex(rel int) int {
	return rel
}

func (oc *OsagoCreateController) euroasiaLookupPinflByPassport(birthdateYYYYMMDD, passportSeries, passportNumber string) (string, error) {
	data, err := oc.euroasiaLookupPersonByPassport(birthdateYYYYMMDD, passportSeries, passportNumber)
	if err != nil {
		return "", err
	}
	pinfl := extractNestedString(data, "pinfls", "0")
	if pinfl == "" {
		pinfl = extractNestedString(data, "external_id")
	}
	return pinfl, nil
}

// euroasiaLookupPersonByPassport возвращает объект data персоны из EuroAsia API (find-by-birthdate) для подстановки ФИО, паспорта и т.д. в Apex.
func (oc *OsagoCreateController) euroasiaLookupPersonByPassport(birthdateYYYYMMDD, passportSeries, passportNumber string) (map[string]interface{}, error) {
	url := oc.cfg.EuroasiaAllBaseURL + "/api/v1/insurance/persons/find-by-birthdate"
	birthNorm := strings.TrimSpace(strings.Split(birthdateYYYYMMDD, "T")[0])
	reqBody := map[string]string{
		"birthdate":       birthNorm,
		"passport_series": strings.TrimSpace(passportSeries),
		"passport_number": strings.TrimSpace(passportNumber),
	}
	resp, err := oc.makeProviderRequest("POST", url, reqBody, "", "", oc.cfg.EuroasiaAllAPIKey)
	if err != nil {
		return nil, err
	}
	data, _ := extractNestedValue(resp, "data").(map[string]interface{})
	return data, nil
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
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "ru")
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

// enrichDriversWithLicenseFromFind автоматически заполняет license_series и license_number для водителей из Find API,
// если переданы только паспорт (серия, номер) и дата рождения, но нет данных водительского удостоверения.
// Использует EuroAsia person API (find-by-birthdate) и извлекает IDMS_RECV_DRIVERS_LICENSE из documents.
func (oc *OsagoCreateController) enrichDriversWithLicenseFromFind(drivers []Driver) []Driver {
	if len(drivers) == 0 {
		return drivers
	}
	enriched := make([]Driver, len(drivers))
	for i, d := range drivers {
		enriched[i] = d
		// Если есть паспорт и дата рождения, но нет license — ищем через Find API
		if d.PassportSeries != "" && d.PassportNumber != "" && d.Birthdate != "" {
			if d.LicenseSeries == "" || d.LicenseNumber == "" {
				birthNorm := toYYYYMMDD(d.Birthdate)
				if personData, err := oc.euroasiaLookupPersonByPassport(birthNorm, d.PassportSeries, d.PassportNumber); err == nil && personData != nil {
					// Ищем водительское удостоверение в documents (IDMS_RECV_DRIVERS_LICENSE)
					docs, _ := extractNestedValue(personData, "documents").([]interface{})
					for _, doc := range docs {
						docMap, _ := doc.(map[string]interface{})
						if docMap == nil {
							continue
						}
						docType := extractNestedString(docMap, "document_type")
						if docType == "IDMS_RECV_DRIVERS_LICENSE" {
							licenseSeries := extractNestedString(docMap, "series")
							licenseNumber := extractNestedString(docMap, "number")
							if licenseSeries != "" && licenseNumber != "" {
								enriched[i].LicenseSeries = licenseSeries
								enriched[i].LicenseNumber = licenseNumber
								// Также можно взять issue_date если есть
								if issueDate := extractNestedString(docMap, "issue_date"); issueDate != "" && enriched[i].LicenseIssueDate == "" {
									enriched[i].LicenseIssueDate = toYYYYMMDD(issueDate)
								}
								break
							}
						}
					}
				}
			}
		}
	}
	return enriched
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
	ownerType := extractNestedString(session.Vehicle, "data", "owner", "type")
	if ownerType == "organization" && (req.Provider == "neo" || req.Provider == "gross") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "для юридического лица Neo и Gross не поддерживаются; используйте provider: trust, euroasia, apex или inson"})
		return
	}

	// Автозаполнение license_series/license_number из Find API для drivers и applicant (если переданы только паспорт + дата рождения)
	req.Drivers = oc.enrichDriversWithLicenseFromFind(req.Drivers)
	if req.ApplicantPassportSeries != "" && req.ApplicantPassportNumber != "" && req.ApplicantBirthdate != "" {
		// Для applicant тоже можно найти license, но обычно он не водитель — пропускаем
	}

	resp := CreateResponse{Errors: []string{}}

	switch req.Provider {
	case "neo":
		r, err := oc.createNeo(session, &req)
		if err != nil {
			resp.Errors = append(resp.Errors, "neo: "+err.Error())
		} else {
			resp.Neo = oc.withPaymentUrls("neo", r)
		}
	case "gross":
		r, err := oc.createGross(session, &req)
		if err != nil {
			resp.Errors = append(resp.Errors, "gross: "+err.Error())
		} else {
			resp.Gross = oc.withPaymentUrls("gross", r)
		}
	case "euroasia":
		r, err := oc.createEuroasia(session, &req)
		if err != nil {
			resp.Errors = append(resp.Errors, "euroasia: "+err.Error())
		} else {
			// Только для EuroAsia: параллельно запрашиваем payment click и payme, ответ — create + payment_click + payment_payme
			policyID := extractNestedString(r, "data", "policy_id")
			if policyID != "" {
				var clickResp, paymeResp interface{}
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					clickResp, _ = oc.euroasiaPayment(policyID, "click")
					wg.Done()
				}()
				go func() {
					paymeResp, _ = oc.euroasiaPayment(policyID, "payme")
					wg.Done()
				}()
				wg.Wait()
				euroasiaObj := map[string]interface{}{
					"create":         r,
					"payment_click":  clickResp,
					"payment_payme":  paymeResp,
				}
				resp.Euroasia = oc.withPaymentUrls("euroasia", euroasiaObj)
			} else {
				resp.Euroasia = oc.withPaymentUrls("euroasia", r)
			}
		}
	case "trust":
		r, err := oc.createTrust(session, &req)
		if err != nil {
			resp.Errors = append(resp.Errors, "trust: "+err.Error())
		} else {
			resp.Trust = oc.trustCreateResponseWithPaymentLinks(r)
		}
	case "apex":
		r, err := oc.createApex(session, &req)
		if err != nil {
			resp.Errors = append(resp.Errors, "apex: "+err.Error())
		} else {
			resp.Apex = oc.withPaymentUrls("apex", r)
		}
	case "inson":
		r, err := oc.createInson(session, &req)
		if err != nil {
			resp.Errors = append(resp.Errors, "inson: "+err.Error())
		} else {
			resp.Inson = r
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider must be one of: neo, gross, euroasia, trust, apex, inson"})
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
	// Автозаполнение license_series/license_number из Find API, если переданы только паспорт + дата рождения
	driversList = oc.enrichDriversWithLicenseFromFind(driversList)

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
				"relative":          relativeToNeo(d.Relative),
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
	// Автозаполнение license_series/license_number из Find API, если переданы только паспорт + дата рождения
	driversList = oc.enrichDriversWithLicenseFromFind(driversList)
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

	// owner — из сессии Find (vehicle owner, documents/passport), как для Neo/EuroAsia
	ownerPassS, ownerPassN := "", ""
	ownerPersonObj := extractNestedValue(v, "data", "owner", "person")
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
	if ownerPassS == "" {
		ownerPassS = rep.PassportSeries
	}
	if ownerPassN == "" {
		ownerPassN = rep.PassportNumber
	}
	ownerPinfl := repPinfl
	if ownerPassS != rep.PassportSeries || ownerPassN != rep.PassportNumber {
		ownerPinfl = extractNestedString(session.Person, "data", "pinfls", "0")
		if ownerPinfl == "" {
			ownerPinfl = extractNestedString(session.Person, "data", "external_id")
		}
		if ownerPinfl == "" {
			ownerPinfl = "00000000000000"
		}
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
		"pass_seria":  ownerPassS,
		"pass_number": ownerPassN,
		"pinfl":       ownerPinfl,
	}

	// applicant — из тела (applicant_*) или первый водитель (rep); PINFL должен соответствовать паспорту заявителя (Gross проверяет)
	applicantPassS, applicantPassN := strings.TrimSpace(req.ApplicantPassportSeries), strings.TrimSpace(req.ApplicantPassportNumber)
	if applicantPassS == "" {
		applicantPassS = rep.PassportSeries
	}
	if applicantPassN == "" {
		applicantPassN = rep.PassportNumber
	}
	applicantPinfl := repPinfl
	if applicantPassS != rep.PassportSeries || applicantPassN != rep.PassportNumber {
		// Заявитель — другой человек: ищем его PINFL по паспорту (session.Person или lookup по EuroAsia)
		applicantPinfl = ""
		if session.Person != nil {
			ps, pn := extractPersonPassportFromFindSession(session.Person)
			if ps == "" {
				ps = extractNestedString(session.Person, "data", "passport", "series")
				pn = extractNestedString(session.Person, "data", "passport", "number")
			}
			if ps == applicantPassS && pn == applicantPassN {
				applicantPinfl = extractNestedString(session.Person, "data", "pinfls", "0")
				if applicantPinfl == "" {
					applicantPinfl = extractNestedString(session.Person, "data", "external_id")
				}
			}
		}
		if applicantPinfl == "" && req.ApplicantBirthdate != "" {
			if p, err := oc.euroasiaLookupPinflByPassport(req.ApplicantBirthdate, applicantPassS, applicantPassN); err == nil {
				applicantPinfl = p
			}
		}
		if applicantPinfl == "" {
			applicantPinfl = "00000000000000"
		}
	}
	applicantBirthdate := strings.TrimSpace(req.ApplicantBirthdate)
	if applicantBirthdate == "" {
		applicantBirthdate = rep.Birthdate
	}
	// Gross API: заявитель должен указать либо ПИНФЛ, либо дату рождения (только одно из них)
	applicantBirthdateGross := applicantBirthdate
	if idx := strings.Index(applicantBirthdate, "T"); idx > 0 {
		applicantBirthdateGross = strings.TrimSpace(applicantBirthdate[:idx])
	}
	applicantObj := map[string]interface{}{
		"pass_seria": applicantPassS,
		"pass_number": applicantPassN,
		"is_driver":  req.ApplicantIsDriver != nil && *req.ApplicantIsDriver,
		"licenseSeria": rep.LicenseSeries,
		"licenseNumber": rep.LicenseNumber,
		"licenseIssueDate": formatDateDDMMYYYY(rep.LicenseIssueDate),
		"relative": relativeToNeo(rep.Relative),
	}
	if applicantPinfl != "" && applicantPinfl != "00000000000000" {
		applicantObj["pinfl"] = applicantPinfl
	} else {
		applicantObj["birthdate"] = applicantBirthdateGross
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
				"relative":         relativeToGross(d.Relative),
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
				"relative":         relativeToGross(rep.Relative),
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

	v := session.Vehicle
	ownerType := extractNestedString(v, "data", "owner", "type")
	// district_id: из запроса, затем из session (person или organization), иначе дефолт для юрлица
	districtID := strings.TrimSpace(req.EuroasiaDistrictID)
	if districtID == "" && session.Person != nil {
		districtID = extractNestedString(session.Person, "data", "district", "id")
	}
	if districtID == "" && session.Organization != nil {
		districtID = extractNestedString(session.Organization, "data", "district", "id")
		if districtID == "" {
			districtID = extractNestedString(session.Organization, "data", "region", "id")
		}
	}
	// EuroAsia принимает district_id только в формате UUID; для юрлица без района в сессии — дефолт (Ташкент)
	if districtID == "" && ownerType == "organization" {
		districtID = "00000000-0000-0000-0000-000000000001"
	}
	if districtID == "" {
		return nil, fmt.Errorf("euroasia_district_id обязателен для EuroAsia create (или получите session через find с паспортом/пинфл физлица — тогда район подставится из ответа автоматически)")
	}

	license := extractNestedString(v, "data", "license_plate")
	techS := extractNestedString(v, "data", "tech_passport", "series")
	techN := extractNestedString(v, "data", "tech_passport", "number")
	if license == "" || techS == "" || techN == "" {
		return nil, fmt.Errorf("недостаточно данных о машине")
	}

	// period_id из сессии (calculate_snapshot), иначе из тела; по умолчанию 1 (чтобы seasonal_insurance_id не был пустым UUID)
	periodID := req.PeriodID
	if snap := session.CalculateSnapshot; snap != nil && snap.PeriodID >= 1 && snap.PeriodID <= 3 {
		periodID = snap.PeriodID
	}
	if periodID == 0 {
		periodID = 1
	}
	driverRestriction := req.DriverRestriction
	if snap := session.CalculateSnapshot; snap != nil {
		driverRestriction = snap.DriverRestriction
	}
	// period -> seasonal_insurance_id UUID (никогда не пустой)
	var seasonalInsuranceID string
	switch periodID {
	case 1:
		seasonalInsuranceID = "8465a831-850f-4445-a995-ef71195094ab" // 365
	case 2:
		seasonalInsuranceID = "9848096e-cc12-4dbd-893b-41f2cdfc9a0e" // 180
	case 3:
		seasonalInsuranceID = "0d546748-0ba6-43bc-9ce2-1b977ad9e494" // 20
	default:
		seasonalInsuranceID = "8465a831-850f-4445-a995-ef71195094ab"
	}

	// drivers — из сессии (calculate_snapshot) или из тела
	driversList := req.Drivers
	if snap := session.CalculateSnapshot; snap != nil && len(snap.Drivers) > 0 {
		driversList = snap.Drivers
	}
	// Автозаполнение license_series/license_number из Find API, если переданы только паспорт + дата рождения
	driversList = oc.enrichDriversWithLicenseFromFind(driversList)
	detailsDrivers := []map[string]interface{}{}
	if driverRestriction {
		if len(driversList) == 0 {
			return nil, fmt.Errorf("drivers[] обязателен если driver_restriction=true для EuroAsia create (вызовите Calculate с водителями или передайте drivers в теле)")
		}
		for _, d := range driversList {
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
		// person insurant: из тела (applicant_*) или из drivers[0], иначе session.person
		ps := strings.TrimSpace(req.ApplicantPassportSeries)
		pn := strings.TrimSpace(req.ApplicantPassportNumber)
		bd := strings.TrimSpace(req.ApplicantBirthdate)
		if (ps == "" || pn == "") && len(driversList) > 0 {
			ps, pn, bd = driversList[0].PassportSeries, driversList[0].PassportNumber, driversList[0].Birthdate
		}
		if (ps == "" || pn == "") && session.Person != nil {
			ps, pn = extractPersonPassportFromFindSession(session.Person)
		}
		if ps == "" || pn == "" {
			ps = extractNestedString(session.Person, "data", "passport", "series")
			pn = extractNestedString(session.Person, "data", "passport", "number")
		}
		if bd == "" {
			bd = extractNestedString(session.Person, "data", "birthdate")
		}
		if ps == "" || pn == "" || bd == "" {
			return nil, fmt.Errorf("не хватает данных insurant.person (passport/birthdate); укажите applicant_* в теле или вызовите Calculate с водителями)")
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
		// owner person — из сессии Find (documents/passport), как для Neo
		ownerPersonObj := extractNestedValue(v, "data", "owner", "person")
		ps, pn := "", ""
		if ownerPersonObj != nil {
			ps, pn = extractPersonPassportFromFindSession(ownerPersonObj)
		}
		if (ps == "" || pn == "") && session.Owner != nil && *session.Owner && session.Person != nil {
			ps, pn = extractPersonPassportFromFindSession(session.Person)
		}
		if ps == "" || pn == "" {
			ps = extractNestedString(v, "data", "owner", "person", "passport", "series")
			pn = extractNestedString(v, "data", "owner", "person", "passport", "number")
		}
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
			"driver_restriction":    driverRestriction,
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

// euroasiaPayment вызывает API оплаты EuroAsia (click или payme); promocode статический PROMO2024
func (oc *OsagoCreateController) euroasiaPayment(policyID, gateway string) (interface{}, error) {
	if policyID == "" {
		return nil, fmt.Errorf("policy_id пустой")
	}
	url := oc.cfg.EuroasiaAllBaseURL + "/api/v1/insurance/policies/" + policyID + "/payments"
	body := map[string]string{
		"gateway":   gateway,
		"promocode": "PROMO2024",
	}
	return oc.makeProviderRequest("POST", url, body, "", "", oc.cfg.EuroasiaAllAPIKey)
}

func (oc *OsagoCreateController) createTrust(session *SessionData, req *CreateRequest) (interface{}, error) {
	if req.ProviderPayload != nil {
		url := oc.cfg.TrustBaseURL + "/api/osgo/create"
		return oc.makeProviderRequest("POST", url, req.ProviderPayload, oc.cfg.TrustLogin, oc.cfg.TrustPassword, "")
	}
	// period_id, driver_restriction, drivers — из сессии (calculate_snapshot), как у Neo/Gross/Apex
	periodID := req.PeriodID
	driverRestriction := req.DriverRestriction
	trustDriversList := req.Drivers
	if snap := session.CalculateSnapshot; snap != nil {
		if snap.PeriodID >= 1 && snap.PeriodID <= 3 {
			periodID = snap.PeriodID
		}
		driverRestriction = snap.DriverRestriction
		if len(snap.Drivers) > 0 {
			trustDriversList = snap.Drivers
		}
	}
	// Автозаполнение license_series/license_number из Find API, если переданы только паспорт + дата рождения
	trustDriversList = oc.enrichDriversWithLicenseFromFind(trustDriversList)
	if req.StartDate == "" {
		return nil, fmt.Errorf("start_date обязателен для Trust create")
	}
	// Trust API принимает только period 1 (6 мес) или 2 (12 мес); период 20 дней не поддерживается
	if periodID == 3 {
		return nil, fmt.Errorf("Trust поддерживает только period_id 1 (12 мес) или 2 (6 мес); для периода 20 дней выберите другого провайдера (например Apex)")
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
	// Trust принимает type/regionId/districtId из своих справочников; берём из Find и маппим
	trustVehicleTypeID := mapFindVehicleTypeToTrust(vehicleTypeID)
	ownerRegion := extractNestedInt(v, "data", "owner", "person", "region", "external_id")
	if ownerRegion == 0 {
		ownerRegion = 10
	}
	ownerDistrict := extractNestedInt(v, "data", "owner", "person", "district", "external_id")
	if ownerDistrict == 0 {
		ownerDistrict = 1001
	}
	trustDistrict := mapFindDistrictToTrust(ownerRegion, ownerDistrict)
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
	// Серия/номер документа владельца — как в Neo/Gross: extractPersonPassportFromFindSession (сначала ID-карта/documents, потом passport)
	ownerPersonObj := extractNestedValue(v, "data", "owner", "person")
	ownerPaspSery, ownerPaspNum := "", ""
	if ownerPersonObj != nil {
		ownerPaspSery, ownerPaspNum = extractPersonPassportFromFindSession(ownerPersonObj)
	}
	if (ownerPaspSery == "" || ownerPaspNum == "") && session.Owner != nil && *session.Owner && session.Person != nil {
		ownerPaspSery, ownerPaspNum = extractPersonPassportFromFindSession(session.Person)
	}
	if ownerPaspSery == "" || ownerPaspNum == "" {
		ownerPaspSery = extractNestedString(v, "data", "owner", "person", "passport", "series")
		ownerPaspNum = extractNestedString(v, "data", "owner", "person", "passport", "number")
	}
	// ФИО владельца — сначала vehicle.owner.person, затем session.Person (как Neo/Gross)
	var ownerSurname, ownerName, ownerPatronym string
	ownerSurname = extractNestedString(v, "data", "owner", "person", "last_name")
	if ownerSurname == "" {
		ownerSurname = extractNestedString(session.Person, "data", "last_name")
	}
	ownerName = extractNestedString(v, "data", "owner", "person", "first_name")
	if ownerName == "" {
		ownerName = extractNestedString(session.Person, "data", "first_name")
	}
	ownerPatronym = extractNestedString(v, "data", "owner", "person", "middle_name")
	if ownerPatronym == "" {
		ownerPatronym = extractNestedString(session.Person, "data", "middle_name")
	}
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
		if len(trustDriversList) == 0 {
			return nil, fmt.Errorf("для Trust (юрлицо) укажите drivers[] (минимум 1 представитель) или вызовите Calculate с водителями")
		}
		rep := trustDriversList[0]
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

	// Trust: 1 -> 6 мес, 2 -> 12 мес (period_id 3 уже отсечён выше)
	periodTrust := 2
	switch periodID {
	case 1:
		periodTrust = 2 // 12 мес
	case 2:
		periodTrust = 1 // 6 мес
	default:
		periodTrust = 2
	}
	driverLimit := 0
	if driverRestriction {
		driverLimit = 1
	}
	contractBegin := formatDateDDMMYYYY(req.StartDate)
	if contractBegin == "" {
		contractBegin = req.StartDate
	}

	var driversList []map[string]interface{}
	if driverRestriction && len(trustDriversList) > 0 {
		for _, d := range trustDriversList {
			pinfl := findPinflCreate(oc, session, d.PassportSeries, d.PassportNumber, d.Birthdate)
			if pinfl == "" {
				pinfl = ownerPinfl
			}
			licdate := formatDateDDMMYYYY(d.LicenseIssueDate)
			if licdate == "" {
				licdate = "01.01.0001"
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
				"licdate":    licdate,
				"relative":   relativeToTrust(d.Relative),
				"resident":   1,
			})
		}
	} else {
		// Trust: при неограниченном полисе (driver_limit=0) не отправлять водителей
		driversList = []map[string]interface{}{}
	}

	body := map[string]interface{}{
		"renumber":           renumber,
		"texpsery":           texpsery,
		"texpnumber":         texpnumber,
		"vmodel":             vmodel,
		"type":               trustVehicleTypeID,
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
		"owner_oblast":       ownerRegion,
		"owner_rayon":        trustDistrict,
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

// withPaymentUrls добавляет в ответ create единые поля click_url и payme_url для фронта (один обработчик для всех провайдеров).
// Сырой ответ не меняется — копируется и дополняется полями из провайдер-специфичных путей.
func (oc *OsagoCreateController) withPaymentUrls(provider string, raw interface{}) interface{} {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return raw
	}
	out := make(map[string]interface{})
	for k, v := range m {
		out[k] = v
	}
	var clickURL, paymeURL string
	switch provider {
	case "neo":
		// Neo: response.url (click), response.payme_url (payme)
		resp := extractNestedValue(m, "response")
		if resp != nil {
			clickURL = extractNestedString(resp, "url")
			paymeURL = extractNestedString(resp, "payme_url")
		}
	case "gross":
		// Gross: response.click.url, response.payme.url
		resp := extractNestedValue(m, "response")
		if resp != nil {
			clickURL = extractNestedString(resp, "click", "url")
			paymeURL = extractNestedString(resp, "payme", "url")
		}
	case "euroasia":
		// EuroAsia: payment_click.data.payment_link, payment_payme.data.payment_link
		clickURL = extractNestedString(m, "payment_click", "data", "payment_link")
		paymeURL = extractNestedString(m, "payment_payme", "data", "payment_link")
	case "apex":
		// Apex: click_link, payme_link
		clickURL = extractNestedString(m, "click_link")
		paymeURL = extractNestedString(m, "payme_link")
	}
	out["click_url"] = clickURL
	out["payme_url"] = paymeURL
	return out
}

// trustCreateResponseWithPaymentLinks обогащает ответ Trust create платёжными ссылками Click и Payme при успехе (error=0).
func (oc *OsagoCreateController) trustCreateResponseWithPaymentLinks(trustResp interface{}) interface{} {
	m, ok := trustResp.(map[string]interface{})
	if !ok {
		return trustResp
	}
	// Проверяем успешность: error должен быть 0 (число или отсутствовать)
	errVal := m["error"]
	if errVal != nil {
		if errNum, ok := errVal.(float64); ok && errNum != 0 {
			return trustResp
		}
		if errNum, ok := errVal.(int); ok && errNum != 0 {
			return trustResp
		}
	}
	anketaID := ""
	switch v := m["anketa_id"].(type) {
	case float64:
		anketaID = strconv.Itoa(int(v))
	case int:
		anketaID = strconv.Itoa(v)
	case string:
		anketaID = v
	}
	uuidStr, _ := m["uuid"].(string)
	amountStr := ""
	switch v := m["insurance_premium"].(type) {
	case string:
		amountStr = v
	case float64:
		amountStr = strconv.Itoa(int(v))
	case int:
		amountStr = strconv.Itoa(v)
	}
	amountSum := 0
	if amountStr != "" {
		amountSum, _ = strconv.Atoi(amountStr)
	}
	if anketaID == "" || amountSum <= 0 {
		return trustResp
	}
	links := oc.trustPaymentLinks(anketaID, uuidStr, amountSum)
	out := make(map[string]interface{})
	for k, v := range m {
		out[k] = v
	}
	out["click_url"] = links["click_url"]
	out["payme_url"] = links["payme_url"]
	return out
}

// trustPaymentLinks генерирует только простые URL для Click и Payme (anketa_id как transaction_param/order_id).
// amountSum — сумма в сумах; для Payme конвертируется в тийины (×100).
func (oc *OsagoCreateController) trustPaymentLinks(anketaID, uuid string, amountSum int) map[string]interface{} {
	serviceID := oc.cfg.ClickServiceID
	if serviceID == "" {
		serviceID = "23572"
	}
	merchantIDClick := oc.cfg.ClickMerchantID
	if merchantIDClick == "" {
		merchantIDClick = "14417"
	}
	merchantIDPayme := oc.cfg.PaymeMerchantID
	if merchantIDPayme == "" {
		merchantIDPayme = "646c8bff2cb83937a7551c95"
	}
	returnURL := oc.cfg.PaymentReturnURL
	if returnURL == "" || returnURL == "https://your-domain.com/payment/return" {
		returnURL = "https://kliro.uz"
	}

	amountStr := strconv.Itoa(amountSum)
	clickURL := fmt.Sprintf("https://my.click.uz/services/pay?service_id=%s&merchant_id=%s&amount=%s&transaction_param=%s&return_url=%s",
		serviceID, merchantIDClick, amountStr, anketaID, returnURL)

	amountTiyins := amountSum * 100
	paramsPayme := fmt.Sprintf("m=%s;ac.order_id=%s;a=%d", merchantIDPayme, anketaID, amountTiyins)
	paymeURL := "https://checkout.paycom.uz/" + base64.StdEncoding.EncodeToString([]byte(paramsPayme))

	return map[string]interface{}{
		"click_url": clickURL,
		"payme_url": paymeURL,
	}
}

// findPinflCreate возвращает PINFL для персоны с паспортом (series, number). Проверяет совпадение паспорта с session.Person и владельцем ТС; иначе — запрос в EuroAsia.
func findPinflCreate(oc *OsagoCreateController, session *SessionData, series, number, birthdate string) string {
	series = strings.TrimSpace(series)
	number = strings.TrimSpace(number)
	// session.Person — только если паспорт совпадает
	if session.Person != nil {
		personSeries, personNumber := extractPersonPassportFromFindSession(session.Person)
		if personSeries == "" {
			personSeries = extractNestedString(session.Person, "data", "passport", "series")
			personNumber = extractNestedString(session.Person, "data", "passport", "number")
		}
		if personSeries == series && personNumber == number {
			pinfl := extractNestedString(session.Person, "data", "pinfls", "0")
			if pinfl == "" {
				pinfl = extractNestedString(session.Person, "data", "external_id")
			}
			if pinfl != "" {
				return pinfl
			}
		}
	}
	// Владелец ТС — только если паспорт совпадает
	if session.Vehicle != nil {
		v := session.Vehicle
		ownerSeries := extractNestedString(v, "data", "owner", "person", "passport", "series")
		ownerNumber := extractNestedString(v, "data", "owner", "person", "passport", "number")
		if ownerSeries == "" || ownerNumber == "" {
			ownerSeries, ownerNumber = extractPersonPassportFromFindSession(extractNestedValue(v, "data", "owner", "person"))
		}
		if ownerSeries == series && ownerNumber == number {
			pinfl := extractNestedString(v, "data", "owner", "person", "pinfls", "0")
			if pinfl == "" {
				pinfl = extractNestedString(v, "data", "owner", "person", "external_id")
			}
			if pinfl != "" {
				return pinfl
			}
		}
	}
	// Иначе — поиск по паспорту и дате рождения через EuroAsia
	if birthdate != "" {
		birthNorm := toYYYYMMDD(birthdate)
		if p, err := oc.euroasiaLookupPinflByPassport(birthNorm, series, number); err == nil && p != "" {
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
	// period_id, driver_restriction, amount_uzs, drivers — из сессии (calculate_snapshot), как у Neo/Gross
	periodID := req.PeriodID
	driverRestriction := req.DriverRestriction
	amountUZS := req.AmountUZS
	driversList := req.Drivers
	if snap := session.CalculateSnapshot; snap != nil {
		if snap.PeriodID >= 1 && snap.PeriodID <= 3 {
			periodID = snap.PeriodID
		}
		driverRestriction = snap.DriverRestriction
		if snap.Premiums != nil && snap.Premiums["apex"] > 0 {
			amountUZS = snap.Premiums["apex"]
		}
		if len(snap.Drivers) > 0 {
			driversList = snap.Drivers
		}
	}
	// Автозаполнение license_series/license_number из Find API, если переданы только паспорт + дата рождения
	driversList = oc.enrichDriversWithLicenseFromFind(driversList)
	if req.StartDate == "" {
		return nil, fmt.Errorf("start_date обязателен для Apex create")
	}
	if amountUZS <= 0 {
		return nil, fmt.Errorf("amount_uzs обязателен для Apex create (вызовите сначала Calculate по этой сессии или передайте amount_uzs в теле)")
	}
	if req.PhoneNumber == "" {
		return nil, fmt.Errorf("phone_number обязателен для Apex create")
	}
	v := session.Vehicle
	ownerType := extractNestedString(v, "data", "owner", "type")
	// Представитель / владелец
	var pinfl, passSery, passNum, birthDate string
	if ownerType == "organization" {
		if len(driversList) == 0 {
			return nil, fmt.Errorf("для Apex (юрлицо) укажите drivers[] (минимум 1 представитель) или вызовите Calculate с водителями")
		}
		d := driversList[0]
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
		// Серия/номер документа владельца — как Neo/Gross/Trust: сначала ID-карта/documents, потом passport
		ownerPersonObj := extractNestedValue(v, "data", "owner", "person")
		if ownerPersonObj != nil {
			passSery, passNum = extractPersonPassportFromFindSession(ownerPersonObj)
		}
		if (passSery == "" || passNum == "") && session.Owner != nil && *session.Owner && session.Person != nil {
			passSery, passNum = extractPersonPassportFromFindSession(session.Person)
		}
		if passSery == "" || passNum == "" {
			passSery = extractNestedString(v, "data", "owner", "person", "passport", "series")
			passNum = extractNestedString(v, "data", "owner", "person", "passport", "number")
		}
		birthDate = extractNestedString(v, "data", "owner", "person", "birthdate")
		if birthDate == "" {
			birthDate = extractNestedString(session.Person, "data", "birthdate")
		}
	}
	// ФИО и даты из Find: сначала vehicle.owner.person, затем session.Person (как у других провайдеров)
	var firstName, lastName, middleName string
	firstName = extractNestedString(v, "data", "owner", "person", "first_name")
	if firstName == "" {
		firstName = extractNestedString(session.Person, "data", "first_name")
	}
	lastName = extractNestedString(v, "data", "owner", "person", "last_name")
	if lastName == "" {
		lastName = extractNestedString(session.Person, "data", "last_name")
	}
	middleName = extractNestedString(v, "data", "owner", "person", "middle_name")
	if middleName == "" {
		middleName = extractNestedString(session.Person, "data", "middle_name")
	}
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
		issueDate = extractNestedString(session.Person, "data", "passport", "issued_at")
	}
	if issueDate == "" {
		issueDate = "2022-01-01"
	}
	issueDate = strings.TrimSpace(strings.Split(issueDate, "T")[0])
	// Apex ожидает даты в формате YYYY-MM-DD (без времени)
	birthDate = toYYYYMMDD(birthDate)
	if birthDate == "" {
		birthDate = issueDate
	}
	ownerIssuedBy := "N"
	// Обогащаем владельца/заявителя из EuroAsia person API (ФИО, паспорт issued_by / issued_at, PINFL)
	if ownerType != "organization" && passSery != "" && passNum != "" && birthDate != "" {
		if personData, err := oc.euroasiaLookupPersonByPassport(birthDate, passSery, passNum); err == nil && personData != nil {
			if fn := extractNestedString(personData, "first_name"); fn != "" {
				firstName = fn
			}
			if ln := extractNestedString(personData, "last_name"); ln != "" {
				lastName = ln
			}
			if mn := extractNestedString(personData, "middle_name"); mn != "" {
				middleName = mn
			}
			if iss := extractNestedString(personData, "passport", "issued_at"); iss != "" {
				issueDate = toYYYYMMDD(iss)
			}
			if by := extractNestedString(personData, "passport", "issued_by"); by != "" {
				ownerIssuedBy = by
			}
			if p := extractNestedString(personData, "pinfls", "0"); p != "" {
				pinfl = p
			} else if p := extractNestedString(personData, "external_id"); p != "" {
				pinfl = p
			}
		}
	}

	// endDate от start_date + период; Apex ожидает дату окончания на 1 день раньше (все периоды)
	endDate := req.StartDate
	if periodID == 1 {
		endDate = addMonthsThenSubDays(req.StartDate, 12, 1)
	} else if periodID == 2 {
		endDate = addMonthsThenSubDays(req.StartDate, 6, 1)
	} else {
		// период 3: 20 дней — Apex ожидает start + 19 дней (на 1 день раньше)
		endDate = addDaysToDate(req.StartDate, 19)
	}

	contractTermID := "1"
	seasonalID := 0 // для годового (periodID=1) Apex требует пустой seasonalInsuranceId
	switch periodID {
	case 1:
		contractTermID = "1"
		seasonalID = 0 // Годовой — seasonalInsuranceId не передаём
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
	// Apex принимает только id из своего справочника /api/references/vehicle-types-osago (1=легковые, 6=грузовые, 9=автобусы, 15=трамваи/мото)
	apexVehicleTypeID := mapFindVehicleTypeToApex(vehicleTypeID)
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
				"issuedBy":   ownerIssuedBy,
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
	applicantIsOwner := true
	// Заявитель из тела запроса (applicant_passport_*, applicant_birthdate) — не владелец
	if aS := strings.TrimSpace(req.ApplicantPassportSeries); aS != "" {
		if aN := strings.TrimSpace(req.ApplicantPassportNumber); aN != "" {
			applicantBirthNorm := toYYYYMMDD(strings.TrimSpace(req.ApplicantBirthdate))
			if applicantBirthNorm == "" {
				applicantBirthNorm = "2000-01-01"
			}
			applicantPinfl := findPinflCreate(oc, session, aS, aN, applicantBirthNorm)
			applicantIssueDate := "2022-01-01"
			applicantIssuedBy := "N"
			afirst, alast, amiddle := "N", "N", "N"
			if personData, err := oc.euroasiaLookupPersonByPassport(applicantBirthNorm, aS, aN); err == nil && personData != nil {
				afirst = extractNestedString(personData, "first_name")
				alast = extractNestedString(personData, "last_name")
				amiddle = extractNestedString(personData, "middle_name")
				if afirst == "" {
					afirst = "N"
				}
				if alast == "" {
					alast = "N"
				}
				if amiddle == "" {
					amiddle = "N"
				}
				if iss := extractNestedString(personData, "passport", "issued_at"); iss != "" {
					applicantIssueDate = toYYYYMMDD(iss)
				}
				if by := extractNestedString(personData, "passport", "issued_by"); by != "" {
					applicantIssuedBy = by
				}
				if p := extractNestedString(personData, "pinfls", "0"); p != "" {
					applicantPinfl = p
				} else if p := extractNestedString(personData, "external_id"); p != "" {
					applicantPinfl = p
				}
			}
			if applicantPinfl == "" {
				applicantPinfl = "00000000000000"
			}
			applicant = map[string]interface{}{
				"person": map[string]interface{}{
					"passportData": map[string]interface{}{
						"pinfl":       applicantPinfl,
						"seria":      aS,
						"number":     aN,
						"issuedBy":   applicantIssuedBy,
						"issueDate":  applicantIssueDate,
					},
					"fullName": map[string]interface{}{
						"firstname":  afirst,
						"lastname":   alast,
						"middlename": amiddle,
					},
					"phoneNumber":   req.PhoneNumber,
					"gender":        "m",
					"birthDate":     applicantBirthNorm,
					"regionId":      10,
					"districtId":    1005,
				},
				"address":        "N",
				"residentOfUzb":  1,
				"citizenshipId":  210,
			}
			applicantIsOwner = false
		}
	}
	var owner map[string]interface{}
	if ownerType == "organization" {
		// Apex: нельзя передавать и Person, и Organization в owner; только organization с name (обязательно) и inn
		inn := extractNestedString(v, "data", "owner", "organization", "inn")
		if inn == "" {
			inn = extractNestedString(session.Organization, "data", "inn")
		}
		orgName := extractNestedString(v, "data", "owner", "organization", "name_short")
		if orgName == "" {
			orgName = extractNestedString(v, "data", "owner", "organization", "name")
		}
		if orgName == "" {
			orgName = extractNestedString(session.Organization, "data", "name")
		}
		if orgName == "" {
			orgName = "N"
		}
		owner = map[string]interface{}{
			"organization": map[string]interface{}{
				"inn":  inn,
				"name": orgName,
			},
			"applicantIsOwner": applicantIsOwner,
		}
	} else {
		owner = map[string]interface{}{
			"person": map[string]interface{}{
				"passportData": map[string]interface{}{
					"pinfl":      pinfl,
					"seria":     passSery,
					"number":    passNum,
					"issuedBy":  ownerIssuedBy,
					"issueDate": issueDate + "T00:00:00",
				},
				"fullName": map[string]interface{}{
					"firstname":  firstName,
					"lastname":   lastName,
					"middlename": middleName,
				},
			},
			"applicantIsOwner": applicantIsOwner,
		}
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

	var apexDriversList []map[string]interface{}
	if driverRestriction && len(driversList) > 0 {
		for _, d := range driversList {
			dpinfl := findPinflCreate(oc, session, d.PassportSeries, d.PassportNumber, d.Birthdate)
			if dpinfl == "" {
				dpinfl = pinfl
			}
			driverBirth := toYYYYMMDD(d.Birthdate)
			driverIssueDate := toYYYYMMDD(d.LicenseIssueDate)
			if driverIssueDate == "" {
				driverIssueDate = "2020-01-01"
			}
			dfirst, dlast, dmiddle := "N", "N", "N"
			dissuedBy := "N"
			// EuroAsia person API — подставляем реальные ФИО и дату выдачи паспорта
			if personData, err := oc.euroasiaLookupPersonByPassport(driverBirth, d.PassportSeries, d.PassportNumber); err == nil && personData != nil {
				dfirst = extractNestedString(personData, "first_name")
				dlast = extractNestedString(personData, "last_name")
				dmiddle = extractNestedString(personData, "middle_name")
				if dfirst == "" {
					dfirst = "N"
				}
				if dlast == "" {
					dlast = "N"
				}
				if dmiddle == "" {
					dmiddle = "N"
				}
				if iss := extractNestedString(personData, "passport", "issued_at"); iss != "" {
					driverIssueDate = toYYYYMMDD(iss)
				}
				if by := extractNestedString(personData, "passport", "issued_by"); by != "" {
					dissuedBy = by
				}
				if p := extractNestedString(personData, "pinfls", "0"); p != "" {
					dpinfl = p
				} else if p := extractNestedString(personData, "external_id"); p != "" {
					dpinfl = p
				}
			}
			apexDriversList = append(apexDriversList, map[string]interface{}{
				"passportData": map[string]interface{}{
					"pinfl":      dpinfl,
					"seria":     d.PassportSeries,
					"number":    d.PassportNumber,
					"issuedBy":  dissuedBy,
					"issueDate": driverIssueDate,
				},
				"fullName": map[string]interface{}{
					"firstname":  dfirst,
					"lastname":   dlast,
					"middlename": dmiddle,
				},
				"licenseNumber":     d.LicenseNumber,
				"licenseSeria":      d.LicenseSeries,
				"relative":          relativeToApex(d.Relative),
				"birthDate":         driverBirth,
				"licenseIssueDate":  driverIssueDate,
				"residentOfUzb":     1,
			})
		}
	} else {
		// Без ограничения водителей (unlimited): Apex запрещает отправлять drivers — только пустой массив
		apexDriversList = []map[string]interface{}{}
	}

	// Apex требует уникальный transactionId в каждом запросе (иначе "Ошибка в поле transactionid - уникальное значение")
	transactionID := time.Now().UnixNano() / 1000000
	body := map[string]interface{}{
		"applicant": applicant,
		"owner":     owner,
		"details": map[string]interface{}{
			"startDate":               req.StartDate,
			"issueDate":               req.StartDate,
			"endDate":                 endDate,
			"driverNumberRestriction": driverRestriction,
			"transactionId":           transactionID,
		},
		"cost": func() map[string]interface{} {
			cost := map[string]interface{}{
				"discountId":                    1,
				"discountSum":                   "0",
				"insurancePremium":              amountUZS,
				"sumInsured":                    "80000000",
				"contractTermConclusionId":     contractTermID,
				"useTerritoryId":                useTerritoryID,
				"commission":                    "0",
				"insurancePremiumPaidToInsurer": amountUZS,
			}
			// Для годового (ID=1) seasonalInsuranceId должен быть пустым
			if seasonalID != 0 {
				cost["seasonalInsuranceId"] = seasonalID
			}
			return cost
		}(),
		"vehicle": map[string]interface{}{
			"techPassport": map[string]interface{}{
				"seria":  techSery,
				"number": techNum,
			},
			"modelCustomName": modelName,
			"engineNumber":    engineNumber,
			"typeId":          apexVehicleTypeID,
			"issueYear":       issueYear,
			"govNumber":       gosNumber,
			"bodyNumber":      bodyNumber,
			"regionId":        10,
		},
		"drivers": apexDriversList,
	}

	userID := oc.cfg.ApexUserID
	if userID == 0 {
		userID = 30541
	}
	url := oc.cfg.ApexBaseURL + "/osago?user_id=" + strconv.Itoa(userID)
	return oc.makeProviderRequest("POST", url, body, oc.cfg.ApexLogin, oc.cfg.ApexPassword, "")
}

// createInson — оформление ОСАГО через Inson Insurance (двухшаговый процесс).
// Шаг 1: POST /api/v2/osago/contract → contractId
// Шаг 2: PUT  /api/v1/osago/contract/payment → policyUrl
// Auth: HTTP Basic. Для резидентов РУз достаточно паспортных данных — ФИО и PINFL система подтягивает сама.
func (oc *OsagoCreateController) createInson(session *SessionData, req *CreateRequest) (interface{}, error) {
	if req.PeriodID == 3 {
		return nil, fmt.Errorf("Inson не поддерживает период 20 дней")
	}
	if req.StartDate == "" {
		return nil, fmt.Errorf("start_date обязателен для Inson create")
	}
	if req.PhoneNumber == "" {
		return nil, fmt.Errorf("phone_number обязателен для Inson create")
	}

	// startDate должен быть сегодня или в будущем (Inson 422)
	startDate := toYYYYMMDD(req.StartDate)
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil && t.Before(time.Now().Truncate(24*time.Hour)) {
			startDate = time.Now().Format("2006-01-02")
		}
	} else {
		startDate = time.Now().Format("2006-01-02")
	}

	v := session.Vehicle
	ownerType := extractNestedString(v, "data", "owner", "type")

	// period: 1→12 мес, 2→6 мес
	period := 12
	if req.PeriodID == 2 {
		period = 6
	}
	// Если period_id не передан — берём из сессии (calculate_snapshot)
	if req.PeriodID == 0 {
		if snap := session.CalculateSnapshot; snap != nil && snap.PeriodID >= 1 && snap.PeriodID <= 2 {
			if snap.PeriodID == 2 {
				period = 6
			}
		}
	}

	gosNumber := extractNestedString(v, "data", "license_plate")
	techSeries := extractNestedString(v, "data", "tech_passport", "series")
	techNumber := extractNestedString(v, "data", "tech_passport", "number")
	if gosNumber == "" {
		return nil, fmt.Errorf("не найден госномер автомобиля")
	}

	// Drivers list: те же водители, что и в Calculate. Приоритет — список с большим числом водителей (из тела create или из snapshot после calculate).
	driverRestriction := req.DriverRestriction
	driversList := req.Drivers
	if snap := session.CalculateSnapshot; snap != nil {
		driverRestriction = snap.DriverRestriction
		if len(snap.Drivers) > 0 {
			if len(snap.Drivers) >= len(req.Drivers) {
				driversList = snap.Drivers
			}
			// иначе оставляем req.Drivers (в теле передали явно, например 2 водителя)
		}
	}
	driversList = oc.enrichDriversWithLicenseFromFind(driversList)

	// Строим applicant (страхователь)
	var applicant map[string]interface{}
	if ownerType == "organization" {
		inn := extractNestedString(v, "data", "owner", "organization", "inn")
		if inn == "" {
			inn = extractNestedString(session.Organization, "data", "inn")
		}
		orgName := extractNestedString(v, "data", "owner", "organization", "name")
		if orgName == "" {
			orgName = extractNestedString(session.Organization, "data", "name")
		}
		if orgName == "" {
			orgName = "N"
		}
		applicant = map[string]interface{}{
			"organization": map[string]interface{}{
				"inn":  inn,
				"name": orgName,
			},
			"residentType":  1,
			"citizenshipId": 1,
			"phoneNumber":   req.PhoneNumber,
			"isOwner":       true,
		}
	} else {
		// Физлицо: паспорт, PINFL, birthDate и телефон (Inson требует для applicant.person)
		ownerPersonObj := extractNestedValue(v, "data", "owner", "person")
		passSeries, passNumber := extractPersonPassportFromFindSession(ownerPersonObj)
		if passSeries == "" {
			passSeries = extractNestedString(v, "data", "owner", "person", "passport", "series")
			passNumber = extractNestedString(v, "data", "owner", "person", "passport", "number")
		}
		if passSeries == "" && session.Person != nil {
			passSeries, passNumber = extractPersonPassportFromFindSession(session.Person)
		}
		ownerBirth := extractNestedString(v, "data", "owner", "person", "birthdate")
		if ownerBirth == "" && session.Person != nil {
			ownerBirth = extractNestedString(session.Person, "data", "birthdate")
		}
		ownerBirth = toYYYYMMDD(ownerBirth)
		if ownerBirth == "" {
			ownerBirth = strings.TrimSpace(req.ApplicantBirthdate)
			ownerBirth = toYYYYMMDD(ownerBirth)
		}
		ownerPinfl := extractNestedString(v, "data", "owner", "person", "pinfls", "0")
		if ownerPinfl == "" {
			ownerPinfl = extractNestedString(v, "data", "owner", "person", "external_id")
		}
		ownerIssuedBy, ownerIssueDate := "ИИБ", "2020-01-01"
		ownerFirst, ownerLast, ownerMiddle := "N", "N", "N"
		ownerAddress := ""
		ownerRegionID, ownerDistrictID := 0, 0
		if ownerPinfl == "" && passSeries != "" && passNumber != "" && ownerBirth != "" {
			ownerPinfl, _ = oc.euroasiaLookupPinflByPassport(ownerBirth, passSeries, passNumber)
		}
		if passSeries != "" && passNumber != "" && ownerBirth != "" {
			if personData, err := oc.euroasiaLookupPersonByPassport(ownerBirth, passSeries, passNumber); err == nil && personData != nil {
				if by := extractNestedString(personData, "passport", "issued_by"); by != "" {
					ownerIssuedBy = by
				}
				if iss := extractNestedString(personData, "passport", "issued_at"); iss != "" {
					ownerIssueDate = toYYYYMMDD(iss)
				}
				if fn := extractNestedString(personData, "first_name"); fn != "" {
					ownerFirst = fn
				}
				if ln := extractNestedString(personData, "last_name"); ln != "" {
					ownerLast = ln
				}
				if mn := extractNestedString(personData, "middle_name"); mn != "" {
					ownerMiddle = mn
				}
				if addr := extractNestedString(personData, "address"); addr != "" {
					ownerAddress = addr
				}
				if rid := extractNestedInt(v, "data", "owner", "person", "region", "external_id"); rid > 0 {
					ownerRegionID = rid
				}
				if did := extractNestedInt(v, "data", "owner", "person", "district", "external_id"); did > 0 {
					ownerDistrictID = did
				}
			}
		}
		if ownerAddress == "" {
			ownerAddress = "N"
		}
		passportData := map[string]interface{}{
			"series":    passSeries,
			"number":    passNumber,
			"issuedBy":  ownerIssuedBy,
			"issueDate": ownerIssueDate,
		}
		if ownerPinfl != "" {
			passportData["pinfl"] = ownerPinfl
		}
		applicantPerson := map[string]interface{}{
			"passportData": passportData,
			"fullName": map[string]interface{}{
				"firstname":  ownerFirst,
				"lastname":   ownerLast,
				"middlename": ownerMiddle,
			},
			"gender": 1,
		}
		if ownerBirth != "" {
			applicantPerson["birthDate"] = ownerBirth
		}
		if ownerAddress != "" {
			applicantPerson["address"] = ownerAddress
		}
		applicantPerson["regionId"] = ownerRegionID
		applicantPerson["districtId"] = ownerDistrictID
		phoneNum := strings.TrimSpace(req.PhoneNumber)
		phoneNum = strings.TrimPrefix(phoneNum, "+")
		phoneNum = strings.ReplaceAll(phoneNum, " ", "")
		phoneNum = strings.ReplaceAll(phoneNum, "-", "")
		if phoneNum != "" && len(phoneNum) < 12 && !strings.HasPrefix(phoneNum, "998") {
			phoneNum = "998" + strings.TrimLeft(phoneNum, "8")
		}
		applicant = map[string]interface{}{
			"person":        applicantPerson,
			"residentType":  1,
			"citizenshipId": 1,
			"phoneNumber":   phoneNum,
			"isOwner":       true,
		}
	}

	// Обрабатываем drivers (только при ограниченном полисе). Inson требует: birthDate, passportData.pinfl, licenseSeries (2–5 символов), licenseNumber (6–10 символов).
	var insonDrivers []map[string]interface{}
	if driverRestriction && len(driversList) > 0 {
		for _, d := range driversList {
			birthNorm := toYYYYMMDD(d.Birthdate)
			if birthNorm == "" {
				return nil, fmt.Errorf("для Inson у каждого водителя обязательна дата рождения (birthdate); водитель: %s %s", d.PassportSeries, d.PassportNumber)
			}
			pinfl := findPinflCreate(oc, session, d.PassportSeries, d.PassportNumber, d.Birthdate)
			if pinfl == "" && birthNorm != "" {
				pinfl, _ = oc.euroasiaLookupPinflByPassport(birthNorm, d.PassportSeries, d.PassportNumber)
			}
			if pinfl == "" {
				return nil, fmt.Errorf("для Inson не удалось получить PINFL водителя (серия %s, номер %s); выполните find по человеку", d.PassportSeries, d.PassportNumber)
			}
			licSeries := strings.TrimSpace(d.LicenseSeries)
			licNumber := strings.TrimSpace(d.LicenseNumber)
			// Inson: licenseSeries 2–5 символов [A-Za-z0-9], licenseNumber 6–10 символов
			if len(licSeries) < 2 || len(licSeries) > 5 {
				licSeries = "AA"
			}
			if len(licNumber) < 6 || len(licNumber) > 10 {
				licNumber = "000000"
			}
			licenseIssueDate := toYYYYMMDD(d.LicenseIssueDate)
			dIssuedBy, dIssueDate := "Toshkent shahar IIB", "2020-01-01"
			dFirst, dLast, dMiddle := "N", "N", "N"
			if personData, err := oc.euroasiaLookupPersonByPassport(birthNorm, d.PassportSeries, d.PassportNumber); err == nil && personData != nil {
				if by := extractNestedString(personData, "passport", "issued_by"); by != "" {
					dIssuedBy = by
				}
				if iss := extractNestedString(personData, "passport", "issued_at"); iss != "" {
					dIssueDate = toYYYYMMDD(iss)
				}
				if fn := extractNestedString(personData, "first_name"); fn != "" {
					dFirst = fn
				}
				if ln := extractNestedString(personData, "last_name"); ln != "" {
					dLast = ln
				}
				if mn := extractNestedString(personData, "middle_name"); mn != "" {
					dMiddle = mn
				}
			}
			driverObj := map[string]interface{}{
				"residentType": 1,
				"birthDate":    birthNorm,
				"relative":     d.Relative,
				"passportData": map[string]interface{}{
					"pinfl":     pinfl,
					"series":    d.PassportSeries,
					"number":    d.PassportNumber,
					"issuedBy":  dIssuedBy,
					"issueDate": dIssueDate,
				},
				"fullName": map[string]interface{}{
					"firstname":  dFirst,
					"lastname":   dLast,
					"middlename": dMiddle,
				},
				"licenseSeries":    licSeries,
				"licenseNumber":    licNumber,
				"licenseIssueDate": licenseIssueDate,
			}
			if licenseIssueDate == "" {
				driverObj["licenseIssueDate"] = "2020-01-01"
			}
			insonDrivers = append(insonDrivers, driverObj)
		}
	}
	if insonDrivers == nil {
		insonDrivers = []map[string]interface{}{}
	}

	// Рабочая форма Inson: applicant, details, vehicle (techPassport только series+number), drivers; owner не передаём
	contractBody := map[string]interface{}{
		"applicant": applicant,
		"details": map[string]interface{}{
			"startDate":              startDate,
			"period":                 period,
			"driverNumberRestricted": driverRestriction,
		},
		"vehicle": map[string]interface{}{
			"governmentNumber": gosNumber,
			"techPassport": map[string]interface{}{
				"series": techSeries,
				"number": techNumber,
			},
		},
		"drivers": insonDrivers,
	}

	contractURL := oc.cfg.InsonBaseURL + "/api/v2/osago/contract"
	contractResp, err := oc.makeProviderRequest("POST", contractURL, contractBody, oc.cfg.InsonLogin, oc.cfg.InsonPassword, "")
	if err != nil {
		return nil, fmt.Errorf("Inson contract create failed: %w", err)
	}

	// Извлекаем contractId из ответа {"data": {"contractId": 123456, ...}}
	contractIDRaw := extractNestedValue(contractResp, "data", "contractId")
	if contractIDRaw == nil {
		// contractId может быть на верхнем уровне
		contractIDRaw = contractResp["contractId"]
	}
	if contractIDRaw == nil {
		// Контракт создан, но ID не получен — возвращаем ответ как есть
		return contractResp, nil
	}

	var contractIDInt int
	switch cid := contractIDRaw.(type) {
	case float64:
		contractIDInt = int(cid)
	case int:
		contractIDInt = cid
	}

	// Шаг 2: финализируем (выпускаем) полис
	paymentBody := map[string]interface{}{
		"contractId": contractIDInt,
	}
	paymentURL := oc.cfg.InsonBaseURL + "/api/v1/osago/contract/payment"
	paymentResp, err := oc.makeProviderRequest("PUT", paymentURL, paymentBody, oc.cfg.InsonLogin, oc.cfg.InsonPassword, "")
	if err != nil {
		// Если финализация не удалась — возвращаем то что есть с contractId
		log.Printf("[Inson create] payment step failed: %v", err)
		return map[string]interface{}{
			"contract": contractResp,
			"payment":  nil,
			"error":    "payment step failed: " + err.Error(),
		}, nil
	}

	return map[string]interface{}{
		"contract": contractResp,
		"payment":  paymentResp,
	}, nil
}

// mapFindVehicleTypeToApex переводит external_id типа ТС из Find/EuroAsia в id справочника Apex (vehicle-types-osago).
// Apex: 1=Легковые, 6=Грузовые, 9=Автобусы, 15=Трамваи/мото. Find: 2=Легковые, 6=Грузовые и т.д.
func mapFindVehicleTypeToApex(findExternalID int) int {
	switch findExternalID {
	case 2:
		return 1 // Легковые автомобили
	case 6:
		return 6 // Грузовые
	case 9:
		return 9 // Автобусы
	case 15:
		return 15 // Трамваи, мотоциклы и т.д.
	default:
		return 1 // по умолчанию легковые
	}
}

// mapFindVehicleTypeToTrust — то же для Trust (те же id: 1,6,9,15).
func mapFindVehicleTypeToTrust(findExternalID int) int {
	return mapFindVehicleTypeToApex(findExternalID)
}

// mapFindDistrictToTrust переводит район из Find (region external_id + district external_id) в id справочника Trust.
// У Trust для региона 10 (г.Ташкент): районы 1001–1011 и 2404 (Алмазарский). Find даёт Алмазарский как 1012.
func mapFindDistrictToTrust(regionExt, districtExt int) int {
	if regionExt == 10 && districtExt == 1012 {
		return 2404 // Алмазарский район
	}
	if districtExt != 0 {
		return districtExt
	}
	return 1001
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

// addMonthsThenSubDays — для Apex годовой период: endDate = start + 12 мес - 1 день
func addMonthsThenSubDays(ymd string, months, subDays int) string {
	ymd = toYYYYMMDD(ymd)
	t, err := time.Parse("2006-01-02", ymd)
	if err != nil {
		return ymd
	}
	t = t.AddDate(0, months, 0)
	t = t.AddDate(0, 0, -subDays)
	return t.Format("2006-01-02")
}

func addDaysToDate(ymd string, days int) string {
	ymd = toYYYYMMDD(ymd)
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

