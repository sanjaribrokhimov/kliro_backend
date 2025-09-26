package controllers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"kliro/config"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type VehicleData struct {
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
	GovNumber             string `json:"govNumber"`
	TechPassportNumber    string `json:"techPassportNumber"`
	TechPassportSeria     string `json:"techPassportSeria"`
	OwnerPNumber          string `json:"ownerPNumber"`
	OwnerPSeries          string `json:"ownerPSeries"`
	OwnerPinfl            string `json:"ownerPinfl"`
	IsOwner               bool   `json:"isOwner"`
	SummaStrahovki        int    `json:"summaStrahovki"`
}

type UnifiedController struct {
	config      *config.Config
	client      *http.Client
	token       string
	tokenExpiry time.Time
	mutex       sync.RWMutex
}

var (
	vehicleStorage = make(map[string]VehicleData)
	storageMutex   = sync.RWMutex{}

	periodMapping = []Mapping{
		{OurID: 6, NeoID: 2, TrustID: 1},
		{OurID: 12, NeoID: 1, TrustID: 2},
	}

	driversMapping = []Mapping{
		{OurID: 5, NeoID: 4, TrustID: 1},
		{OurID: 0, NeoID: 1, TrustID: 0},
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

type NacaloRequest struct {
	GovNumber          string `json:"govNumber"`
	TechPassportNumber string `json:"techPassportNumber"`
	TechPassportSeria  string `json:"techPassportSeria"`
	OwnerPNumber       string `json:"ownerPNumber"`
	OwnerPSeries       string `json:"ownerPSeries"`
	OwnerPinfl         string `json:"ownerPinfl"`
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

type CalcRequest struct {
	StrahovkaMonth int    `json:"strahovkaMonth"`
	IsDrivers      int    `json:"isDrivers"`
	CarType        int    `json:"carType"`
	UUID           string `json:"uuid"`
	Provider       string `json:"provider"`
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

	req, err := http.NewRequest("POST", c.config.TrustBaseURL+"/api/products/auth/login", bytes.NewBuffer(jsonData))
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

	var trustResponse TrustVehicleResponse
	if err := json.Unmarshal(body, &trustResponse); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	if trustResponse.Error != 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": trustResponse.ErrorMessage})
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

	vehicleData := VehicleData{
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
		Inn:                   trustResponse.Inn,
		Seats:                 trustResponse.Seats,
		GovNumber:             request.GovNumber,
		TechPassportNumber:    request.TechPassportNumber,
		TechPassportSeria:     request.TechPassportSeria,
		OwnerPNumber:          request.OwnerPNumber,
		OwnerPSeries:          request.OwnerPSeries,
		OwnerPinfl:            request.OwnerPinfl,
		IsOwner:               neoResponse.Result,
	}

	storageMutex.Lock()
	vehicleStorage[sessionID] = vehicleData
	storageMutex.Unlock()

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
		IsOwner:            neoResponse.Result,
	}

	ctx.JSON(http.StatusOK, response)
}

func (c *UnifiedController) GetSession(ctx *gin.Context) {
	sessionID := ctx.Param("id")
	if sessionID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "session ID is required"})
		return
	}

	storageMutex.RLock()
	vehicleData, exists := vehicleStorage[sessionID]
	storageMutex.RUnlock()

	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	ctx.JSON(http.StatusOK, vehicleData)
}

func (c *UnifiedController) Calc(ctx *gin.Context) {
	var request CalcRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	storageMutex.RLock()
	vehicleData, exists := vehicleStorage[request.UUID]
	storageMutex.RUnlock()

	if !exists {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	periodID, err := findMapping(periodMapping, request.StrahovkaMonth, request.Provider)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	driversID, err := findMapping(driversMapping, request.IsDrivers, request.Provider)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	carTypeID, err := findMapping(carTypeMapping, request.CarType, request.Provider)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var summaStrahovki int

	if request.Provider == "neo" {
		neoRequest := NeoCalcRequest{
			GosNumber:       vehicleData.GovNumber,
			TechSery:        vehicleData.TechPassportSeria,
			TechNumber:      vehicleData.TechPassportNumber,
			PeriodID:        periodID,
			NumberDriversID: driversID,
			CarTypeID:       carTypeID,
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

		summaStrahovki = neoResponse.Response.AmountUZS

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
			CarType:      carTypeID,
		}

		trustResponse, err := c.callTrustCalcAPI(trustRequest)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to call trust insurance calc API", "details": err.Error()})
			return
		}

		summaStrahovki = int(trustResponse.Prem)

	} else {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider. Must be 'neo' or 'trust'"})
		return
	}

	storageMutex.Lock()
	vehicleData.SummaStrahovki = summaStrahovki
	vehicleStorage[request.UUID] = vehicleData
	storageMutex.Unlock()

	response := CalcResponse{
		SessionID:      request.UUID,
		SummaStrahovki: summaStrahovki,
	}

	ctx.JSON(http.StatusOK, response)
}
