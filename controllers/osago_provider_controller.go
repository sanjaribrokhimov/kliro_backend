package controllers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kliro/config"

	"github.com/gin-gonic/gin"
)

// OsagoProviderController implements provider-specific calculate endpoints (NEO, GROSS, Euroasia).
// Эти ручки изолированы от старого unified-контроллера, чтобы можно было тестировать каждую
// интеграцию отдельно на этапе расчёта.
type OsagoProviderController struct {
	config *config.Config
	client *http.Client
}

func NewOsagoProviderController(cfg *config.Config) *OsagoProviderController {
	return &OsagoProviderController{
		config: cfg,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// ===== NEO: calculate =====

type neoCalcPayload struct {
	GosNumber       string `json:"gos_number" binding:"required"`
	TechSery        string `json:"tech_sery" binding:"required"`
	TechNumber      string `json:"tech_number" binding:"required"`
	PeriodID        int    `json:"period_id" binding:"required"`         // 12 | 6 (месяцев)
	NumberDriversID int    `json:"number_drivers_id" binding:"required"` // 0 (unlimited) | 5 (лимит)
}

type neoCalcResponse struct {
	Error    int    `json:"error"`
	Result   bool   `json:"result"`
	Message  string `json:"message"`
	Response struct {
		AmountUZS int  `json:"amount_uzs"`
		Inn       bool `json:"inn"`
	} `json:"response"`
}

type neoJuridikRequest struct {
	GosNumber  string `json:"gos_number"`
	TechSery   string `json:"tech_sery"`
	TechNumber string `json:"tech_number"`
}

type neoJuridikResponse struct {
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

// CalcNeo выполняет только расчет в NEO (без сохранения/создания полиса).
func (c *OsagoProviderController) CalcNeo(ctx *gin.Context) {
	var payload neoCalcPayload
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	neoPeriodID := mapPeriodToNeo(payload.PeriodID)
	if neoPeriodID == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid period_id: allowed 12 or 6"})
		return
	}
	neoDriversID := mapDriversToNeo(payload.NumberDriversID)
	if neoDriversID == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid number_drivers_id: allowed 0 (unlimited) or 5"})
		return
	}

	// Сначала проверим юр/физ статус авто, чтобы получить vehicleTypeId.
	jReq := neoJuridikRequest{
		GosNumber:  payload.GosNumber,
		TechSery:   payload.TechSery,
		TechNumber: payload.TechNumber,
	}
	jResp, err := c.callNeoJuridikAPI(jReq)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}

	neoRequest := map[string]interface{}{
		"gos_number":        payload.GosNumber,
		"tech_sery":         payload.TechSery,
		"tech_number":       payload.TechNumber,
		"period_id":         neoPeriodID,
		"number_drivers_id": neoDriversID,
		"car_type_id":       jResp.VehicleTypeID,
	}

	neoResp, err := c.callNeoCalcAPI(neoRequest)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"provider": "neo",
		"success":  neoResp.Error == 0 && neoResp.Result,
		"message":  neoResp.Message,
		"data": map[string]interface{}{
			"amount_uzs": neoResp.Response.AmountUZS,
			"inn":        neoResp.Response.Inn,
			"juridik":    jResp,
		},
	})
}

func (c *OsagoProviderController) callNeoJuridikAPI(request neoJuridikRequest) (*neoJuridikResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %v", err)
	}
	req, err := http.NewRequest("POST", c.config.NeoBaseURL+"/api/osago-neo/osago-juridik", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	creds := c.config.NeoLogin + ":" + c.config.NeoPassword
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("neo juridik status %d: %s", resp.StatusCode, string(body))
	}
	var out neoJuridikResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("unmarshal response: %v", err)
	}
	return &out, nil
}

// ===== Shared / helpers =====

// euroasiaVehicleGroup описывает один элемент из /api/v1/insurance/lookups/vehicle-groups
type euroasiaVehicleGroup struct {
	ID           string `json:"id"`
	ExternalID   int    `json:"external_id"`
	Name         string `json:"name"`
	Translations []struct {
		Lang  string `json:"lang"`
		Value string `json:"value"`
	} `json:"translations"`
}

type euroasiaVehicleGroupsResponse struct {
	Data    []euroasiaVehicleGroup `json:"data"`
	Message string                 `json:"message"`
	Success bool                   `json:"success"`
}

// callEuroasiaVehicleGroupsAPI запрашивает список vehicle groups у Euroasia.
// Мы используем external_id как единый (универсальный) ID для фронта.
func (c *OsagoProviderController) callEuroasiaVehicleGroupsAPI(lang string) ([]euroasiaVehicleGroup, error) {
	if c.config.EuroasiaBaseURL == "" || c.config.EuroasiaAPIKey == "" {
		return nil, fmt.Errorf("euroasia config missing (EUROASIA_BASE_URL or EUROASIA_API_KEY)")
	}
	if lang == "" {
		lang = "ru"
	}

	url := strings.TrimRight(c.config.EuroasiaBaseURL, "/") + "/api/v1/insurance/lookups/vehicle-groups"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %v", err)
	}
	req.Header.Set("Accept-Language", lang)
	req.Header.Set("Authorization", c.config.EuroasiaAPIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vehicle-groups status %d: %s", resp.StatusCode, string(body))
	}

	var out euroasiaVehicleGroupsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("unmarshal response: %v", err)
	}
	if !out.Success {
		return nil, fmt.Errorf("vehicle-groups not success: %s", out.Message)
	}
	return out.Data, nil
}

func (c *OsagoProviderController) callNeoCalcAPI(payload map[string]interface{}) (*neoCalcResponse, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %v", err)
	}
	req, err := http.NewRequest("POST", c.config.NeoBaseURL+"/api/osago-neo/get-calc-osago", bytes.NewBuffer(b))
	if err != nil {
		return nil, fmt.Errorf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	creds := c.config.NeoLogin + ":" + c.config.NeoPassword
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(creds)))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("neo calc status %d: %s", resp.StatusCode, string(body))
	}
	var out neoCalcResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("unmarshal response: %v", err)
	}
	return &out, nil
}

// ===== Unified: calculate all providers =====

// CalcAllPayload — единый запрос для расчёта по всем провайдерам.
// Верхнеуровневые поля используются для NEO.
// Euroasia использует поля accept_language, vehicle_group_external_id и euroasia_body.
type CalcAllPayload struct {
	GosNumber         string                 `json:"gos_number" binding:"required"`
	TechSery          string                 `json:"tech_sery" binding:"required"`
	TechNumber        string                 `json:"tech_number" binding:"required"`
	PeriodID          int                    `json:"period_id" binding:"required"`         // 12 | 6
	NumberDriversID   int                    `json:"number_drivers_id" binding:"required"` // 0 | 5
	AcceptLanguage    string                 `json:"accept_language"`
	VehicleGroupExtID *int                   `json:"vehicle_group_external_id,omitempty"`
	EuroasiaBody      map[string]interface{} `json:"euroasia_body"` // тело CalculateOsagoRequest без vehicle_group_id
}

// CalcAll считает ОСАГО сразу во всех провайдерах и возвращает агрегированный ответ.
func (c *OsagoProviderController) CalcAll(ctx *gin.Context) {
	var payload CalcAllPayload
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("invalid request: %v", err)})
		return
	}

	// --- NEO часть (повторяет CalcNeo) ---
	neoPeriodID := mapPeriodToNeo(payload.PeriodID)
	if neoPeriodID == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid period_id: allowed 12 or 6"})
		return
	}
	neoDriversID := mapDriversToNeo(payload.NumberDriversID)
	if neoDriversID == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid number_drivers_id: allowed 0 (unlimited) or 5"})
		return
	}

	jReq := neoJuridikRequest{
		GosNumber:  payload.GosNumber,
		TechSery:   payload.TechSery,
		TechNumber: payload.TechNumber,
	}
	jResp, neoJErr := c.callNeoJuridikAPI(jReq)

	var neoResult gin.H
	if neoJErr != nil {
		neoResult = gin.H{
			"provider": "neo",
			"success":  false,
			"message":  neoJErr.Error(),
		}
	} else {
		neoReq := map[string]interface{}{
			"gos_number":        payload.GosNumber,
			"tech_sery":         payload.TechSery,
			"tech_number":       payload.TechNumber,
			"period_id":         neoPeriodID,
			"number_drivers_id": neoDriversID,
			"car_type_id":       jResp.VehicleTypeID,
		}
		neoResp, neoErr := c.callNeoCalcAPI(neoReq)
		if neoErr != nil {
			neoResult = gin.H{
				"provider": "neo",
				"success":  false,
				"message":  neoErr.Error(),
			}
		} else {
			neoResult = gin.H{
				"provider": "neo",
				"success":  neoResp.Error == 0 && neoResp.Result,
				"message":  neoResp.Message,
				"data": map[string]interface{}{
					"amount_uzs": neoResp.Response.AmountUZS,
					"inn":        neoResp.Response.Inn,
					"juridik":    jResp,
				},
			}
		}
	}

	// --- Euroasia часть (похожа на CalcEuroasia + unified vehicle_group_external_id) ---
	var euroasiaResult gin.H
	if c.config.EuroasiaBaseURL == "" || c.config.EuroasiaAPIKey == "" {
		euroasiaResult = gin.H{
			"provider": "euroasia",
			"success":  false,
			"message":  "Euroasia config missing (EUROASIA_BASE_URL or EUROASIA_API_KEY)",
		}
	} else if payload.EuroasiaBody == nil {
		euroasiaResult = gin.H{
			"provider": "euroasia",
			"success":  false,
			"message":  "euroasia_body is required",
		}
	} else {
		lang := payload.AcceptLanguage
		if lang == "" {
			lang = "ru"
		}

		body := payload.EuroasiaBody
		if payload.VehicleGroupExtID != nil {
			groups, err := c.callEuroasiaVehicleGroupsAPI(lang)
			if err != nil {
				euroasiaResult = gin.H{
					"provider": "euroasia",
					"success":  false,
					"message":  fmt.Sprintf("failed to fetch Euroasia vehicle groups: %v", err),
				}
			} else {
				var vgID string
				for _, g := range groups {
					if g.ExternalID == *payload.VehicleGroupExtID {
						vgID = g.ID
						break
					}
				}
				if vgID == "" {
					euroasiaResult = gin.H{
						"provider": "euroasia",
						"success":  false,
						"message":  fmt.Sprintf("vehicle_group_external_id %d not found in Euroasia", *payload.VehicleGroupExtID),
					}
				} else {
					body["vehicle_group_id"] = vgID
				}
			}
		}

		if euroasiaResult == nil {
			b, err := json.Marshal(body)
			if err != nil {
				euroasiaResult = gin.H{
					"provider": "euroasia",
					"success":  false,
					"message":  fmt.Sprintf("marshal euroasia_body: %v", err),
				}
			} else {
				url := strings.TrimRight(c.config.EuroasiaBaseURL, "/") + "/api/v1/insurance/osago/calculate"
				req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
				if err != nil {
					euroasiaResult = gin.H{
						"provider": "euroasia",
						"success":  false,
						"message":  fmt.Sprintf("create request: %v", err),
					}
				} else {
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Accept-Language", lang)
					req.Header.Set("Authorization", c.config.EuroasiaAPIKey)

					resp, err := c.client.Do(req)
					if err != nil {
						euroasiaResult = gin.H{
							"provider": "euroasia",
							"success":  false,
							"message":  err.Error(),
						}
					} else {
						defer resp.Body.Close()
						bodyBytes, _ := io.ReadAll(resp.Body)
						euroasiaResult = gin.H{
							"provider": "euroasia",
							"success":  resp.StatusCode == http.StatusOK,
							"raw":      json.RawMessage(bodyBytes),
						}
					}
				}
			}
		}
	}

	// --- Gross часть: пока заглушка ---
	grossResult := gin.H{
		"provider": "gross",
		"success":  false,
		"message":  "Gross calculation endpoint is not defined. Provide Gross calc API to enable.",
	}

	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OSAGO calculation for all providers",
		"data": gin.H{
			"neo":      neoResult,
			"euroasia": euroasiaResult,
			"gross":    grossResult,
		},
	})
}

// ===== Euroasia: calculate =====

type euroasiaCalcPayload struct {
	AcceptLanguage         string                 `json:"accept_language"`                     // optional, default "ru"
	VehicleGroupExternalID *int                   `json:"vehicle_group_external_id,omitempty"` // единый ID (external_id из Euroasia), общий для всех провайдеров
	Body                   map[string]interface{} `json:"body" binding:"required"`             // полное тело CalculateOsagoRequest
}

// CalcEuroasia проксирует запрос на /api/v1/insurance/osago/calculate Euroasia.
func (c *OsagoProviderController) CalcEuroasia(ctx *gin.Context) {
	var payload euroasiaCalcPayload
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("invalid request: %v", err)})
		return
	}
	if c.config.EuroasiaBaseURL == "" || c.config.EuroasiaAPIKey == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Euroasia config missing (EUROASIA_BASE_URL or EUROASIA_API_KEY)"})
		return
	}
	lang := payload.AcceptLanguage
	if lang == "" {
		lang = "ru"
	}

	// Если пришел единый vehicle_group_external_id — маппим его в конкретный vehicle_group_id Euroasia.
	if payload.VehicleGroupExternalID != nil {
		groups, err := c.callEuroasiaVehicleGroupsAPI(lang)
		if err != nil {
			ctx.JSON(http.StatusBadGateway, gin.H{"success": false, "message": fmt.Sprintf("failed to fetch Euroasia vehicle groups: %v", err)})
			return
		}
		var vgID string
		for _, g := range groups {
			if g.ExternalID == *payload.VehicleGroupExternalID {
				vgID = g.ID
				break
			}
		}
		if vgID == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("vehicle_group_external_id %d not found in Euroasia", *payload.VehicleGroupExternalID),
			})
			return
		}
		if payload.Body == nil {
			payload.Body = make(map[string]interface{})
		}
		// Перезаписываем/устанавливаем vehicle_group_id в теле запроса Euroasia.
		payload.Body["vehicle_group_id"] = vgID
	}

	b, err := json.Marshal(payload.Body)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("marshal body: %v", err)})
		return
	}
	url := strings.TrimRight(c.config.EuroasiaBaseURL, "/") + "/api/v1/insurance/osago/calculate"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": fmt.Sprintf("create request: %v", err)})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", lang)
	req.Header.Set("Authorization", c.config.EuroasiaAPIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// Возвращаем “сырые” данные провайдера, чтобы можно было их проинспектировать.
	ctx.JSON(resp.StatusCode, gin.H{
		"provider": "euroasia",
		"success":  resp.StatusCode == http.StatusOK,
		"raw":      json.RawMessage(body),
	})
}

// ===== GROSS: calculate (TODO) =====

// CalcGross — пока заглушка: в текущих интеграциях Gross не предоставляет отдельного
// расчётного эндпоинта. Нужен URL/спека для калькуляции. Сейчас возвращаем 501.
func (c *OsagoProviderController) CalcGross(ctx *gin.Context) {
	ctx.JSON(http.StatusNotImplemented, gin.H{
		"provider": "gross",
		"success":  false,
		"message":  "Gross calculation endpoint is not defined. Provide Gross calc API to enable.",
	})
}
