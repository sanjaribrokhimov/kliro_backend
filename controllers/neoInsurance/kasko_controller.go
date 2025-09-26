package insurance

import (
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"kliro/config"
)

type KaskoController struct {
	cfg *config.Config
	cl  *http.Client
}

func NewKaskoController(cfg *config.Config) *KaskoController {
	return &KaskoController{
		cfg: cfg,
		cl:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (kc *KaskoController) basicAuthHeader() string {
	creds := kc.cfg.NeoLogin + ":" + kc.cfg.NeoPassword
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

func (kc *KaskoController) proxyRequest(c *gin.Context, method string, externalPath string) {
	url := kc.cfg.NeoBaseURL + externalPath

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
	req.Header.Set("Authorization", kc.basicAuthHeader())

	resp, err := kc.cl.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "external api error", "details": err.Error()})
		return
	}
	defer resp.Body.Close()

	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func (kc *KaskoController) Cars(c *gin.Context) {
	kc.proxyRequest(c, http.MethodGet, "/api/sayt/kasko/cars")
}
func (kc *KaskoController) GetTarif(c *gin.Context) {
	kc.proxyRequest(c, http.MethodGet, "/api/kasko-neo/get-tarif")
}
func (kc *KaskoController) CarPriceCalc(c *gin.Context) {
	kc.proxyRequest(c, http.MethodPost, "/api/kasko-neo/car_price_cal")
}
func (kc *KaskoController) Calculate(c *gin.Context) {
	kc.proxyRequest(c, http.MethodPost, "/api/kasko-neo/hisoblash")
}
func (kc *KaskoController) Save(c *gin.Context) {
	kc.proxyRequest(c, http.MethodPost, "/api/kasko-neo/saqlash")
}
func (kc *KaskoController) GetPaymentLink(c *gin.Context) {
	kc.proxyRequest(c, http.MethodPost, "/api/kasko-neo/get-payment-link")
}
func (kc *KaskoController) CheckPayment(c *gin.Context) {
	kc.proxyRequest(c, http.MethodPost, "/api/kasko-neo/check_tolov")
}

// ImageUpload handles multipart photo upload for KASKO
func (kc *KaskoController) ImageUpload(c *gin.Context) {
	kc.proxyRequest(c, http.MethodPost, "/api/kasko-neo/image-upload")
}
