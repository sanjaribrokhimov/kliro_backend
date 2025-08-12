package insurance

import (
	"encoding/base64"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"kliro/config"
)

type OsagoController struct {
	cfg *config.Config
	cl  *http.Client
}

func NewOsagoController(cfg *config.Config) *OsagoController {
	return &OsagoController{
		cfg: cfg,
		cl:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (oc *OsagoController) basicAuthHeader() string {
	creds := oc.cfg.NeoLogin + ":" + oc.cfg.NeoPassword
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(creds))
}

func (oc *OsagoController) proxyRequest(c *gin.Context, method string, externalPath string) {
	url := oc.cfg.NeoBaseURL + externalPath

	var body io.Reader
	if method == http.MethodPost || method == http.MethodPut {
		body = c.Request.Body
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
		return
	}
	if method == http.MethodPost || method == http.MethodPut {
		if ct := c.GetHeader("Content-Type"); ct != "" {
			req.Header.Set("Content-Type", ct)
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	req.Header.Set("Authorization", oc.basicAuthHeader())

	resp, err := oc.cl.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "external api error", "details": err.Error()})
		return
	}
	defer resp.Body.Close()

	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func (oc *OsagoController) Calc(c *gin.Context) {
	oc.proxyRequest(c, http.MethodPost, "/api/osago-neo/get-calc-osago")
}
func (oc *OsagoController) Juridik(c *gin.Context) {
	oc.proxyRequest(c, http.MethodPost, "/api/osago-neo/osago-juridik")
}
func (oc *OsagoController) CheckPerson(c *gin.Context) {
	oc.proxyRequest(c, http.MethodPost, "/api/osago-neo/check-person")
}
func (oc *OsagoController) SavePolicy(c *gin.Context) {
	oc.proxyRequest(c, http.MethodPost, "/api/osago-neo/save-policy/v2")
}
func (oc *OsagoController) ConfirmPolicy(c *gin.Context) {
	oc.proxyRequest(c, http.MethodPost, "/api/osago-neo/confirm-policy")
}
func (oc *OsagoController) ConfirmCheck(c *gin.Context) {
	oc.proxyRequest(c, http.MethodPost, "/api/osago-neo/confirm-check")
}

// Add OSAGO payment endpoints
func (oc *OsagoController) GetPaymentLink(c *gin.Context) {
	oc.proxyRequest(c, http.MethodPost, "/api/osago-neo/get-payment-link")
}
func (oc *OsagoController) CheckPayment(c *gin.Context) {
	oc.proxyRequest(c, http.MethodPost, "/api/osago-neo/check_tolov")
}
