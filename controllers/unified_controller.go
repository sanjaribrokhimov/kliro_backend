package controllers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"kliro/config"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UnifiedController struct {
	config *config.Config
	client *http.Client
}

type CalcRequest struct {
	GosNumber       string `json:"gos_number" binding:"required"`
	TechSery        string `json:"tech_sery" binding:"required"`
	TechNumber      string `json:"tech_number" binding:"required"`
	OwnerPassSeria  string `json:"owner__pass_seria"`
	OwnerPassNumber string `json:"owner__pass_number"`
	PeriodID        string `json:"period_id" binding:"required"`
	NumberDriversID string `json:"number_drivers_id" binding:"required"`
}

type CalcResponse struct {
	Success bool        `json:"success"`
	Error   int         `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
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

type NeoJuridikRequest struct {
	GosNumber  string `json:"gos_number"`
	TechSery   string `json:"tech_sery"`
	TechNumber string `json:"tech_number"`
}

type NeoJuridikResponse struct {
	Error         int    `json:"error"`
	Result        bool   `json:"result"`
	Message       string `json:"message"`
	Response      int    `json:"response"`
	Pinfl         string `json:"pinfl"`
	Name          string `json:"name"`
	YoqilgiType   string `json:"yoqilgi_type"`
	VehicleTypeID int    `json:"vehicleTypeId"`
	IssueYear     int    `json:"issueYear"`
	ModelName     string `json:"modelName"`
}

type NeoCheckPersonRequest struct {
	GosNumber       string `json:"gos_number"`
	TechSery        string `json:"tech_sery"`
	TechNumber      string `json:"tech_number"`
	OwnerPassSeria  string `json:"owner__pass_seria"`
	OwnerPassNumber string `json:"owner__pass_number"`
}

type NeoCheckPersonResponse struct {
	Error   int    `json:"error"`
	Result  bool   `json:"result"`
	Message string `json:"message"`
}

type calcSession struct {
	AmountUZS       int
	Inn             bool
	IsOwner         bool
	Juridik         NeoJuridikResponse
	GosNumber       string
	TechSery        string
	TechNumber      string
	OwnerPassSeria  string
	OwnerPassNumber string
	PeriodID        string
	NumberDriversID string
	Provider        string
	OrderID         int64
}

var (
	sessionsMu sync.RWMutex
	sessions   = make(map[string]*calcSession)
)

func NewUnifiedController(cfg *config.Config) *UnifiedController {
	return &UnifiedController{
		config: cfg,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *UnifiedController) Calculate(ctx *gin.Context) {
	var request CalcRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, CalcResponse{
			Success: false,
			Error:   1,
			Message: fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	periodID, err := strconv.Atoi(request.PeriodID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, CalcResponse{
			Success: false,
			Error:   1,
			Message: "Invalid period_id format",
		})
		return
	}

	numberDriversID, err := strconv.Atoi(request.NumberDriversID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, CalcResponse{
			Success: false,
			Error:   1,
			Message: "Invalid number_drivers_id format",
		})
		return
	}

	c.calculateNeo(ctx, request, periodID, numberDriversID)
}

func (c *UnifiedController) calculateNeo(ctx *gin.Context, request CalcRequest, periodID, numberDriversID int) {
	neoPeriodID := mapPeriodToNeo(periodID)
	if neoPeriodID == 0 {
		ctx.JSON(http.StatusBadRequest, CalcResponse{
			Success: false,
			Error:   1,
			Message: "Invalid period_id. Must be 12 (12 months) or 6 (6 months)",
		})
		return
	}

	neoDriversID := mapDriversToNeo(numberDriversID)
	if neoDriversID == 0 {
		ctx.JSON(http.StatusBadRequest, CalcResponse{
			Success: false,
			Error:   1,
			Message: "Invalid number_drivers_id. Must be 0 (unlimited) or 5 (limited to 5 drivers)",
		})
		return
	}

	juridikRequest := NeoJuridikRequest{
		GosNumber:  request.GosNumber,
		TechSery:   request.TechSery,
		TechNumber: request.TechNumber,
	}

	juridikResponse, err := c.callNeoJuridikAPI(juridikRequest)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, CalcResponse{
			Success: false,
			Error:   1,
			Message: fmt.Sprintf("Failed to check juridic status: %v", err),
		})
		return
	}

	neoRequest := NeoCalcRequest{
		GosNumber:       request.GosNumber,
		TechSery:        request.TechSery,
		TechNumber:      request.TechNumber,
		PeriodID:        neoPeriodID,
		NumberDriversID: neoDriversID,
		CarTypeID:       juridikResponse.VehicleTypeID,
	}

	var checkPersonResp *NeoCheckPersonResponse
	if request.OwnerPassSeria != "" && request.OwnerPassNumber != "" {
		cpReq := NeoCheckPersonRequest{
			GosNumber:       request.GosNumber,
			TechSery:        request.TechSery,
			TechNumber:      request.TechNumber,
			OwnerPassSeria:  request.OwnerPassSeria,
			OwnerPassNumber: request.OwnerPassNumber,
		}
		cp, cpErr := c.callNeoCheckPersonAPI(cpReq)
		if cpErr != nil {
			checkPersonResp = &NeoCheckPersonResponse{Error: 1, Result: false, Message: cpErr.Error()}
		} else {
			checkPersonResp = cp
		}
	} else {
		checkPersonResp = &NeoCheckPersonResponse{Error: 1, Result: false, Message: "owner passport not provided"}
	}

	neoResponse, err := c.callNeoCalcAPI(neoRequest)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, CalcResponse{
			Success: false,
			Error:   1,
			Message: fmt.Sprintf("Failed to calculate insurance: %v", err),
		})
		return
	}

	if neoResponse.Error != 0 || !neoResponse.Result {
		sessionID := uuid.NewString()
		sessionsMu.Lock()
		sessions[sessionID] = &calcSession{
			AmountUZS:       neoResponse.Response.AmountUZS,
			Inn:             neoResponse.Response.Inn,
			IsOwner:         checkPersonResp.Result,
			Juridik:         *juridikResponse,
			GosNumber:       request.GosNumber,
			TechSery:        request.TechSery,
			TechNumber:      request.TechNumber,
			OwnerPassSeria:  request.OwnerPassSeria,
			OwnerPassNumber: request.OwnerPassNumber,
			PeriodID:        request.PeriodID,
			NumberDriversID: request.NumberDriversID,
		}
		sessionsMu.Unlock()
		ctx.JSON(http.StatusOK, CalcResponse{
			Success: false,
			Error:   neoResponse.Error,
			Message: neoResponse.Message,
			Data: map[string]interface{}{
				"calc": map[string]interface{}{
					"amount_uzs": neoResponse.Response.AmountUZS,
					"inn":        neoResponse.Response.Inn,
				},
				"is_owner": checkPersonResp.Result,
				"juridik":  juridikResponse,
				"requestsData": map[string]interface{}{
					"gos_number":         request.GosNumber,
					"tech_sery":          request.TechSery,
					"tech_number":        request.TechNumber,
					"owner__pass_seria":  request.OwnerPassSeria,
					"owner__pass_number": request.OwnerPassNumber,
					"period_id":          request.PeriodID,
					"number_drivers_id":  request.NumberDriversID,
				},
				"session_id": sessionID,
			},
		})
		return
	}

	sessionID := uuid.NewString()
	sessionsMu.Lock()
	sessions[sessionID] = &calcSession{
		AmountUZS:       neoResponse.Response.AmountUZS,
		Inn:             neoResponse.Response.Inn,
		IsOwner:         checkPersonResp.Result,
		Juridik:         *juridikResponse,
		GosNumber:       request.GosNumber,
		TechSery:        request.TechSery,
		TechNumber:      request.TechNumber,
		OwnerPassSeria:  request.OwnerPassSeria,
		OwnerPassNumber: request.OwnerPassNumber,
		PeriodID:        request.PeriodID,
		NumberDriversID: request.NumberDriversID,
	}
	sessionsMu.Unlock()
	ctx.JSON(http.StatusOK, CalcResponse{
		Success: true,
		Error:   neoResponse.Error,
		Message: neoResponse.Message,
		Data: map[string]interface{}{
			"calc": map[string]interface{}{
				"amount_uzs": neoResponse.Response.AmountUZS,
				"inn":        neoResponse.Response.Inn,
			},
			"is_owner": checkPersonResp.Result,
			"juridik":  juridikResponse,
			"requestsData": map[string]interface{}{
				"gos_number":         request.GosNumber,
				"tech_sery":          request.TechSery,
				"tech_number":        request.TechNumber,
				"owner__pass_seria":  request.OwnerPassSeria,
				"owner__pass_number": request.OwnerPassNumber,
				"period_id":          request.PeriodID,
				"number_drivers_id":  request.NumberDriversID,
			},
			"session_id": sessionID,
		},
	})
}

func (c *UnifiedController) callNeoJuridikAPI(request NeoJuridikRequest) (*NeoJuridikResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", c.config.NeoBaseURL+"/api/osago-neo/osago-juridik", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	creds := c.config.NeoLogin + ":" + c.config.NeoPassword
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
	req.Header.Set("Authorization", auth)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("neo juridik API returned status %d: %s", resp.StatusCode, string(body))
	}

	var juridikResponse NeoJuridikResponse
	if err := json.Unmarshal(body, &juridikResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &juridikResponse, nil
}

func (c *UnifiedController) callNeoCheckPersonAPI(request NeoCheckPersonRequest) (*NeoCheckPersonResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", c.config.NeoBaseURL+"/api/osago-neo/check-person", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	creds := c.config.NeoLogin + ":" + c.config.NeoPassword
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
	req.Header.Set("Authorization", auth)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("neo check-person API returned status %d: %s", resp.StatusCode, string(body))
	}

	var checkResp NeoCheckPersonResponse
	if err := json.Unmarshal(body, &checkResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &checkResp, nil
}

func (c *UnifiedController) callNeoCalcAPI(request NeoCalcRequest) (*NeoCalcResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", c.config.NeoBaseURL+"/api/osago-neo/get-calc-osago", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	creds := c.config.NeoLogin + ":" + c.config.NeoPassword
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
	req.Header.Set("Authorization", auth)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("neo calc API returned status %d: %s", resp.StatusCode, string(body))
	}

	var neoResponse NeoCalcResponse
	if err := json.Unmarshal(body, &neoResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	return &neoResponse, nil
}

func mapPeriodToNeo(ourPeriod int) int {
	switch ourPeriod {
	case 12:
		return 1
	case 6:
		return 2
	default:
		return 0
	}
}

func mapDriversToNeo(ourDrivers int) int {
	switch ourDrivers {
	case 0:
		return 1
	case 5:
		return 4
	default:
		return 0
	}
}

type CreateRequest struct {
	Provider          string                   `json:"provider" binding:"required"`
	SessionID         string                   `json:"session_id" binding:"required"`
	Drivers           []map[string]interface{} `json:"drivers"`
	ApplicantIsDriver bool                     `json:"applicant_is_driver"`
	PhoneNumber       string                   `json:"phone_number"`
	OwnerInn          string                   `json:"owner__inn"`
	StartDate         string                   `json:"start_date"`
	Vehicle           map[string]interface{}   `json:"vehicle"`
	Owner             map[string]interface{}   `json:"owner"`
	Policy            map[string]interface{}   `json:"policy"`
}

type neoSaveV2Request struct {
	AmountUZS         int                      `json:"amount_uzs"`
	GosNumber         string                   `json:"gos_number"`
	TechNumber        string                   `json:"tech_number"`
	TechSery          string                   `json:"tech_sery"`
	Drivers           []map[string]interface{} `json:"drivers"`
	ApplicantIsDriver bool                     `json:"applicant_is_driver"`
	PeriodID          int                      `json:"period_id"`
	NumberDriversID   string                   `json:"number_drivers_id"`
	PhoneNumber       string                   `json:"phone_number"`
	OwnerInn          string                   `json:"owner__inn"`
	StartDate         string                   `json:"startDate,omitempty"`
	OwnerPinfl        string                   `json:"owner__pinfl,omitempty"`
	OwnerPassSeria    string                   `json:"owner__pass_seria,omitempty"`
	OwnerPassNumber   string                   `json:"owner__pass_number,omitempty"`
}

type neoSaveV2Response struct {
	Result   bool   `json:"result"`
	Message  string `json:"message"`
	Response struct {
		AmountUZS  int    `json:"amount_uzs"`
		OrderID    int64  `json:"order_id"`
		ContractID int64  `json:"contract_id"`
		URL        string `json:"url"`
		PaymeURL   string `json:"payme_url"`
	} `json:"response"`
}

func (c *UnifiedController) callNeoSaveV2(payload neoSaveV2Request) (*neoSaveV2Response, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %v", err)
	}
	url := c.config.NeoBaseURL + "/api/osago-neo/save-policy/v2"
	fmt.Printf("NEO SAVE V2 URL: %s\n", url)
	fmt.Printf("NEO SAVE V2 REQUEST: %s\n", string(b))
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	creds := c.config.NeoLogin + ":" + c.config.NeoPassword
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %v", err)
	}
	fmt.Printf("NEO SAVE V2 STATUS: %d\n", resp.StatusCode)
	fmt.Printf("NEO SAVE V2 RESPONSE: %s\n", string(body))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("neo save v2 status %d: %s", resp.StatusCode, string(body))
	}
	var out neoSaveV2Response
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("unmarshal response: %v", err)
	}
	return &out, nil
}

func (c *UnifiedController) Create(ctx *gin.Context) {
	var req CreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, CalcResponse{Success: false, Error: 1, Message: fmt.Sprintf("Invalid request: %v", err)})
		return
	}
	sessionsMu.RLock()
	s, ok := sessions[req.SessionID]
	sessionsMu.RUnlock()
	if !ok {
		ctx.JSON(http.StatusNotFound, CalcResponse{Success: false, Error: 1, Message: "session not found"})
		return
	}
	switch req.Provider {
	case "neo":
		for i := range req.Drivers {
			if _, ok := req.Drivers[i]["name"]; !ok {
				req.Drivers[i]["name"] = ""
			}
		}
		p := neoSaveV2Request{
			AmountUZS:  s.AmountUZS,
			GosNumber:  s.GosNumber,
			TechNumber: s.TechNumber,
			TechSery:   s.TechSery,
			Drivers:    req.Drivers,
			ApplicantIsDriver: func() bool {
				if req.ApplicantIsDriver {
					return true
				}
				return s.IsOwner
			}(),
			PeriodID:        mapPeriodToNeo(mustAtoi(s.PeriodID)),
			NumberDriversID: mapDriversToNeoStr(s.NumberDriversID),
			PhoneNumber:     req.PhoneNumber,
			OwnerInn:        req.OwnerInn,
			StartDate:       req.StartDate,
			OwnerPinfl:      s.Juridik.Pinfl,
			OwnerPassSeria:  s.OwnerPassSeria,
			OwnerPassNumber: s.OwnerPassNumber,
		}
		combinedRequests := map[string]interface{}{
			"session_id":          req.SessionID,
			"gos_number":          s.GosNumber,
			"tech_sery":           s.TechSery,
			"tech_number":         s.TechNumber,
			"owner__pass_seria":   s.OwnerPassSeria,
			"owner__pass_number":  s.OwnerPassNumber,
			"period_id":           s.PeriodID,
			"number_drivers_id":   s.NumberDriversID,
			"amount_uzs":          s.AmountUZS,
			"inn":                 s.Inn,
			"is_owner":            s.IsOwner,
			"juridik":             s.Juridik,
			"drivers":             req.Drivers,
			"applicant_is_driver": true,
			"phone_number":        req.PhoneNumber,
			"owner__inn":          req.OwnerInn,
			"start_date":          req.StartDate,
		}
		resp, err := c.callNeoSaveV2(p)
		if err != nil {
			ctx.JSON(http.StatusBadGateway, CalcResponse{Success: false, Error: 1, Message: err.Error(), Data: map[string]interface{}{"provider": req.Provider, "requestsData": combinedRequests}})
			return
		}
		pay := map[string]interface{}{"click": resp.Response.URL, "payme": resp.Response.PaymeURL}
		respClean := map[string]interface{}{
			"amount_uzs":  resp.Response.AmountUZS,
			"order_id":    resp.Response.OrderID,
			"contract_id": resp.Response.ContractID,
		}
		sessionsMu.Lock()
		if ss, ok := sessions[req.SessionID]; ok {
			ss.Provider = "neo"
			ss.OrderID = resp.Response.OrderID
			sessions[req.SessionID] = ss
		}
		sessionsMu.Unlock()
		ctx.JSON(http.StatusOK, CalcResponse{Success: resp.Result, Error: 0, Message: resp.Message, Data: map[string]interface{}{"provider": req.Provider, "response": respClean, "pay": pay, "requestsData": combinedRequests}})
		return
	}
	if req.Provider == "gross" {
		g := grossSaveRequest{}
		g.Details.StartDate = req.StartDate
		if strings.Contains(g.Details.StartDate, "-") && len(g.Details.StartDate) == 10 {
			parts := strings.Split(g.Details.StartDate, "-")
			if len(parts) == 3 {
				g.Details.StartDate = parts[2] + "." + parts[1] + "." + parts[0]
			}
		}
		g.Details.PeriodID = mapPeriodToNeo(mustAtoi(s.PeriodID))
		if g.Details.PeriodID == 1 {
			g.Details.PeriodID = 1
		} else {
			g.Details.PeriodID = 2
		}
		if mapDriversToNeoStr(s.NumberDriversID) == "1" {
			g.Details.NumberDriversID = 1
		} else {
			g.Details.NumberDriversID = 4
		}
		phone := req.PhoneNumber
		if phone != "" && !strings.HasPrefix(phone, "+") {
			phone = "+" + phone
		}
		g.Details.PhoneNumber = phone
		g.Details.AmountUZS = s.AmountUZS
		g.TechPassport.GovNumber = s.GosNumber
		g.TechPassport.TechSery = s.TechSery
		g.TechPassport.TechNumber = s.TechNumber
		g.Owner.Person.Pinfl = s.Juridik.Pinfl
		g.Owner.Person.PassSeria = s.OwnerPassSeria
		g.Owner.Person.PassNumber = s.OwnerPassNumber
		g.Applicant.Pinfl = s.Juridik.Pinfl
		g.Applicant.PassSeria = s.OwnerPassSeria
		g.Applicant.PassNumber = s.OwnerPassNumber
		g.Applicant.IsDriver = req.ApplicantIsDriver
		g.Drivers = req.Drivers
		resp, err := c.callGrossSave(g)
		if err != nil {
			ctx.JSON(http.StatusBadGateway, CalcResponse{Success: false, Error: 1, Message: err.Error()})
			return
		}
		pay := map[string]interface{}{}
		if resp.Response != nil {
			if v, ok := resp.Response["click"].(map[string]interface{}); ok {
				if u, ok2 := v["url"].(string); ok2 {
					pay["click"] = u
				}
			}
			if v, ok := resp.Response["payme"].(map[string]interface{}); ok {
				if u, ok2 := v["url"].(string); ok2 {
					pay["payme"] = u
				}
			}
		}
		responseClean := map[string]interface{}{}
		if resp.Response != nil {
			if a, ok := resp.Response["amount_uzs"]; ok {
				responseClean["amount_uzs"] = a
			}
			if o, ok := resp.Response["order_id"]; ok {
				responseClean["order_id"] = o
			}
		}
		combinedRequests := map[string]interface{}{
			"session_id":         req.SessionID,
			"gos_number":         s.GosNumber,
			"tech_sery":          s.TechSery,
			"tech_number":        s.TechNumber,
			"owner__pass_seria":  s.OwnerPassSeria,
			"owner__pass_number": s.OwnerPassNumber,
			"period_id":          s.PeriodID,
			"number_drivers_id":  s.NumberDriversID,
			"amount_uzs":         s.AmountUZS,
			"inn":                s.Inn,
			"is_owner":           s.IsOwner,
			"juridik":            s.Juridik,
			"drivers":            req.Drivers,
			"phone_number":       req.PhoneNumber,
			"owner__inn":         req.OwnerInn,
			"start_date":         g.Details.StartDate,
		}
		if oid, ok := resp.Response["order_id"].(float64); ok {
			sessionsMu.Lock()
			if ss, ok2 := sessions[req.SessionID]; ok2 {
				ss.Provider = "gross"
				ss.OrderID = int64(oid)
				sessions[req.SessionID] = ss
			}
			sessionsMu.Unlock()
		}
		ctx.JSON(http.StatusOK, CalcResponse{Success: resp.Result, Error: 0, Message: resp.Message, Data: map[string]interface{}{"provider": req.Provider, "response": responseClean, "pay": pay, "requestsData": combinedRequests}})
		return
	}
	ctx.JSON(http.StatusNotImplemented, CalcResponse{Success: false, Error: 1, Message: "provider not implemented"})
}

func mustAtoi(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func mapDriversToNeoStr(our string) string {
	switch our {
	case "0":
		return "1"
	case "5":
		return "4"
	default:
		return our
	}
}

type grossSaveRequest struct {
	Details struct {
		StartDate       string `json:"start_date"`
		PeriodID        int    `json:"period_id"`
		NumberDriversID int    `json:"number_drivers_id"`
		PhoneNumber     string `json:"phone_number"`
		AmountUZS       int    `json:"amount_uzs"`
	} `json:"details"`
	TechPassport struct {
		GovNumber  string `json:"govNumber"`
		TechSery   string `json:"tech_sery"`
		TechNumber string `json:"tech_number"`
	} `json:"techPassport"`
	Owner struct {
		Person struct {
			Pinfl      string `json:"pinfl"`
			PassSeria  string `json:"pass_seria"`
			PassNumber string `json:"pass_number"`
		} `json:"person"`
	} `json:"owner"`
	Applicant struct {
		Pinfl            string `json:"pinfl"`
		Birthdate        string `json:"birthdate"`
		PassSeria        string `json:"pass_seria"`
		PassNumber       string `json:"pass_number"`
		IsDriver         bool   `json:"is_driver"`
		LicenseSeria     string `json:"licenseSeria,omitempty"`
		LicenseNumber    string `json:"licenseNumber,omitempty"`
		LicenseIssueDate string `json:"licenseIssueDate,omitempty"`
		Relative         *int   `json:"relative"`
	} `json:"applicant"`
	Drivers []map[string]interface{} `json:"drivers"`
}

type grossAPIResponse struct {
	Result   bool                   `json:"result"`
	Message  string                 `json:"message"`
	Response map[string]interface{} `json:"response"`
}

func (c *UnifiedController) callGrossSave(req grossSaveRequest) (*grossAPIResponse, error) {
	b, _ := json.Marshal(req)
	url := c.config.GrossBaseURL + "/osago-gross/save-policy-manual"
	fmt.Printf("GROSS SAVE URL: %s\n", url)
	fmt.Printf("GROSS SAVE REQUEST: %s\n", string(b))
	httpReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.SetBasicAuth(c.config.GrossLogin, c.config.GrossPassword)
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("GROSS SAVE STATUS: %d\n", resp.StatusCode)
	fmt.Printf("GROSS SAVE RESPONSE: %s\n", string(body))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gross status %d: %s", resp.StatusCode, string(body))
	}
	var out grossAPIResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type CheckPaymentRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

type UnifiedCheckResponse struct {
	IsPaid       bool   `json:"isPaid"`
	DownloadURL  string `json:"downloadUrl,omitempty"`
	PolicyNumber string `json:"policyNumber,omitempty"`
	BeginDate    string `json:"beginDate,omitempty"`
	EndDate      string `json:"endDate,omitempty"`
}

func (c *UnifiedController) CheckPaymentUnified(ctx *gin.Context) {
	var req CheckPaymentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, CalcResponse{Success: false, Error: 1, Message: "Invalid request"})
		return
	}
	sessionsMu.RLock()
	s, ok := sessions[req.SessionID]
	sessionsMu.RUnlock()
	if !ok || s.OrderID == 0 || s.Provider == "" {
		ctx.JSON(http.StatusBadRequest, CalcResponse{Success: false, Error: 1, Message: "session not ready"})
		return
	}
	if s.Provider == "neo" {
		url := c.config.NeoBaseURL + "/api/osago-neo/confirm-check"
		payload := map[string]interface{}{"order_id": s.OrderID}
		b, _ := json.Marshal(payload)
		reqHttp, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))
		reqHttp.Header.Set("Content-Type", "application/json")
		creds := c.config.NeoLogin + ":" + c.config.NeoPassword
		reqHttp.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))
		fmt.Printf("NEO CHECK URL: %s\n", url)
		fmt.Printf("NEO CHECK REQUEST: %s\n", string(b))
		resp, err := c.client.Do(reqHttp)
		if err != nil {
			ctx.JSON(http.StatusBadGateway, CalcResponse{Success: false, Error: 1, Message: err.Error()})
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("NEO CHECK STATUS: %d\n", resp.StatusCode)
		fmt.Printf("NEO CHECK RESPONSE: %s\n", string(body))
		var raw map[string]interface{}
		_ = json.Unmarshal(body, &raw)
		isPaid := false
		if v, ok := raw["error"].(float64); ok && (int(v) == 0 || int(v) == 2) {
			isPaid = true
		}
		if msg, ok := raw["message"].(string); ok && strings.EqualFold(msg, "PAID") {
			isPaid = true
		}
		var gos, pol, bdt, edt, pdf string
		if res, ok := raw["result"].(map[string]interface{}); ok {
			if v, ok := res["gos_number"].(string); ok {
				gos = v
			}
			if v, ok := res["policy_number"].(string); ok {
				pol = v
			}
			if v, ok := res["begin_date"].(string); ok {
				bdt = v
			}
			if v, ok := res["end_date"].(string); ok {
				edt = v
			}
			if v, ok := res["pdf_url"].(string); ok {
				pdf = v
			}
		}
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"isPaid":       isPaid,
			"gosNumber":    gos,
			"policyNumber": pol,
			"beginDate":    bdt,
			"endDate":      edt,
			"downloadUrl":  pdf,
			"responseData": raw,
		})
		return
	}
	if s.Provider == "gross" {
		url := c.config.GrossBaseURL + "/osago-gross/confirm-policy"
		payload := map[string]interface{}{"order_id": s.OrderID}
		b, _ := json.Marshal(payload)
		reqHttp, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))
		reqHttp.Header.Set("Content-Type", "application/json")
		reqHttp.Header.Set("Accept", "application/json")
		reqHttp.SetBasicAuth(c.config.GrossLogin, c.config.GrossPassword)
		fmt.Printf("GROSS CHECK URL: %s\n", url)
		fmt.Printf("GROSS CHECK REQUEST: %s\n", string(b))
		resp, err := c.client.Do(reqHttp)
		if err != nil {
			ctx.JSON(http.StatusBadGateway, CalcResponse{Success: false, Error: 1, Message: err.Error()})
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("GROSS CHECK STATUS: %d\n", resp.StatusCode)
		fmt.Printf("GROSS CHECK RESPONSE: %s\n", string(body))
		var raw map[string]interface{}
		_ = json.Unmarshal(body, &raw)
		var gr struct {
			Error   int                    `json:"error"`
			Message string                 `json:"message"`
			Result  map[string]interface{} `json:"result"`
		}
		_ = json.Unmarshal(body, &gr)
		isPaid := gr.Error == 2 || strings.EqualFold(gr.Message, "PAID")
		var gos, pol, bdt, edt, pdf string
		if gr.Result != nil {
			if u, ok := gr.Result["pdf_url"].(string); ok {
				pdf = u
			}
			if p, ok := gr.Result["policy_number"].(string); ok {
				pol = p
			}
			if bd, ok := gr.Result["begin_date"].(string); ok {
				bdt = bd
			}
			if ed, ok := gr.Result["end_date"].(string); ok {
				edt = ed
			}
			if gsn, ok := gr.Result["gos_number"].(string); ok {
				gos = gsn
			}
		}
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"isPaid":       isPaid,
			"gosNumber":    gos,
			"policyNumber": pol,
			"beginDate":    bdt,
			"endDate":      edt,
			"downloadUrl":  pdf,
			"responseData": raw,
		})
		return
	}
	ctx.JSON(http.StatusBadRequest, CalcResponse{Success: false, Error: 1, Message: "unknown provider"})
}
