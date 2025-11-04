package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// PaymentMulticardUIController — вспомогательные эндпоинты для фронтенда (редиректы/проверки)
// Не изменяет «сырые» прокси и не меняет контрактов Multicard.
type PaymentMulticardUIController struct {
	baseURL string
	appID   string
	secret  string
	token   string
	expiry  time.Time
	http    *http.Client
	// целевые URL для редиректов после оплаты
	successURL string
	errorURL   string
}

func NewPaymentMulticardUIController() *PaymentMulticardUIController {
	base := os.Getenv("MULTICARD_BASE_URL")
	if base == "" {
		base = "https://dev-mesh.multicard.uz"
	}
	return &PaymentMulticardUIController{
		baseURL:    base,
		appID:      os.Getenv("MULTICARD_APPLICATION_ID"),
		secret:     os.Getenv("MULTICARD_SECRET"),
		http:       &http.Client{Timeout: 30 * time.Second},
		successURL: os.Getenv("FRONTEND_PAYMENT_SUCCESS_URL"),
		errorURL:   os.Getenv("FRONTEND_PAYMENT_ERROR_URL"),
	}
}

func (pc *PaymentMulticardUIController) ensureToken() error {
	if pc.token != "" && time.Now().Before(pc.expiry.Add(-5*time.Minute)) {
		return nil
	}
	reqBody := map[string]string{"application_id": pc.appID, "secret": pc.secret}
	b, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", pc.baseURL+"/auth", bytesReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := pc.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth failed: %d", resp.StatusCode)
	}
	var res struct{ Token, Expiry string }
	_ = json.NewDecoder(resp.Body).Decode(&res)
	pc.token = res.Token
	if t, err := time.Parse("2006-01-02 15:04:05", res.Expiry); err == nil {
		pc.expiry = t
	} else {
		pc.expiry = time.Now().Add(55 * time.Minute)
	}
	return nil
}

// bytesReader — минимальная обёртка, чтобы не тянуть bytes во все места
func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }

// GetStatus — удобный эндпоинт для фронта: вернуть статус по uuid или invoice_id через Multicard
// GET /payment/ui/status?uuid=...&invoice_id=...
func (pc *PaymentMulticardUIController) GetStatus(c *gin.Context) {
	uuid := c.Query("uuid")
	if uuid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uuid is required"})
		return
	}
	if err := pc.ensureToken(); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "auth failed"})
		return
	}
	req, _ := http.NewRequest("GET", pc.baseURL+"/payment/invoice/"+uuid, nil)
	req.Header.Set("Authorization", "Bearer "+pc.token)
	req.Header.Set("X-Access-Token", pc.token)
	resp, err := pc.http.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	// пробрасываем статус и тело как есть
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

// Return — публичная точка для return_url из Multicard. Делает проверку статуса и редиректит на фронт.
// GET /payment/return?uuid=...&invoice_id=...
func (pc *PaymentMulticardUIController) Return(c *gin.Context) {
	pc.redirectWithStatus(c, true)
}

// Error — публичная точка для return_error_url из Multicard. Редиректит на фронт со статусом error.
// GET /payment/error?uuid=...&invoice_id=...
func (pc *PaymentMulticardUIController) Error(c *gin.Context) {
	pc.redirectWithStatus(c, false)
}

func (pc *PaymentMulticardUIController) redirectWithStatus(c *gin.Context, check bool) {
	uuid := c.Query("uuid")
	invoiceID := c.Query("invoice_id")

	status := "unknown"
	if check && uuid != "" {
		if err := pc.ensureToken(); err == nil {
			req, _ := http.NewRequest("GET", pc.baseURL+"/payment/invoice/"+uuid, nil)
			req.Header.Set("Authorization", "Bearer "+pc.token)
			req.Header.Set("X-Access-Token", pc.token)
			resp, err := pc.http.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				var obj struct {
					Success bool
					Data    map[string]interface{}
				}
				_ = json.NewDecoder(resp.Body).Decode(&obj)
				if obj.Data != nil {
					if s, ok := obj.Data["payment"].(map[string]interface{}); ok {
						if st, ok := s["status"].(string); ok {
							status = st
						}
					}
				}
			}
		}
	}

	target := pc.successURL
	if target == "" {
		target = "https://kliro.uz/payment/result"
	}
	if !check { // для error-ветки используем errorURL, если задан
		if pc.errorURL != "" {
			target = pc.errorURL
		}
	}
	q := url.Values{}
	if uuid != "" {
		q.Set("uuid", uuid)
	}
	if invoiceID != "" {
		q.Set("invoice_id", invoiceID)
	}
	if status != "" {
		q.Set("status", status)
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("%s?%s", target, q.Encode()))
}
