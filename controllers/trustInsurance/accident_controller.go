package trustInsurance

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	ID               int     `json:"id"`
	InsurancePremium float64 `json:"insurance_premium"`
	InsuranceOtv     float64 `json:"insurance_otv"`
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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read response"})
		return
	}

	fmt.Printf("Trust API Response Status: %d\n", resp.StatusCode)
	fmt.Printf("Trust API Response Body: %s\n", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "external api returned non-200 status", "details": string(bodyBytes)})
		return
	}

	var tariffsResp TariffsResponse
	if err := json.Unmarshal(bodyBytes, &tariffsResp); err != nil {
		fmt.Printf("Failed to decode response: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode response", "details": err.Error(), "raw_response": string(bodyBytes)})
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
	Result           int     `json:"result"`
	ResultMessage    string  `json:"result_message"`
	AnketaID         int     `json:"anketa_id"`
	InsurancePremium float64 `json:"insurance_premium"`
	InsuranceOtv     float64 `json:"insurance_otv"`
}

type PaymentURLs struct {
	Click string `json:"click"`
	Payme string `json:"payme"`
}

type CreateResponseWithPayment struct {
	Result           int         `json:"result"`
	ResultMessage    string      `json:"result_message"`
	AnketaID         int         `json:"anketa_id"`
	InsurancePremium float64     `json:"insurance_premium"`
	InsuranceOtv     float64     `json:"insurance_otv"`
	PaymentURLs      PaymentURLs `json:"payment_urls"`
}

type CheckPaymentRequest struct {
	AnketaID int    `json:"anketa_id" binding:"required"`
	Lan      string `json:"lan" binding:"required"`
}

type CheckPaymentResponse struct {
	Result        int    `json:"result"`
	ResultMessage string `json:"result_message"`
	PolicyID      string `json:"policy_id"`
	PolicySery    string `json:"policy_sery"`
	PolicyNumber  int    `json:"policy_number"`
	StatusPolicy  string `json:"status_policy"`
	URL           string `json:"url"`
	URLNapp       string `json:"url_napp"`
	StatusPayment string `json:"status_payment"`
	PaymentType   string `json:"payment_type"`
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

	fmt.Printf("Trust Create API Response Status: %d\n", resp.StatusCode)
	fmt.Printf("Trust Create API Response Body: %s\n", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "external api returned non-200 status", "details": string(bodyBytes)})
		return
	}

	var createResp CreateResponse
	if err := json.Unmarshal(bodyBytes, &createResp); err != nil {
		fmt.Printf("Failed to decode create response: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode response", "details": err.Error(), "raw_response": string(bodyBytes)})
		return
	}

	fmt.Printf("Parsed Create Response: AnketaID=%d, Premium=%.0f\n", createResp.AnketaID, createResp.InsurancePremium)

	premiumTiyin := fmt.Sprintf("%.0f", createResp.InsurancePremium*100)

	clickURL := fmt.Sprintf("https://my.click.uz/services/pay?service_id=23572&merchant_id=14417&amount=%.0f&transaction_param=%d&return_url=https://kliro.uz/payment/return",
		createResp.InsurancePremium, createResp.AnketaID)

	paymeString := fmt.Sprintf("m=646c8bff2cb83937a7551c95;ac.order_id=%d;a=%s", createResp.AnketaID, premiumTiyin)
	paymeEncoded := base64.StdEncoding.EncodeToString([]byte(paymeString))
	paymeURL := fmt.Sprintf("https://checkout.paycom.uz/%s", paymeEncoded)

	fmt.Printf("Generated Click URL: %s\n", clickURL)
	fmt.Printf("Generated Payme URL: %s\n", paymeURL)

	responseWithPayment := CreateResponseWithPayment{
		Result:           createResp.Result,
		ResultMessage:    createResp.ResultMessage,
		AnketaID:         createResp.AnketaID,
		InsurancePremium: createResp.InsurancePremium,
		InsuranceOtv:     createResp.InsuranceOtv,
		PaymentURLs: PaymentURLs{
			Click: clickURL,
			Payme: paymeURL,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"result": responseWithPayment,
	})
}

func (ac *AccidentController) CheckPayment(c *gin.Context) {
	var req CheckPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	url := ac.cfg.TrustBaseURL + "/api/payments/check"

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

	var checkResp CheckPaymentResponse
	if err := json.Unmarshal(bodyBytes, &checkResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode response"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": checkResp,
	})
}
