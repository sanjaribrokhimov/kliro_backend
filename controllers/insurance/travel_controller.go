package insurance

import (
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"kliro/config"
)

type TravelController struct {
	cfg *config.Config
	cl  *http.Client
}

func NewTravelController(cfg *config.Config) *TravelController {
	return &TravelController{
		cfg: cfg,
		cl:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (tc *TravelController) basicAuthHeader() string {
	creds := tc.cfg.NeoLogin + ":" + tc.cfg.NeoPassword
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

func (tc *TravelController) proxyRequest(c *gin.Context, method string, externalPath string) {
	url := tc.cfg.NeoBaseURL + externalPath

	var body io.Reader
	if method == http.MethodPost || method == http.MethodPut {
		body = c.Request.Body
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
		return
	}
	// Preserve Content-Length for multipart/form-data uploads
	req.ContentLength = c.Request.ContentLength
	if method == http.MethodPost || method == http.MethodPut {
		if ct := c.GetHeader("Content-Type"); ct != "" {
			req.Header.Set("Content-Type", ct)
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	req.Header.Set("Authorization", tc.basicAuthHeader())

	resp, err := tc.cl.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "external api error", "details": err.Error()})
		return
	}
	defer resp.Body.Close()

	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

// Risk-Travel (simplified product)
func (tc *TravelController) RiskGetData(c *gin.Context) {
	tc.proxyRequest(c, http.MethodGet, "/api/travel-risk-neo/get-data")
}
func (tc *TravelController) RiskGetCountry(c *gin.Context) {
	tc.proxyRequest(c, http.MethodGet, "/api/accident_one_day-neo/get-country")
}
func (tc *TravelController) RiskCalculator(c *gin.Context) {
	tc.proxyRequest(c, http.MethodPost, "/api/travel-risk-neo/calculator")
}
func (tc *TravelController) RiskSave(c *gin.Context) {
	tc.proxyRequest(c, http.MethodPost, "/api/travel-risk-neo/save")
}

// Full Travel API + risk
func (tc *TravelController) TravelGetData(c *gin.Context) {
	tc.proxyRequest(c, http.MethodGet, "/api/travel-neo/get-data")
}
func (tc *TravelController) TravelCalculatorTotal(c *gin.Context) {
	tc.proxyRequest(c, http.MethodPost, "/api/travel-neo/calculator-total")
}
func (tc *TravelController) TravelSavePolis(c *gin.Context) {
	tc.proxyRequest(c, http.MethodPost, "/api/travel-neo/save-polis")
}
func (tc *TravelController) TravelCheckPolis(c *gin.Context) {
	tc.proxyRequest(c, http.MethodPost, "/api/travel-neo/checkPolis")
}
func (tc *TravelController) TravelPassportPerson(c *gin.Context) {
	tc.proxyRequest(c, http.MethodPost, "/api/travel-neo/passport-person")
}
