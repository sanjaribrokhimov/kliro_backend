package trustInsurance

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"kliro/config"
)

type AccidentController struct {
	cfg *config.Config
	cl  *http.Client
}

func NewAccidentController(cfg *config.Config) *AccidentController {
	return &AccidentController{
		cfg: cfg,
		cl:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (ac *AccidentController) basicAuthHeader() string {
	creds := ac.cfg.TrustLogin + ":" + ac.cfg.TrustPassword
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

type Tariff struct {
	ID               int `json:"id"`
	InsurancePremium int `json:"insurance_premium"`
	InsuranceOtv     int `json:"insurance_otv"`
}

type TariffsResponse struct {
	Result        int      `json:"result"`
	ResultMessage string   `json:"result_message"`
	Tariffs       []Tariff `json:"tariffs"`
}

func (ac *AccidentController) GetTariffs(c *gin.Context) {
	url := ac.cfg.TrustBaseURL + "/api/v1/accident/tariff"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
		return
	}

	req.Header.Set("Authorization", ac.basicAuthHeader())

	resp, err := ac.cl.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "external api error", "details": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "external api returned non-200 status"})
		return
	}

	var tariffsResp TariffsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tariffsResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": tariffsResp,
	})
}

type PersonData struct {
	Pinfl        string `json:"pinfl" binding:"required"`
	PassSery     string `json:"pass_sery" binding:"required"`
	PassNum      string `json:"pass_num" binding:"required"`
	DateBirth    string `json:"date_birth" binding:"required"`
	LastName     string `json:"last_name" binding:"required"`
	FirstName    string `json:"first_name" binding:"required"`
	PatronymName string `json:"patronym_name" binding:"required"`
	Oblast       int    `json:"oblast" binding:"required"`
	Rayon        int    `json:"rayon" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Address      string `json:"address" binding:"required"`
}

type CreateRequest struct {
	StartDate string     `json:"start_date" binding:"required"`
	TariffID  int        `json:"tariff_id" binding:"required"`
	Person    PersonData `json:"person" binding:"required"`
}

type CreateResponse struct {
	Result           int    `json:"result"`
	ResultMessage    string `json:"result_message"`
	AnketaID         int    `json:"anketa_id"`
	InsurancePremium int    `json:"insurance_premium"`
	InsuranceOtv     int    `json:"insurance_otv"`
}

func (ac *AccidentController) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	url := ac.cfg.TrustBaseURL + "/api/v1/accident/create"

	jsonData, err := json.Marshal(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to marshal request"})
		return
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", ac.basicAuthHeader())

	resp, err := ac.cl.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "external api error", "details": err.Error()})
		return
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read response"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "external api returned non-200 status", "details": string(bodyBytes)})
		return
	}

	var createResp CreateResponse
	if err := json.Unmarshal(bodyBytes, &createResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": createResp,
	})
}
