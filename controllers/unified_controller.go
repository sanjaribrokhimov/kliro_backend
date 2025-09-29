package controllers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"kliro/config"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type DriverStored struct {
	DriverPinfl     string `json:"driver_pinfl"`
	DriverPSeries   string `json:"driver_p_series"`
	DriverPNumber   string `json:"driver_p_number"`
	LastNameLatin   string `json:"last_name_latin"`
	FirstNameLatin  string `json:"first_name_latin"`
	MiddleNameLatin string `json:"middle_name_latin"`
	BirthDate       string `json:"birth_date"`
	Oblast          string `json:"oblast"`
	Rayon           string `json:"rayon"`
	LicenseNumber   string `json:"license_number"`
	LicenseSeries   string `json:"license_series"`
	LicenseDate     string `json:"license_date"`
}

type ApplicantStored struct {
	ApplPinfl       string `json:"appl_pinfl"`
	ApplPSeries     string `json:"appl_p_series"`
	ApplPNumber     string `json:"appl_p_number"`
	ApplPhone       string `json:"appl_phone"`
	LastNameLatin   string `json:"last_name_latin"`
	FirstNameLatin  string `json:"first_name_latin"`
	MiddleNameLatin string `json:"middle_name_latin"`
	BirthDate       string `json:"birth_date"`
	Oblast          string `json:"oblast"`
	Rayon           string `json:"rayon"`
}

type VehicleData struct {
	Error                 int                        `json:"error"`
	ErrorMessage          string                     `json:"error_message"`
	TechPassportIssueDate string                     `json:"tech_passport_issue_date"`
	IssueYear             string                     `json:"issue_year"`
	VehicleTypeID         string                     `json:"vehicle_type_id"`
	BodyNumber            string                     `json:"body_number"`
	EngineNumber          string                     `json:"engine_number"`
	ModelID               string                     `json:"model_id"`
	MarkaID               string                     `json:"marka_id"`
	ModelName             string                     `json:"model_name"`
	OrgName               string                     `json:"orgname"`
	LastName              string                     `json:"last_name"`
	FirstName             string                     `json:"first_name"`
	MiddleName            string                     `json:"middle_name"`
	UseTerritory          int                        `json:"use_territory"`
	Fy                    int                        `json:"fy"`
	Pinfl                 string                     `json:"pinfl"`
	Inn                   *string                    `json:"inn"`
	Seats                 string                     `json:"seats"`
	GovNumber             string                     `json:"govNumber"`
	TechPassportNumber    string                     `json:"techPassportNumber"`
	TechPassportSeria     string                     `json:"techPassportSeria"`
	OwnerPNumber          string                     `json:"ownerPNumber"`
	OwnerPSeries          string                     `json:"ownerPSeries"`
	OwnerPinfl            string                     `json:"ownerPinfl"`
	OwnerLastNameLatin    string                     `json:"owner_last_name_latin"`
	OwnerFirstNameLatin   string                     `json:"owner_first_name_latin"`
	OwnerMiddleNameLatin  string                     `json:"owner_middle_name_latin"`
	OwnerBirthDate        string                     `json:"owner_birth_date"`
	OwnerOblast           string                     `json:"owner_oblast"`
	OwnerRayon            string                     `json:"owner_rayon"`
	IsOwner               bool                       `json:"isOwner"`
	SummaStrahovki        int64                      `json:"summaStrahovki"`
	Provider              string                     `json:"provider"`
	Drivers               []DriverStored             `json:"drivers"`
	Applicant             ApplicantStored            `json:"applicant"`
	StrahovkaMonth        int                        `json:"strahovka_month"`
	IsDrivers             int                        `json:"is_drivers"`
	CarType               int                        `json:"car_type"`
	ContractBegin         string                     `json:"contract_begin"`
	NeoOrderID            *int64                     `json:"neo_order_id"`
	NeoContractID         *int64                     `json:"neo_contract_id"`
	NeoPayURL             *string                    `json:"neo_pay_url"`
	ProviderUUID          *string                    `json:"provider_uuid"`
	ProviderContractID    *int                       `json:"provider_contract_id"`
	ProviderResponseRaw   *string                    `json:"provider_response_raw"`
	PaymentCheckResponse  *TrustPaymentCheckResponse `json:"payment_check_response"`
	PayURLs               *struct {
		Click string `json:"click"`
		Payme string `json:"payme"`
	} `json:"pay_urls"`
}

type UnifiedController struct {
	config            *config.Config
	client            *http.Client
	token             string
	tokenExpiry       time.Time
	mutex             sync.RWMutex
	merchantInfoCache *MerchantInfoCache
	merchantMutex     sync.RWMutex
}

var (
	store = struct {
		sync.RWMutex
		M map[string]*VehicleData
	}{M: make(map[string]*VehicleData)}

	neoDebugInfo string

	periodMapping = []Mapping{
		{OurID: 12, NeoID: 1, TrustID: 2},
		{OurID: 6, NeoID: 2, TrustID: 1},
	}

	driversMapping = []Mapping{
		{OurID: 0, NeoID: 1, TrustID: 0},
		{OurID: 5, NeoID: 4, TrustID: 1},
	}

	carTypeMapping = []Mapping{
		{OurID: 1, NeoID: 1, TrustID: 1},
		{OurID: 2, NeoID: 2, TrustID: 6},
		{OurID: 3, NeoID: 3, TrustID: 9},
		{OurID: 4, NeoID: 4, TrustID: 15},
	}
)

func NewUnifiedController(cfg *config.Config) *UnifiedController {
	return &UnifiedController{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func getMappingID(slice []Mapping, ourID int) (neoID int, trustID int, ok bool) {
	for _, m := range slice {
		if m.OurID == ourID {
			return m.NeoID, m.TrustID, true
		}
	}
	return 0, 0, false
}

func (c *UnifiedController) makeHTTPRequest(method, url string, body interface{}) (*http.Response, error) {
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

	// Добавляем токен если он есть
	c.mutex.RLock()
	token := c.token
	c.mutex.RUnlock()

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return c.client.Do(req)
}

func (c *UnifiedController) parseResponse(resp *http.Response, result interface{}) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return json.Unmarshal(body, result)
}

func (c *UnifiedController) callPassportPinflAPI(request PassportPinflRequest) (*PassportPinflResponse, error) {
	token, err := c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
	if err != nil {
		fmt.Printf("Failed to get token for passport-pinfl: %v\n", err)
		return nil, err
	}

	fmt.Printf("Making passport-pinfl request with token: %s\n", token[:10]+"...")
	fmt.Printf("Request data: %+v\n", request)

	resp, err := c.makeHTTPRequest("POST", c.config.TrustBaseURL+"/api/provider/passport-pinfl", request)
	if err != nil {
		fmt.Printf("HTTP request failed: %v\n", err)
		return nil, err
	}

	fmt.Printf("Response status: %d\n", resp.StatusCode)

	var response PassportPinflResponse
	if err := c.parseResponse(resp, &response); err != nil {
		fmt.Printf("Parse response failed: %v\n", err)
		return nil, err
	}

	if response.Error != 0 {
		fmt.Printf("API returned error: %d - %s\n", response.Error, response.ErrorMessage)
		return nil, fmt.Errorf("API error: %s", response.ErrorMessage)
	}

	return &response, nil
}

func (c *UnifiedController) callDriverLicenseAPI(request DriverLicenseRequest) (*DriverLicenseResponse, error) {
	_, err := c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
	if err != nil {
		return nil, err
	}

	resp, err := c.makeHTTPRequest("POST", c.config.TrustBaseURL+"/api/provider/driver-license", request)
	if err != nil {
		return nil, err
	}

	var response DriverLicenseResponse
	if err := c.parseResponse(resp, &response); err != nil {
		return nil, err
	}

	if response.Error != 0 {
		return nil, fmt.Errorf("API error: %s", response.ErrorMessage)
	}

	return &response, nil
}

func (c *UnifiedController) callNeoSaveAPI(request NeoSaveRequest) (*NeoSaveResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal neo save request: %v", err)
	}

	url := "http://localhost:8080/neoInsurance/osago/save-policy"
	fmt.Printf("Calling internal Neo API: %s\n", url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	fmt.Printf("Internal Neo API Response - Status: %d, Body: %s\n", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("neo API returned status %d: %s", resp.StatusCode, string(body))
	}

	var neoSaveResp NeoSaveResponse
	if err := json.Unmarshal(body, &neoSaveResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &neoSaveResp, nil
}

func (c *UnifiedController) callTrustCreateAPI(request TrustCreateRequest) (*TrustCreateResponse, error) {
	fmt.Printf("=== UNIFIED API -> INTERNAL TRUST API ===\n")

	jsonData, err := json.Marshal(request)
	if err != nil {
		fmt.Printf("ERROR: Failed to marshal trust create request: %v\n", err)
		return nil, fmt.Errorf("failed to marshal trust create request: %v", err)
	}

	url := "http://localhost:8080/trustInsurance/osago/create"
	fmt.Printf("URL: %s\n", url)
	fmt.Printf("Request struct: %+v\n", request)
	fmt.Printf("JSON data: %s\n", string(jsonData))
	fmt.Printf("JSON size: %d bytes\n", len(jsonData))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("ERROR: Failed to create HTTP request: %v\n", err)
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	fmt.Printf("Headers: %v\n", req.Header)

	fmt.Printf("Sending request to internal Trust API...\n")
	resp, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("ERROR: HTTP request failed: %v\n", err)
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %d\n", resp.StatusCode)
	fmt.Printf("Response headers: %v\n", resp.Header)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("ERROR: Failed to read response body: %v\n", err)
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	fmt.Printf("Response body: %s\n", string(body))
	fmt.Printf("Response body length: %d bytes\n", len(body))

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("ERROR: Internal Trust API returned non-OK status: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("trust API returned status %d: %s", resp.StatusCode, string(body))
	}

	var trustCreateResp TrustCreateResponse
	if err := json.Unmarshal(body, &trustCreateResp); err != nil {
		fmt.Printf("ERROR: Failed to unmarshal response: %v\n", err)
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	fmt.Printf("Successfully parsed response: %+v\n", trustCreateResp)
	return &trustCreateResp, nil
}

func (c *UnifiedController) callCheckPersonAPI(request CheckPersonRequest) (*CheckPersonResponse, error) {
	fmt.Printf("=== CHECK PERSON API ===\n")

	jsonData, err := json.Marshal(request)
	if err != nil {
		fmt.Printf("ERROR: Failed to marshal check person request: %v\n", err)
		return nil, fmt.Errorf("failed to marshal check person request: %v", err)
	}

	url := "http://localhost:8080/neoInsurance/osago/check-person"
	fmt.Printf("URL: %s\n", url)
	fmt.Printf("Request: %+v\n", request)
	fmt.Printf("JSON: %s\n", string(jsonData))

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("ERROR: Failed to create HTTP request: %v\n", err)
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("ERROR: HTTP request failed: %v\n", err)
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("ERROR: Failed to read response: %v\n", err)
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	fmt.Printf("Response status: %d\n", resp.StatusCode)
	fmt.Printf("Response body: %s\n", string(body))

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("ERROR: Check person API returned non-OK status: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("check person API returned status %d: %s", resp.StatusCode, string(body))
	}

	var checkPersonResp CheckPersonResponse
	if err := json.Unmarshal(body, &checkPersonResp); err != nil {
		fmt.Printf("ERROR: Failed to unmarshal response: %v\n", err)
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	fmt.Printf("Check person result: error=%d, result=%t, message=%s\n",
		checkPersonResp.Error, checkPersonResp.Result, checkPersonResp.Message)

	return &checkPersonResp, nil
}

type NacaloRequest struct {
	GovNumber          string `json:"govNumber"`
	TechPassportNumber string `json:"techPassportNumber"`
	TechPassportSeria  string `json:"techPassportSeria"`
	OwnerPNumber       string `json:"ownerPNumber"`
	OwnerPSeries       string `json:"ownerPSeries"`
}

type NacaloResponse struct {
	OwnerPinfl         string `json:"ownerPinfl"`
	OwnerName          string `json:"ownerName"`
	OwnerFamiliya      string `json:"ownerFamiliya"`
	OwnerOtchestvo     string `json:"ownerOtchestvo"`
	CarModel           string `json:"carModel"`
	CarYear            string `json:"carYear"`
	GovNumber          string `json:"govNumber"`
	TechPassportNumber string `json:"techPassportNumber"`
	TechPassportSeria  string `json:"techPassportSeria"`
	SessionID          string `json:"sessionId"`
	IsOwner            bool   `json:"isOwner"`
}

type TrustVehicleRequest struct {
	GovNumber          string `json:"govnumber"`
	TechPassportNumber string `json:"techPassportNumber"`
	TechPassportSeria  string `json:"techPassportSeria"`
}

type TrustVehicleResponse struct {
	Error                 int    `json:"error"`
	ErrorMessage          string `json:"error_message"`
	TechPassportIssueDate string `json:"tech_passport_issue_date"`
	IssueYear             string `json:"issue_year"`
	VehicleTypeID         string `json:"vehicle_type_id"`
	BodyNumber            string `json:"body_number"`
	EngineNumber          string `json:"engine_number"`
	ModelID               string `json:"model_id"`
	MarkaID               string `json:"marka_id"`
	ModelName             string `json:"model_name"`
	OrgName               string `json:"orgname"`
	LastName              string `json:"last_name"`
	FirstName             string `json:"first_name"`
	MiddleName            string `json:"middle_name"`
	UseTerritory          int    `json:"use_territory"`
	Fy                    int    `json:"fy"`
	Pinfl                 string `json:"pinfl"`
	Inn                   string `json:"inn"`
	Seats                 string `json:"seats"`
}

type NeoInsuranceRequest struct {
	GosNumber       string `json:"gos_number"`
	TechSery        string `json:"tech_sery"`
	TechNumber      string `json:"tech_number"`
	OwnerPassSeria  string `json:"owner__pass_seria"`
	OwnerPassNumber string `json:"owner__pass_number"`
}

type NeoInsuranceResponse struct {
	Error   int    `json:"error"`
	Result  bool   `json:"result"`
	Message string `json:"message"`
}

type Mapping struct {
	OurID   int
	NeoID   int
	TrustID int
}

type InitConRequest struct {
	UUID          string          `json:"uuid" binding:"required"`
	Drivers       []DriverRequest `json:"drivers"`
	ApplPinfl     string          `json:"applPinfl" binding:"required"`
	ApplPSeries   string          `json:"applPSeries" binding:"required"`
	ApplPNumber   string          `json:"applPNumber" binding:"required"`
	ApplPhone     string          `json:"applPhone" binding:"required"`
	ContractBegin string          `json:"contractBegin"`
}

type DriverRequest struct {
	DriverPinfl   string `json:"driverPinfl" binding:"required"`
	DriverPSeries string `json:"driverPSeries" binding:"required"`
	DriverPNumber string `json:"driverPNumber" binding:"required"`
}

type CalcRequest struct {
	StrahovkaMonth int    `json:"strahovkaMonth" binding:"required"`
	IsDrivers      int    `json:"isDrivers"`
	UUID           string `json:"uuid" binding:"required"`
	Provider       string `json:"provider" binding:"required"`
	ContractBegin  string `json:"contractBegin"`
}

type SubmitRequest struct {
	UUID string `json:"uuid" binding:"required"`
}

type CalcResponse struct {
	SessionID      string `json:"sessionId"`
	SummaStrahovki int    `json:"summaStrahovki"`
}

type NeoCalcRequest struct {
	GosNumber       string `json:"gos_number"`
	TechSery        string `json:"tech_sery"`
	TechNumber      string `json:"tech_number"`
	PeriodID        int    `json:"period_id"`
	NumberDriversID int    `json:"number_drivers_id"`
	CarTypeID       int    `json:"car_type_id"`
}

type NeoCalcResponse struct {
	Error    int    `json:"error"`
	Result   bool   `json:"result"`
	Message  string `json:"message"`
	Response struct {
		AmountUZS int  `json:"amount_uzs"`
		Inn       bool `json:"inn"`
	} `json:"response"`
}

type TrustCalcRequest struct {
	Discount     int `json:"discount"`
	DriverLimit  int `json:"driver_limit"`
	Period       int `json:"period"`
	UseTerritory int `json:"use_territory"`
	Vehicle      int `json:"vehicle"`
	CarType      int `json:"car_type"`
}

type TrustCalcResponse struct {
	Prem float64 `json:"prem"`
}

type PassportPinflRequest struct {
	Pinfl          string `json:"pinfl"`
	PassportSeries string `json:"passportSeries"`
	PassportNumber string `json:"passportNumber"`
}

type PassportPinflResponse struct {
	Error           int    `json:"error"`
	ErrorMessage    string `json:"error_message"`
	LastNameLatin   string `json:"last_name_latin"`
	FirstNameLatin  string `json:"first_name_latin"`
	MiddleNameLatin string `json:"middle_name_latin"`
	BirthDate       string `json:"birth_date"`
	Oblast          string `json:"oblast"`
	Rayon           string `json:"rayon"`
	IsPensioner     string `json:"ispensioner"`
	Address         string `json:"address"`
}

type DriverLicenseRequest struct {
	Pinfl          string `json:"pinfl"`
	PassportSeries string `json:"passportSeries"`
	PassportNumber string `json:"passportNumber"`
}

type DriverLicenseResponse struct {
	Error         int    `json:"error"`
	ErrorMessage  string `json:"error_message"`
	LicenseNumber string `json:"license_number"`
	LicenseSeries string `json:"license_seria"`
	LicenseDate   string `json:"issue_date"`
}

type NeoSaveRequest struct {
	AmountUZS         int64           `json:"amount_uzs"`
	GosNumber         string          `json:"gos_number"`
	TechNumber        string          `json:"tech_number"`
	TechSery          string          `json:"tech_sery"`
	Drivers           []NeoSaveDriver `json:"drivers"`
	ApplicantIsDriver bool            `json:"applicant_is_driver"`
	PeriodID          int             `json:"period_id"`
	NumberDriversID   string          `json:"number_drivers_id"`
	PhoneNumber       string          `json:"phone_number"`
	OwnerPassSeria    string          `json:"owner__pass_seria"`
	OwnerPassNumber   string          `json:"owner__pass_number"`
	OwnerPinfl        string          `json:"owner_pinfl"`
}

type NeoSaveDriver struct {
	PassportSeria  string `json:"passport__seria"`
	PassportNumber string `json:"passport__number"`
	DriverBirthday string `json:"driver_birthday"`
	Relative       int    `json:"relative"`
	Name           string `json:"name"`
}

type NeoSaveResponse struct {
	Result   bool   `json:"result"`
	Message  string `json:"message"`
	Response struct {
		AmountUZS  int64  `json:"amount_uzs"`
		OrderID    int64  `json:"order_id"`
		ContractID int64  `json:"contract_id"`
		URL        string `json:"url"`
		PaymeURL   string `json:"payme_url"`
	} `json:"response"`
}

type TrustCreateRequest struct {
	ApplicantIsowner int                 `json:"applicant_isowner"`
	ContractBegin    string              `json:"contract_begin"`
	DriverLimit      int                 `json:"driver_limit"`
	Dvigatel         string              `json:"dvigatel"`
	HasBenefit       int                 `json:"has_benefit"`
	Kuzov            string              `json:"kuzov"`
	OwnerFy          int                 `json:"owner_fy"`
	OwnerIsdriver    int                 `json:"owner_isdriver"`
	OwnerPhone       string              `json:"owner_phone"`
	Period           int                 `json:"period"`
	Renumber         string              `json:"renumber"`
	TexpDate         string              `json:"texpdate"`
	TexpNumber       string              `json:"texpnumber"`
	TexpSery         string              `json:"texpsery"`
	Type             int                 `json:"type"`
	UseTerritory     int                 `json:"use_territory"`
	Vmodel           string              `json:"vmodel"`
	Year             int                 `json:"year"`
	OwnerPinfl       string              `json:"owner_pinfl"`
	OwnerBirthdate   string              `json:"owner_birthdate"`
	OwnerPaspSery    string              `json:"owner_pasp_sery"`
	OwnerPaspNum     string              `json:"owner_pasp_num"`
	OwnerSurname     string              `json:"owner_surname"`
	OwnerName        string              `json:"owner_name"`
	OwnerPatronym    string              `json:"owner_patronym"`
	OwnerOblast      int                 `json:"owner_oblast"`
	OwnerRayon       int                 `json:"owner_rayon"`
	OwnerIspensioner int                 `json:"owner_ispensioner"`
	ApplFizyur       int                 `json:"appl_fizyur"`
	ApplPinfl        string              `json:"appl_pinfl"`
	ApplBirthdate    string              `json:"appl_birthdate"`
	ApplPaspSery     string              `json:"appl_pasp_sery"`
	ApplPaspNum      string              `json:"appl_pasp_num"`
	ApplSurname      string              `json:"appl_surname"`
	ApplName         string              `json:"appl_name"`
	ApplPatronym     string              `json:"appl_patronym"`
	ApplOblast       int                 `json:"appl_oblast"`
	ApplRayon        int                 `json:"appl_rayon"`
	Drivers          []TrustCreateDriver `json:"drivers"`
}

type TrustCreateDriver struct {
	DateBirth  string `json:"datebirth"`
	Paspsery   string `json:"paspsery"`
	Paspnumber string `json:"paspnumber"`
	Pinfl      string `json:"pinfl"`
	Surname    string `json:"surname"`
	Name       string `json:"name"`
	Patronym   string `json:"patronym"`
	Licnumber  string `json:"licnumber"`
	Licsery    string `json:"licsery"`
	Licdate    string `json:"licdate"`
	Relative   int    `json:"relative"`
	Resident   int    `json:"resident"`
}

type TrustCreateResponse struct {
	Error            int    `json:"error"`
	ErrorMessage     string `json:"error_message"`
	InsurancePremium string `json:"insurance_premium"`
	UUID             string `json:"uuid"`
	AnketaID         int    `json:"anketa_id"`
}

type TrustPaymentCheckRequest struct {
	AnketaID int    `json:"anketa_id"`
	Lan      string `json:"lan"`
}

type TrustPaymentCheckResponse struct {
	Error   int    `json:"error"`
	Message string `json:"message"`
	Result  bool   `json:"result"`
}

type MerchantInfo struct {
	Click struct {
		ServiceID  string `json:"service_id"`
		MerchantID string `json:"merchant_id"`
	} `json:"click"`
	Payme struct {
		MerchantID string `json:"merchant_id"`
	} `json:"payme"`
}

type MerchantInfoCache struct {
	Data      *MerchantInfo
	ExpiresAt time.Time
}

type CheckPersonRequest struct {
	GosNumber       string `json:"gos_number"`
	TechSery        string `json:"tech_sery"`
	TechNumber      string `json:"tech_number"`
	OwnerPassSeria  string `json:"owner__pass_seria"`
	OwnerPassNumber string `json:"owner__pass_number"`
}

type CheckPersonResponse struct {
	Error    int         `json:"error"`
	Result   bool        `json:"result"`
	Message  string      `json:"message"`
	Response interface{} `json:"response"`
}

func (c *UnifiedController) getValidToken(login, password string) (string, error) {
	c.mutex.RLock()
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		token := c.token
		c.mutex.RUnlock()
		return token, nil
	}
	c.mutex.RUnlock()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		return c.token, nil
	}

	token, err := c.authenticate(login, password)
	if err != nil {
		return "", err
	}

	c.token = token
	c.tokenExpiry = time.Now().Add(55 * time.Minute)
	return token, nil
}

func (c *UnifiedController) authenticate(login, password string) (string, error) {
	authData := map[string]string{
		"login":    login,
		"password": password,
	}

	jsonData, err := json.Marshal(authData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth data: %v", err)
	}

	// Определяем URL для аутентификации на основе логина
	var authURL string
	if login == c.config.TrustLogin {
		authURL = c.config.TrustBaseURL + "/api/products/auth/login"
		fmt.Printf("Using Trust auth URL: %s\n", authURL)
	} else if login == c.config.NeoLogin {
		authURL = c.config.NeoBaseURL + "/api/products/auth/login"
		fmt.Printf("Using Neo auth URL: %s\n", authURL)
	} else {
		authURL = c.config.TrustBaseURL + "/api/products/auth/login" // default
		fmt.Printf("Using default Trust auth URL: %s\n", authURL)
	}

	req, err := http.NewRequest("POST", authURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make auth request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth failed with status: %d", resp.StatusCode)
	}

	var authResponse struct {
		Result        int    `json:"result"`
		ResultMessage string `json:"result_message"`
		Roles         []struct {
			ID       int    `json:"id"`
			RoleName string `json:"roleName"`
		} `json:"roles"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read auth response: %v", err)
	}

	if err := json.Unmarshal(body, &authResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal auth response: %v", err)
	}

	if authResponse.Result == 0 && authResponse.ResultMessage != "" {
		token := authResponse.ResultMessage
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		return token, nil
	}

	return "", fmt.Errorf("no token found in auth response")
}

func (c *UnifiedController) callNeoInsuranceAPI(request NeoInsuranceRequest) (*NeoInsuranceResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal neo insurance request: %v", err)
	}

	req, err := http.NewRequest("POST", c.config.NeoBaseURL+"/api/osago-neo/osago-juridik", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create neo insurance request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	creds := c.config.NeoLogin + ":" + c.config.NeoPassword
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
	req.Header.Set("Authorization", auth)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make neo insurance request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read neo insurance response: %v", err)
	}

	fmt.Printf("NeoInsurance API response status: %d\n", resp.StatusCode)
	fmt.Printf("NeoInsurance API response body: %s\n", string(body))

	var neoResponse NeoInsuranceResponse
	if err := json.Unmarshal(body, &neoResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal neo insurance response: %v, body: %s", err, string(body))
	}

	return &neoResponse, nil
}

func findMapping(mappings []Mapping, ourID int, provider string) (int, error) {
	for _, mapping := range mappings {
		if mapping.OurID == ourID {
			if provider == "neo" {
				return mapping.NeoID, nil
			} else if provider == "trust" {
				return mapping.TrustID, nil
			}
		}
	}
	return 0, fmt.Errorf("mapping not found for ourID: %d, provider: %s", ourID, provider)
}

func (c *UnifiedController) callNeoCalcAPI(request NeoCalcRequest) (*NeoCalcResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal neo calc request: %v", err)
	}

	req, err := http.NewRequest("POST", c.config.NeoBaseURL+"/api/osago-neo/get-calc-osago", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create neo calc request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	creds := c.config.NeoLogin + ":" + c.config.NeoPassword
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
	req.Header.Set("Authorization", auth)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make neo calc request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read neo calc response: %v", err)
	}

	fmt.Printf("NeoInsurance Calc API response status: %d\n", resp.StatusCode)
	fmt.Printf("NeoInsurance Calc API response body: %s\n", string(body))

	var neoResponse NeoCalcResponse
	if err := json.Unmarshal(body, &neoResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal neo calc response: %v, body: %s", err, string(body))
	}

	return &neoResponse, nil
}

func (c *UnifiedController) callTrustCalcAPI(request TrustCalcRequest) (*TrustCalcResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trust calc request: %v", err)
	}

	token, err := c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to get trust token: %v", err)
	}

	req, err := http.NewRequest("POST", c.config.TrustBaseURL+"/api/osgo/calc-prem", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create trust calc request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make trust calc request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read trust calc response: %v", err)
	}

	fmt.Printf("TrustInsurance Calc API response status: %d\n", resp.StatusCode)
	fmt.Printf("TrustInsurance Calc API response body: %s\n", string(body))

	var trustResponse TrustCalcResponse
	if err := json.Unmarshal(body, &trustResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trust calc response: %v, body: %s", err, string(body))
	}

	return &trustResponse, nil
}

func (c *UnifiedController) Nacalo(ctx *gin.Context) {
	var request NacaloRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	token, err := c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to get auth token", "details": err.Error()})
		return
	}

	trustRequest := TrustVehicleRequest{
		GovNumber:          request.GovNumber,
		TechPassportNumber: request.TechPassportNumber,
		TechPassportSeria:  request.TechPassportSeria,
	}

	jsonData, err := json.Marshal(trustRequest)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal request"})
		return
	}

	req, err := http.NewRequest("POST", c.config.TrustBaseURL+"/api/provider/vehicle", bytes.NewBuffer(jsonData))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make request"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	fmt.Printf("Trust API Response: %s\n", string(body))

	var trustResponse TrustVehicleResponse
	if err := json.Unmarshal(body, &trustResponse); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	if trustResponse.Error != 0 {
		fmt.Printf("Trust API Error: %d - %s\n", trustResponse.Error, trustResponse.ErrorMessage)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": trustResponse.ErrorMessage, "errorCode": trustResponse.Error})
		return
	}

	neoRequest := NeoInsuranceRequest{
		GosNumber:       request.GovNumber,
		TechSery:        request.TechPassportSeria,
		TechNumber:      request.TechPassportNumber,
		OwnerPassSeria:  request.OwnerPSeries,
		OwnerPassNumber: request.OwnerPNumber,
	}

	neoResponse, err := c.callNeoInsuranceAPI(neoRequest)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to call neo insurance API", "details": err.Error()})
		return
	}

	sessionID := uuid.New().String()

	carType := 1
	if trustResponse.VehicleTypeID != "" {
		if vehicleTypeID, err := strconv.Atoi(trustResponse.VehicleTypeID); err == nil {
			carType = vehicleTypeID
		}
	}

	vehicleData := &VehicleData{
		Error:                 trustResponse.Error,
		ErrorMessage:          trustResponse.ErrorMessage,
		TechPassportIssueDate: trustResponse.TechPassportIssueDate,
		IssueYear:             trustResponse.IssueYear,
		VehicleTypeID:         trustResponse.VehicleTypeID,
		BodyNumber:            trustResponse.BodyNumber,
		EngineNumber:          trustResponse.EngineNumber,
		ModelID:               trustResponse.ModelID,
		MarkaID:               trustResponse.MarkaID,
		ModelName:             trustResponse.ModelName,
		OrgName:               trustResponse.OrgName,
		LastName:              trustResponse.LastName,
		FirstName:             trustResponse.FirstName,
		MiddleName:            trustResponse.MiddleName,
		UseTerritory:          trustResponse.UseTerritory,
		Fy:                    trustResponse.Fy,
		Pinfl:                 trustResponse.Pinfl,
		Inn:                   &trustResponse.Inn,
		Seats:                 trustResponse.Seats,
		GovNumber:             request.GovNumber,
		TechPassportNumber:    request.TechPassportNumber,
		TechPassportSeria:     request.TechPassportSeria,
		OwnerPNumber:          request.OwnerPNumber,
		OwnerPSeries:          request.OwnerPSeries,
		OwnerPinfl:            trustResponse.Pinfl,
		OwnerLastNameLatin:    "",
		OwnerFirstNameLatin:   "",
		OwnerMiddleNameLatin:  "",
		OwnerBirthDate:        "",
		OwnerOblast:           "",
		OwnerRayon:            "",
		IsOwner:               neoResponse.Result,
		Drivers:               []DriverStored{},
		Applicant:             ApplicantStored{},
		SummaStrahovki:        0,
		CarType:               carType,
	}

	// Проверяем isOwner через API check-person
	fmt.Printf("=== CHECKING IS_OWNER ===\n")
	checkPersonReq := CheckPersonRequest{
		GosNumber:       request.GovNumber,
		TechSery:        request.TechPassportSeria,
		TechNumber:      request.TechPassportNumber,
		OwnerPassSeria:  request.OwnerPSeries,
		OwnerPassNumber: request.OwnerPNumber,
	}

	checkPersonResp, err := c.callCheckPersonAPI(checkPersonReq)
	if err != nil {
		fmt.Printf("WARNING: Failed to check person ownership: %v\n", err)
		fmt.Printf("Using default IsOwner value: %t\n", vehicleData.IsOwner)
	} else {
		fmt.Printf("Check person API response: error=%d, result=%t\n",
			checkPersonResp.Error, checkPersonResp.Result)

		if checkPersonResp.Error == 1 {
			// API вернул ошибку, но это может означать, что человек не владелец
			vehicleData.IsOwner = false
			fmt.Printf("Setting IsOwner to false due to API error\n")
		} else {
			// Используем result из ответа
			vehicleData.IsOwner = checkPersonResp.Result
			fmt.Printf("Setting IsOwner to %t from API result\n", vehicleData.IsOwner)
		}
	}

	store.Lock()
	store.M[sessionID] = vehicleData
	store.Unlock()

	fmt.Printf("=== FINAL RESPONSE ===\n")
	fmt.Printf("Final IsOwner value: %t\n", vehicleData.IsOwner)

	response := NacaloResponse{
		OwnerPinfl:         trustResponse.Pinfl,
		OwnerName:          trustResponse.LastName,
		OwnerFamiliya:      trustResponse.FirstName,
		OwnerOtchestvo:     trustResponse.MiddleName,
		CarModel:           trustResponse.ModelName,
		CarYear:            trustResponse.IssueYear,
		GovNumber:          request.GovNumber,
		TechPassportNumber: request.TechPassportNumber,
		TechPassportSeria:  request.TechPassportSeria,
		SessionID:          sessionID,
		IsOwner:            vehicleData.IsOwner,
	}

	fmt.Printf("Response JSON: %+v\n", response)
	ctx.JSON(http.StatusOK, response)
}

func (c *UnifiedController) GetSession(ctx *gin.Context) {
	sessionID := ctx.Param("id")
	if sessionID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "session ID is required"})
		return
	}

	store.RLock()
	vehicleData, exists := store.M[sessionID]
	store.RUnlock()

	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	ctx.JSON(http.StatusOK, vehicleData)
}

func (c *UnifiedController) Calc(ctx *gin.Context) {
	var request CalcRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON", "details": err.Error()})
		return
	}

	if request.IsDrivers != 0 && request.IsDrivers != 5 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "isDrivers must be 0 or 5"})
		return
	}

	store.RLock()
	vehicleData, exists := store.M[request.UUID]
	store.RUnlock()

	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	periodNeoID, periodTrustID, ok := getMappingID(periodMapping, request.StrahovkaMonth)
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid strahovka month"})
		return
	}

	driversNeoID, driversTrustID, ok := getMappingID(driversMapping, request.IsDrivers)
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid drivers option"})
		return
	}

	_, _, ok = getMappingID(carTypeMapping, vehicleData.CarType)
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid car type in vehicle data"})
		return
	}

	var periodID, driversID int
	if request.Provider == "neo" {
		periodID = periodNeoID
		driversID = driversNeoID
	} else if request.Provider == "trust" {
		periodID = periodTrustID
		driversID = driversTrustID
	} else {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider. Must be 'neo' or 'trust'"})
		return
	}

	var summaStrahovki int64

	if request.Provider == "neo" {
		neoRequest := NeoCalcRequest{
			GosNumber:       vehicleData.GovNumber,
			TechSery:        vehicleData.TechPassportSeria,
			TechNumber:      vehicleData.TechPassportNumber,
			PeriodID:        periodID,
			NumberDriversID: driversID,
			CarTypeID:       vehicleData.CarType,
		}

		neoResponse, err := c.callNeoCalcAPI(neoRequest)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to call neo insurance calc API", "details": err.Error()})
			return
		}

		if neoResponse.Error != 0 {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": neoResponse.Message})
			return
		}

		summaStrahovki = int64(neoResponse.Response.AmountUZS)

	} else if request.Provider == "trust" {
		vehicleTypeID, err := strconv.Atoi(vehicleData.VehicleTypeID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid vehicle type ID"})
			return
		}

		trustRequest := TrustCalcRequest{
			Discount:     1,
			DriverLimit:  driversID,
			Period:       periodID,
			UseTerritory: vehicleData.UseTerritory,
			Vehicle:      vehicleTypeID,
		}

		trustResponse, err := c.callTrustCalcAPI(trustRequest)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to call trust insurance calc API", "details": err.Error()})
			return
		}

		fmt.Printf("=== TRUST CALC RESPONSE ===\n")
		fmt.Printf("Trust calc response: %+v\n", trustResponse)
		fmt.Printf("Trust prem value: %f\n", trustResponse.Prem)

		summaStrahovki = int64(trustResponse.Prem)
		fmt.Printf("Converted to int64: %d\n", summaStrahovki)

	} else {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider. Must be 'neo' or 'trust'"})
		return
	}

	store.Lock()
	vehicleData.SummaStrahovki = summaStrahovki
	vehicleData.Provider = request.Provider
	vehicleData.StrahovkaMonth = request.StrahovkaMonth
	vehicleData.IsDrivers = request.IsDrivers
	if request.ContractBegin != "" {
		vehicleData.ContractBegin = request.ContractBegin
	}
	store.M[request.UUID] = vehicleData
	store.Unlock()

	response := CalcResponse{
		SessionID:      request.UUID,
		SummaStrahovki: int(summaStrahovki),
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *UnifiedController) InitCon(ctx *gin.Context) {
	var request InitConRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	if len(request.Drivers) > 5 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "maximum 5 drivers allowed"})
		return
	}

	store.RLock()
	vehicleData, exists := store.M[request.UUID]
	store.RUnlock()

	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	if request.ContractBegin != "" {
		vehicleData.ContractBegin = request.ContractBegin
	}

	drivers := make([]DriverStored, 0, len(request.Drivers))
	for _, driverReq := range request.Drivers {
		passportReq := PassportPinflRequest{
			Pinfl:          driverReq.DriverPinfl,
			PassportSeries: driverReq.DriverPSeries,
			PassportNumber: driverReq.DriverPNumber,
		}

		passportResp, err := c.callPassportPinflAPI(passportReq)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get driver passport info", "details": err.Error()})
			return
		}

		fmt.Printf("Driver passport response: %+v\n", passportResp)

		licenseReq := DriverLicenseRequest{
			Pinfl:          driverReq.DriverPinfl,
			PassportSeries: driverReq.DriverPSeries,
			PassportNumber: driverReq.DriverPNumber,
		}

		licenseResp, err := c.callDriverLicenseAPI(licenseReq)
		if err != nil {
			fmt.Printf("Driver license API error: %v\n", err)
			licenseResp = &DriverLicenseResponse{}
		} else {
			fmt.Printf("Driver license response: %+v\n", licenseResp)
		}

		driver := DriverStored{
			DriverPinfl:     driverReq.DriverPinfl,
			DriverPSeries:   driverReq.DriverPSeries,
			DriverPNumber:   driverReq.DriverPNumber,
			LastNameLatin:   passportResp.LastNameLatin,
			FirstNameLatin:  passportResp.FirstNameLatin,
			MiddleNameLatin: passportResp.MiddleNameLatin,
			BirthDate:       passportResp.BirthDate,
			Oblast:          passportResp.Oblast,
			Rayon:           passportResp.Rayon,
			LicenseNumber:   licenseResp.LicenseNumber,
			LicenseSeries:   licenseResp.LicenseSeries,
			LicenseDate:     licenseResp.LicenseDate,
		}

		drivers = append(drivers, driver)
	}

	applicantPassportReq := PassportPinflRequest{
		Pinfl:          request.ApplPinfl,
		PassportSeries: request.ApplPSeries,
		PassportNumber: request.ApplPNumber,
	}

	applicantPassportResp, err := c.callPassportPinflAPI(applicantPassportReq)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get applicant passport info", "details": err.Error()})
		return
	}

	fmt.Printf("Applicant passport response: %+v\n", applicantPassportResp)

	applicant := ApplicantStored{
		ApplPinfl:       request.ApplPinfl,
		ApplPSeries:     request.ApplPSeries,
		ApplPNumber:     request.ApplPNumber,
		ApplPhone:       request.ApplPhone,
		LastNameLatin:   applicantPassportResp.LastNameLatin,
		FirstNameLatin:  applicantPassportResp.FirstNameLatin,
		MiddleNameLatin: applicantPassportResp.MiddleNameLatin,
		BirthDate:       applicantPassportResp.BirthDate,
		Oblast:          applicantPassportResp.Oblast,
		Rayon:           applicantPassportResp.Rayon,
	}

	// Получаем данные паспорта для owner
	ownerPassportReq := PassportPinflRequest{
		Pinfl:          vehicleData.OwnerPinfl,
		PassportSeries: vehicleData.OwnerPSeries,
		PassportNumber: vehicleData.OwnerPNumber,
	}

	ownerPassportResp, err := c.callPassportPinflAPI(ownerPassportReq)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get owner passport info", "details": err.Error()})
		return
	}

	fmt.Printf("Owner passport response: %+v\n", ownerPassportResp)

	store.Lock()
	vehicleData.Drivers = drivers
	vehicleData.Applicant = applicant
	vehicleData.OwnerLastNameLatin = ownerPassportResp.LastNameLatin
	vehicleData.OwnerFirstNameLatin = ownerPassportResp.FirstNameLatin
	vehicleData.OwnerMiddleNameLatin = ownerPassportResp.MiddleNameLatin
	vehicleData.OwnerBirthDate = ownerPassportResp.BirthDate
	vehicleData.OwnerOblast = ownerPassportResp.Oblast
	vehicleData.OwnerRayon = ownerPassportResp.Rayon
	store.M[request.UUID] = vehicleData
	store.Unlock()

	response := gin.H{
		"sessionId": request.UUID,
		"message":   "Data saved successfully",
		"drivers":   len(drivers),
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *UnifiedController) Submit(ctx *gin.Context) {
	fmt.Printf("=== SUBMIT REQUEST START ===\n")

	var request SubmitRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		fmt.Printf("ERROR: Invalid JSON in submit request: %v\n", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	fmt.Printf("Submit request: %+v\n", request)

	store.RLock()
	vehicleData, exists := store.M[request.UUID]
	store.RUnlock()

	if !exists {
		fmt.Printf("ERROR: Session not found for UUID: %s\n", request.UUID)
		ctx.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	fmt.Printf("Found session data for UUID: %s\n", request.UUID)
	fmt.Printf("Session provider: %s\n", vehicleData.Provider)

	if vehicleData.Provider == "" {
		fmt.Printf("ERROR: Provider not set in session data\n")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "provider not set. Call calc first"})
		return
	}

	if vehicleData.Provider == "neo" {
		fmt.Printf("=== NEO INSURANCE SUBMIT ===\n")
		fmt.Printf("Provider: %s\n", vehicleData.Provider)

		periodNeoID, _, _ := getMappingID(periodMapping, vehicleData.StrahovkaMonth)
		driversNeoID, _, _ := getMappingID(driversMapping, vehicleData.IsDrivers)

		fmt.Printf("Mapping - StrahovkaMonth: %d -> Neo PeriodID: %d\n", vehicleData.StrahovkaMonth, periodNeoID)
		fmt.Printf("Mapping - IsDrivers: %d -> Neo DriversID: %d\n", vehicleData.IsDrivers, driversNeoID)

		neoDrivers := make([]NeoSaveDriver, 0, len(vehicleData.Drivers))
		for i, driver := range vehicleData.Drivers {
			fmt.Printf("Driver %d: PINFL=%s, Series=%s, Number=%s, BirthDate=%s, Name=%s %s %s\n",
				i+1, driver.DriverPinfl, driver.DriverPSeries, driver.DriverPNumber,
				driver.BirthDate, driver.LastNameLatin, driver.FirstNameLatin, driver.MiddleNameLatin)

			neoDrivers = append(neoDrivers, NeoSaveDriver{
				PassportSeria:  driver.DriverPSeries,
				PassportNumber: driver.DriverPNumber,
				DriverBirthday: driver.BirthDate,
				Relative:       0,
				Name:           fmt.Sprintf("%s %s %s", driver.LastNameLatin, driver.FirstNameLatin, driver.MiddleNameLatin),
			})
		}

		applicantIsDriver := false
		for _, driver := range vehicleData.Drivers {
			if driver.DriverPinfl == vehicleData.Applicant.ApplPinfl {
				applicantIsDriver = true
				break
			}
		}
		fmt.Printf("Applicant is driver: %t (Applicant PINFL: %s)\n", applicantIsDriver, vehicleData.Applicant.ApplPinfl)

		neoSaveReq := NeoSaveRequest{
			AmountUZS:         vehicleData.SummaStrahovki,
			GosNumber:         vehicleData.GovNumber,
			TechNumber:        vehicleData.TechPassportNumber,
			TechSery:          vehicleData.TechPassportSeria,
			Drivers:           neoDrivers,
			ApplicantIsDriver: applicantIsDriver,
			PeriodID:          periodNeoID,
			NumberDriversID:   fmt.Sprintf("%d", driversNeoID),
			PhoneNumber:       vehicleData.Applicant.ApplPhone,
			OwnerPassSeria:    vehicleData.OwnerPSeries,
			OwnerPassNumber:   vehicleData.OwnerPNumber,
			OwnerPinfl:        vehicleData.OwnerPinfl,
		}

		fmt.Printf("=== NEO API REQUEST ===\n")
		fmt.Printf("URL: http://localhost:8080/neoInsurance/osago/save-policy\n")
		fmt.Printf("Request JSON:\n")
		reqJSON, _ := json.MarshalIndent(neoSaveReq, "", "  ")
		fmt.Printf("%s\n", string(reqJSON))

		neoSaveResp, err := c.callNeoSaveAPI(neoSaveReq)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to save neo insurance",
				"details": err.Error(),
				"debug":   neoDebugInfo,
			})
			return
		}

		store.Lock()
		vehicleData.NeoOrderID = &neoSaveResp.Response.OrderID
		vehicleData.NeoContractID = &neoSaveResp.Response.ContractID
		vehicleData.NeoPayURL = &neoSaveResp.Response.URL
		store.M[request.UUID] = vehicleData
		store.Unlock()

		ctx.JSON(http.StatusOK, gin.H{
			"sessionId":  request.UUID,
			"orderId":    neoSaveResp.Response.OrderID,
			"contractId": neoSaveResp.Response.ContractID,
			"payUrl":     neoSaveResp.Response.URL,
			"paymeUrl":   neoSaveResp.Response.PaymeURL,
			"amount":     vehicleData.SummaStrahovki,
		})

	} else if vehicleData.Provider == "trust" {
		fmt.Printf("=== TRUST INSURANCE SUBMIT ===\n")
		fmt.Printf("Provider: %s\n", vehicleData.Provider)
		fmt.Printf("VehicleData: %+v\n", vehicleData)

		periodTrustID, _, _ := getMappingID(periodMapping, vehicleData.StrahovkaMonth)
		_, driversTrustID, _ := getMappingID(driversMapping, vehicleData.IsDrivers)
		_, carTypeTrustID, _ := getMappingID(carTypeMapping, vehicleData.CarType)

		fmt.Printf("Mapping - StrahovkaMonth: %d -> Trust Period: %d\n", vehicleData.StrahovkaMonth, periodTrustID)
		fmt.Printf("Mapping - IsDrivers: %d -> Trust DriverLimit: %d\n", vehicleData.IsDrivers, driversTrustID)
		fmt.Printf("Mapping - CarType: %d -> Trust Type: %d\n", vehicleData.CarType, carTypeTrustID)

		_, _ = strconv.Atoi(vehicleData.VehicleTypeID)

		ownerIsDriver := false
		for _, driver := range vehicleData.Drivers {
			if driver.DriverPinfl == vehicleData.OwnerPinfl {
				ownerIsDriver = true
				break
			}
		}

		applicantIsOwner := (vehicleData.Applicant.ApplPinfl == vehicleData.OwnerPinfl) ||
			(vehicleData.Applicant.ApplPSeries == vehicleData.OwnerPSeries && vehicleData.Applicant.ApplPNumber == vehicleData.OwnerPNumber)

		fmt.Printf("Owner is driver: %t (Owner PINFL: %s)\n", ownerIsDriver, vehicleData.OwnerPinfl)
		fmt.Printf("Applicant is owner: %t (Applicant PINFL: %s, Owner PINFL: %s)\n", applicantIsOwner, vehicleData.Applicant.ApplPinfl, vehicleData.OwnerPinfl)

		trustDrivers := make([]TrustCreateDriver, 0, len(vehicleData.Drivers))
		for i, driver := range vehicleData.Drivers {
			fmt.Printf("Driver %d: PINFL=%s, Series=%s, Number=%s, BirthDate=%s, Name=%s %s %s, License=%s %s %s\n",
				i+1, driver.DriverPinfl, driver.DriverPSeries, driver.DriverPNumber,
				driver.BirthDate, driver.LastNameLatin, driver.FirstNameLatin, driver.MiddleNameLatin,
				driver.LicenseSeries, driver.LicenseNumber, driver.LicenseDate)

			trustDrivers = append(trustDrivers, TrustCreateDriver{
				DateBirth:  driver.BirthDate,
				Paspsery:   driver.DriverPSeries,
				Paspnumber: driver.DriverPNumber,
				Pinfl:      driver.DriverPinfl,
				Surname:    driver.LastNameLatin,
				Name:       driver.FirstNameLatin,
				Patronym:   driver.MiddleNameLatin,
				Licnumber:  driver.LicenseNumber,
				Licsery:    driver.LicenseSeries,
				Licdate:    driver.LicenseDate,
				Relative:   0,
				Resident:   1,
			})
		}

		fmt.Printf("Owner data: PINFL=%s, Series=%s, Number=%s, BirthDate=%s, Name=%s %s %s, Oblast=%s, Rayon=%s\n",
			vehicleData.OwnerPinfl, vehicleData.OwnerPSeries, vehicleData.OwnerPNumber,
			vehicleData.OwnerBirthDate, vehicleData.OwnerLastNameLatin, vehicleData.OwnerFirstNameLatin, vehicleData.OwnerMiddleNameLatin,
			vehicleData.OwnerOblast, vehicleData.OwnerRayon)

		fmt.Printf("Applicant data: PINFL=%s, Series=%s, Number=%s, BirthDate=%s, Name=%s %s %s, Oblast=%s, Rayon=%s, Phone=%s\n",
			vehicleData.Applicant.ApplPinfl, vehicleData.Applicant.ApplPSeries, vehicleData.Applicant.ApplPNumber,
			vehicleData.Applicant.BirthDate, vehicleData.Applicant.LastNameLatin, vehicleData.Applicant.FirstNameLatin, vehicleData.Applicant.MiddleNameLatin,
			vehicleData.Applicant.Oblast, vehicleData.Applicant.Rayon, vehicleData.Applicant.ApplPhone)

		yearInt, _ := strconv.Atoi(vehicleData.IssueYear)
		ownerOblastInt, _ := strconv.Atoi(vehicleData.OwnerOblast)
		ownerRayonInt, _ := strconv.Atoi(vehicleData.OwnerRayon)
		applOblastInt, _ := strconv.Atoi(vehicleData.Applicant.Oblast)
		applRayonInt, _ := strconv.Atoi(vehicleData.Applicant.Rayon)

		contractBeginFormatted := formatDateToDDMMYYYY(vehicleData.ContractBegin)

		trustCreateReq := TrustCreateRequest{
			ApplicantIsowner: boolToInt(applicantIsOwner),
			ContractBegin:    contractBeginFormatted,
			DriverLimit:      driversTrustID,
			Dvigatel:         vehicleData.EngineNumber,
			HasBenefit:       1,
			Kuzov:            vehicleData.BodyNumber,
			OwnerFy:          vehicleData.Fy,
			OwnerIsdriver:    boolToInt(ownerIsDriver),
			OwnerPhone:       vehicleData.Applicant.ApplPhone,
			Period:           periodTrustID,
			Renumber:         vehicleData.GovNumber,
			TexpDate:         vehicleData.TechPassportIssueDate,
			TexpNumber:       vehicleData.TechPassportNumber,
			TexpSery:         vehicleData.TechPassportSeria,
			Type:             carTypeTrustID,
			UseTerritory:     vehicleData.UseTerritory,
			Vmodel:           vehicleData.ModelName,
			Year:             yearInt,
			OwnerPinfl:       vehicleData.OwnerPinfl,
			OwnerBirthdate:   vehicleData.OwnerBirthDate,
			OwnerPaspSery:    vehicleData.OwnerPSeries,
			OwnerPaspNum:     vehicleData.OwnerPNumber,
			OwnerSurname:     vehicleData.OwnerLastNameLatin,
			OwnerName:        vehicleData.OwnerFirstNameLatin,
			OwnerPatronym:    vehicleData.OwnerMiddleNameLatin,
			OwnerOblast:      ownerOblastInt,
			OwnerRayon:       ownerRayonInt,
			OwnerIspensioner: 1,
			ApplFizyur:       0,
			ApplPinfl:        vehicleData.Applicant.ApplPinfl,
			ApplBirthdate:    vehicleData.Applicant.BirthDate,
			ApplPaspSery:     vehicleData.Applicant.ApplPSeries,
			ApplPaspNum:      vehicleData.Applicant.ApplPNumber,
			ApplSurname:      vehicleData.Applicant.LastNameLatin,
			ApplName:         vehicleData.Applicant.FirstNameLatin,
			ApplPatronym:     vehicleData.Applicant.MiddleNameLatin,
			ApplOblast:       applOblastInt,
			ApplRayon:        applRayonInt,
			Drivers:          trustDrivers,
		}

		fmt.Printf("=== TRUST API REQUEST ===\n")
		fmt.Printf("URL: http://localhost:8080/trustInsurance/osago/create\n")
		fmt.Printf("Request JSON:\n")
		reqJSON, _ := json.MarshalIndent(trustCreateReq, "", "  ")
		fmt.Printf("%s\n", string(reqJSON))
		fmt.Printf("Request JSON (compact): %s\n", string(reqJSON))
		fmt.Printf("Request size: %d bytes\n", len(reqJSON))

		trustCreateResp, err := c.callTrustCreateAPI(trustCreateReq)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create trust insurance", "details": err.Error()})
			return
		}

		fmt.Printf("Trust create response: %+v\n", trustCreateResp)

		// Parse insurance premium amount from Trust response
		insurancePremiumFromTrust, err := strconv.ParseInt(trustCreateResp.InsurancePremium, 10, 64)
		if err != nil {
			fmt.Printf("ERROR: Failed to parse insurance premium '%s': %v\n", trustCreateResp.InsurancePremium, err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse insurance premium", "details": err.Error()})
			return
		}

		// Use the amount from calc (stored in vehicleData.SummaStrahovki) instead of overwriting it
		// This ensures consistency between calc and submit
		amount := vehicleData.SummaStrahovki
		fmt.Printf("=== AMOUNT COMPARISON ===\n")
		fmt.Printf("Amount from calc (stored): %d\n", amount)
		fmt.Printf("Amount from Trust create: %d\n", insurancePremiumFromTrust)
		if amount != insurancePremiumFromTrust {
			fmt.Printf("WARNING: Amount mismatch! Using calc amount (%d) instead of Trust amount (%d)\n", amount, insurancePremiumFromTrust)
		} else {
			fmt.Printf("Amounts match - using calc amount: %d\n", amount)
		}

		// Get merchant info
		merchantInfo, err := c.getMerchantInfo()
		if err != nil {
			fmt.Printf("ERROR: Failed to get merchant info: %v\n", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get merchant info", "details": err.Error()})
			return
		}

		// Generate payment URLs
		clickURL, err := c.generateClickPaymentURL(amount, trustCreateResp.UUID, merchantInfo)
		if err != nil {
			fmt.Printf("ERROR: Failed to generate Click URL: %v\n", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate Click URL", "details": err.Error()})
			return
		}

		paymeURL, err := c.generatePaymePaymentURL(amount, trustCreateResp.AnketaID, trustCreateResp.UUID, merchantInfo)
		if err != nil {
			fmt.Printf("ERROR: Failed to generate Payme URL: %v\n", err)
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate Payme URL", "details": err.Error()})
			return
		}

		fmt.Printf("Generated Click URL: %s\n", clickURL)
		fmt.Printf("Generated Payme URL: %s\n", paymeURL)

		// Call payment check API
		paymentCheckReq := TrustPaymentCheckRequest{
			AnketaID: trustCreateResp.AnketaID,
			Lan:      "uz",
		}

		paymentCheckResp, err := c.callTrustPaymentCheckAPI(paymentCheckReq)
		if err != nil {
			fmt.Printf("WARNING: Failed to check payment status: %v\n", err)
			paymentCheckResp = &TrustPaymentCheckResponse{Error: 1, Message: "Payment check failed", Result: false}
		} else {
			fmt.Printf("Payment check response: %+v\n", paymentCheckResp)
		}

		// Save provider response raw JSON
		providerResponseRaw, _ := json.Marshal(trustCreateResp)
		providerResponseRawStr := string(providerResponseRaw)

		// Update store with all provider data (keep the amount from calc, don't overwrite)
		store.Lock()
		// vehicleData.SummaStrahovki already contains the correct amount from calc
		vehicleData.ProviderUUID = &trustCreateResp.UUID
		vehicleData.ProviderContractID = &trustCreateResp.AnketaID
		vehicleData.ProviderResponseRaw = &providerResponseRawStr
		vehicleData.PaymentCheckResponse = paymentCheckResp
		vehicleData.PayURLs = &struct {
			Click string `json:"click"`
			Payme string `json:"payme"`
		}{
			Click: clickURL,
			Payme: paymeURL,
		}
		store.M[request.UUID] = vehicleData
		store.Unlock()

		// Return unified response format (same as Neo)
		ctx.JSON(http.StatusOK, gin.H{
			"amount":     amount,
			"contractId": trustCreateResp.AnketaID,
			"orderId":    trustCreateResp.AnketaID,
			"sessionId":  request.UUID,
			"payUrl":     clickURL,
			"paymeUrl":   paymeURL,
		})
	} else {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider"})
		return
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func formatDateToDDMMYYYY(dateStr string) string {
	if dateStr == "" {
		return ""
	}

	// Парсим дату в формате YYYY-MM-DD
	parts := strings.Split(dateStr, "-")
	if len(parts) != 3 {
		return dateStr // Возвращаем как есть, если формат неожиданный
	}

	// Конвертируем в DD.MM.YYYY
	return fmt.Sprintf("%s.%s.%s", parts[2], parts[1], parts[0])
}

func (c *UnifiedController) generateClickPaymentURL(amount int64, providerUuid string, merchantInfo *MerchantInfo) (string, error) {
	amountFormatted := fmt.Sprintf("%.2f", float64(amount))
	returnURL := fmt.Sprintf("https://ersp.e-osgo.uz/uz/site/export-to-pdf?id=%s", providerUuid)

	clickURL := fmt.Sprintf("https://my.click.uz/services/pay?service_id=%s&merchant_id=%s&amount=%s&transaction_param=%s&return_url=%s",
		merchantInfo.Click.ServiceID,
		merchantInfo.Click.MerchantID,
		url.QueryEscape(amountFormatted),
		url.QueryEscape(providerUuid),
		url.QueryEscape(returnURL))

	return clickURL, nil
}

func (c *UnifiedController) generatePaymePaymentURL(amount int64, anketaID int, providerUuid string, merchantInfo *MerchantInfo) (string, error) {
	merchantID := merchantInfo.Payme.MerchantID
	orderID := fmt.Sprintf("%d", anketaID)
	amountInTiyin := amount
	returnURL := fmt.Sprintf("https://ersp.e-osgo.uz/uz/site/export-to-pdf?id=%s", providerUuid)

	params := fmt.Sprintf("m=%s;ac.order_id=%s;a=%d;c=%s;l=uz",
		merchantID, orderID, amountInTiyin, returnURL)

	encodedParams := base64.StdEncoding.EncodeToString([]byte(params))
	paymeURL := fmt.Sprintf("https://checkout.paycom.uz/%s", url.QueryEscape(encodedParams))

	return paymeURL, nil
}

func (c *UnifiedController) callTrustPaymentCheckAPI(request TrustPaymentCheckRequest) (*TrustPaymentCheckResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trust payment check request: %v", err)
	}

	token, err := c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to get trust token: %v", err)
	}

	req, err := http.NewRequest("POST", c.config.TrustBaseURL+"/api/payments/check", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create trust payment check request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make trust payment check request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read trust payment check response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("trust payment check API returned status %d: %s", resp.StatusCode, string(body))
	}

	var response TrustPaymentCheckResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trust payment check response: %v", err)
	}

	return &response, nil
}

func (c *UnifiedController) getMerchantInfo() (*MerchantInfo, error) {
	c.merchantMutex.RLock()
	if c.merchantInfoCache != nil && time.Now().Before(c.merchantInfoCache.ExpiresAt) {
		data := c.merchantInfoCache.Data
		c.merchantMutex.RUnlock()
		return data, nil
	}
	c.merchantMutex.RUnlock()

	c.merchantMutex.Lock()
	defer c.merchantMutex.Unlock()

	if c.merchantInfoCache != nil && time.Now().Before(c.merchantInfoCache.ExpiresAt) {
		return c.merchantInfoCache.Data, nil
	}

	req, err := http.NewRequest("GET", c.config.TrustBaseURL+"/api/payments/merchant-info", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create merchant info request: %v", err)
	}

	token, err := c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to get trust token: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make merchant info request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read merchant info response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("merchant info API returned status %d: %s", resp.StatusCode, string(body))
	}

	var merchantInfo MerchantInfo
	if err := json.Unmarshal(body, &merchantInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal merchant info response: %v", err)
	}

	if merchantInfo.Click.ServiceID == "" || merchantInfo.Click.MerchantID == "" || merchantInfo.Payme.MerchantID == "" {
		return nil, fmt.Errorf("merchant info missing required fields: %+v", merchantInfo)
	}

	c.merchantInfoCache = &MerchantInfoCache{
		Data:      &merchantInfo,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	return &merchantInfo, nil
}

func (c *UnifiedController) CheckPayment(ctx *gin.Context) {
	var request struct {
		AnketaID int `json:"anketa_id" binding:"required"`
	}

	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON", "details": err.Error()})
		return
	}

	paymentCheckReq := TrustPaymentCheckRequest{
		AnketaID: request.AnketaID,
		Lan:      "uz",
	}

	paymentCheckResp, err := c.callTrustPaymentCheckAPI(paymentCheckReq)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check payment status",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"anketa_id": request.AnketaID,
		"result":    paymentCheckResp.Result,
		"message":   paymentCheckResp.Message,
		"error":     paymentCheckResp.Error,
	})
}
