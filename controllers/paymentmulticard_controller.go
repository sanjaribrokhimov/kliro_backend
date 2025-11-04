package controllers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// PaymentMulticardController — прозрачный прокси для платёжной страницы Multicard
type PaymentMulticardController struct {
	baseURL string
	appID   string
	secret  string
	token   string
	expiry  time.Time
	client  *http.Client
}

func NewPaymentMulticardController() *PaymentMulticardController {
	base := os.Getenv("MULTICARD_BASE_URL")
	if base == "" {
		base = "https://dev-mesh.multicard.uz"
	}
	return &PaymentMulticardController{
		baseURL: base,
		appID:   os.Getenv("MULTICARD_APPLICATION_ID"),
		secret:  os.Getenv("MULTICARD_SECRET"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// ensureToken получает и кэширует X-Access-Token у Multicard на стороне бэкенда
func (pc *PaymentMulticardController) ensureToken() error {
	if pc.token != "" && time.Now().Before(pc.expiry.Add(-5*time.Minute)) {
		return nil
	}

	u := pc.baseURL + "/auth"
	payload := map[string]string{
		"application_id": pc.appID,
		"secret":         pc.secret,
	}
	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", u, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := pc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return err
	}
	var res struct {
		Token  string `json:"token"`
		Expiry string `json:"expiry"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	pc.token = res.Token
	if t, err := time.Parse("2006-01-02 15:04:05", res.Expiry); err == nil {
		pc.expiry = t
	} else {
		pc.expiry = time.Now().Add(55 * time.Minute)
	}
	return nil
}

// proxyRaw проксирует запрос как есть к Multicard и возвращает сырой ответ и статус код как у источника
func (pc *PaymentMulticardController) proxyRaw(c *gin.Context, targetPath string) {
	// Собираем полный URL
	u, err := url.Parse(pc.baseURL)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	u.Path = targetPath
	u.RawQuery = c.Request.URL.RawQuery

	// Читаем сырое тело
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	_ = c.Request.Body.Close()

	// Создаём новый запрос к апстриму
	upReq, err := http.NewRequest(c.Request.Method, u.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}

	// Авторизация на стороне бэкенда
	if err := pc.ensureToken(); err != nil {
		c.Status(http.StatusBadGateway)
		return
	}

	// Ставим только необходимые заголовки: тип контента и токен
	if ct := c.GetHeader("Content-Type"); ct != "" {
		upReq.Header.Set("Content-Type", ct)
	} else {
		upReq.Header.Set("Content-Type", "application/json")
	}
	if accept := c.GetHeader("Accept"); accept != "" {
		upReq.Header.Set("Accept", accept)
	}
	upReq.Header.Set("X-Access-Token", pc.token)
	upReq.Header.Set("Authorization", "Bearer "+pc.token)

	resp, err := pc.client.Do(upReq)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Пробрасываем статус код апстрима и тело как есть
	for k, v := range resp.Header {
		for _, vv := range v {
			c.Header(k, vv)
		}
	}
	c.Status(resp.StatusCode)
	c.Writer.Write(respBody)
}

// CreateInvoice — POST /payment/invoice
func (pc *PaymentMulticardController) CreateInvoice(c *gin.Context) {
	// Читаем тело запроса
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	// Валидация invoice_id: avia{7 цифр} или hotel{7 цифр}
	invoiceID, _ := req["invoice_id"].(string)
	re := regexp.MustCompile(`^(avia|hotel)\d{7}$`)
	if !re.MatchString(invoiceID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid invoice_id format; expected aviaXXXXXXX or hotelXXXXXXX"})
		return
	}

	// Извлекаем amount (как целое)
	var amountInt int64
	switch v := req["amount"].(type) {
	case float64:
		amountInt = int64(v)
	case string:
		if p, err := strconv.ParseInt(v, 10, 64); err == nil {
			amountInt = p
		}
	}
	if amountInt <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be positive integer"})
		return
	}

	// Сформировать OFD на бэкенде по префиксу invoice_id
	ofdItem := map[string]interface{}{
		"qty":          1,
		"price":        amountInt,
		"total":        amountInt,
		"mxik":         "",
		"package_code": "",
		"name":         "",
	}
	if strings.HasPrefix(invoiceID, "avia") {
		ofdItem["mxik"] = "11199002021000000"
		ofdItem["package_code"] = "1506591"
		ofdItem["name"] = "AVIA TICKET"
	} else if strings.HasPrefix(invoiceID, "hotel") {
		ofdItem["mxik"] = "10204001001000000"
		ofdItem["package_code"] = "1500169"
		ofdItem["name"] = "HOTEL BOOKING"
	}
	// Принудительно подменяем ofd независимо от входного тела
	ofd := []map[string]interface{}{ofdItem}

	// Разрешённые поля тела, остальные отбрасываем
	filtered := map[string]interface{}{}
	// store_id всегда берём из .env
	envStoreID := os.Getenv("MULTICARD_STORE_ID")
	if envStoreID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "MULTICARD_STORE_ID is not configured"})
		return
	}
	filtered["store_id"] = envStoreID
	filtered["amount"] = amountInt
	filtered["invoice_id"] = invoiceID
	if v, ok := req["return_url"].(string); ok {
		filtered["return_url"] = v
	}
	if v, ok := req["callback_url"].(string); ok {
		filtered["callback_url"] = v
	}
	if v, ok := req["lang"].(string); ok {
		filtered["lang"] = v
	}
	filtered["ofd"] = ofd

	// Авторизация к Multicard
	if err := pc.ensureToken(); err != nil {
		c.Status(http.StatusBadGateway)
		return
	}

	// Отправляем в Multicard
	u := pc.baseURL + "/payment/invoice"
	b, _ := json.Marshal(filtered)
	upReq, _ := http.NewRequest("POST", u, bytes.NewReader(b))
	ct := c.GetHeader("Content-Type")
	if ct == "" {
		ct = "application/json"
	}
	upReq.Header.Set("Content-Type", ct)
	if accept := c.GetHeader("Accept"); accept != "" {
		upReq.Header.Set("Accept", accept)
	}
	upReq.Header.Set("Authorization", "Bearer "+pc.token)
	upReq.Header.Set("X-Access-Token", pc.token)

	resp, err := pc.client.Do(upReq)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	// Пробрасываем заголовки ответа
	for k, v := range resp.Header {
		for _, vv := range v {
			c.Header(k, vv)
		}
	}
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

// GetInvoice — GET /payment/invoice/:uuid
func (pc *PaymentMulticardController) GetInvoice(c *gin.Context) {
	pc.proxyRaw(c, "/payment/invoice/"+c.Param("uuid"))
}

// DeleteInvoice — DELETE /payment/invoice/:uuid
func (pc *PaymentMulticardController) DeleteInvoice(c *gin.Context) {
	pc.proxyRaw(c, "/payment/invoice/"+c.Param("uuid"))
}

// QuickPay — PUT /payment/:uuid/scanpay
func (pc *PaymentMulticardController) QuickPay(c *gin.Context) {
	pc.proxyRaw(c, "/payment/"+c.Param("uuid")+"/scanpay")
}

// CallbackSuccess — POST /payment/callback/success (эхо без изменений)
func (pc *PaymentMulticardController) CallbackSuccess(c *gin.Context) {
	// Возвращаем 200 и то же тело без обработки
	body, _ := io.ReadAll(c.Request.Body)
	_ = c.Request.Body.Close()
	c.Data(http.StatusOK, c.ContentType(), body)
}

// CallbackWebhooks — POST /payment/callback/webhooks (эхо без изменений)
func (pc *PaymentMulticardController) CallbackWebhooks(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	_ = c.Request.Body.Close()
	c.Data(http.StatusOK, c.ContentType(), body)
}
