package controllers

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	baseURL         string
	appID           string
	secret          string
	token           string
	expiry          time.Time
	tokenLastRefresh time.Time
	client          *http.Client
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
	now := time.Now()
	
	// Проверяем, нужно ли обновить токен:
	// 1. Если токена нет
	// 2. Если токен истекает в течение 5 минут
	// 3. Если прошло более 24 часов с последнего обновления
	shouldRefresh := pc.token == "" || 
		now.Add(5*time.Minute).After(pc.expiry) || 
		now.Sub(pc.tokenLastRefresh) >= 24*time.Hour

	if !shouldRefresh {
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
		return fmt.Errorf("auth status %d", resp.StatusCode)
	}
	var res struct {
		Token  string `json:"token"`
		Expiry string `json:"expiry"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	pc.token = res.Token
	pc.tokenLastRefresh = now
	if t, err := time.Parse("2006-01-02 15:04:05", res.Expiry); err == nil {
		pc.expiry = t
	} else {
		pc.expiry = now.Add(55 * time.Minute)
	}
	return nil
}

// refreshToken принудительно обновляет токен (используется при ошибках авторизации)
func (pc *PaymentMulticardController) refreshToken() error {
	pc.token = ""
	pc.expiry = time.Time{}
	pc.tokenLastRefresh = time.Time{}
	return pc.ensureToken()
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

	// Авторизация на стороне бэкенда
	if err := pc.ensureToken(); err != nil {
		c.Status(http.StatusBadGateway)
		return
	}

	// Функция для выполнения запроса
	doRequest := func() (*http.Response, error) {
		upReq, err := http.NewRequest(c.Request.Method, u.String(), bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, err
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

		return pc.client.Do(upReq)
	}

	resp, err := doRequest()
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Если получили ошибку авторизации (401), обновляем токен и повторяем запрос
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		
		// Принудительно обновляем токен
		if err := pc.refreshToken(); err != nil {
			c.Status(http.StatusBadGateway)
			return
		}

		// Повторяем запрос с новым токеном
		resp, err = doRequest()
		if err != nil {
			c.Status(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
	}

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
	// application_id всегда берём из .env
	if pc.appID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "MULTICARD_APPLICATION_ID is not configured"})
		return
	}
	filtered["application_id"] = pc.appID
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

	// Формируем split массив на основе invoice_id
	splitArray := pc.buildSplitArray(invoiceID, amountInt)
	if len(splitArray) > 0 {
		filtered["split"] = splitArray
	}

	// Авторизация к Multicard
	if err := pc.ensureToken(); err != nil {
		c.Status(http.StatusBadGateway)
		return
	}

	// Отправляем в Multicard
	u := pc.baseURL + "/payment/invoice"
	b, _ := json.Marshal(filtered)
	
	// Функция для выполнения запроса
	doRequest := func() (*http.Response, error) {
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

		return pc.client.Do(upReq)
	}

	resp, err := doRequest()
	if err != nil {
		c.Status(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Если получили ошибку авторизации (401), обновляем токен и повторяем запрос
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		
		// Принудительно обновляем токен
		if err := pc.refreshToken(); err != nil {
			c.Status(http.StatusBadGateway)
			return
		}

		// Повторяем запрос с новым токеном
		resp, err = doRequest()
		if err != nil {
			c.Status(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
	}

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
	// Ограничение по IP включаем только если явно задано MULTICARD_CALLBACK_IP_CHECK_ENABLE=1
	allowedIP := "195.158.26.90"
	if os.Getenv("MULTICARD_CALLBACK_IP_CHECK_ENABLE") == "1" {
		ip := c.ClientIP()
		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			if idx := strings.Index(xff, ","); idx > 0 {
				ip = strings.TrimSpace(xff[:idx])
			} else {
				ip = strings.TrimSpace(xff)
			}
		}
		if ip != allowedIP && ip != "127.0.0.1" && ip != "::1" {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "forbidden ip"})
			return
		}
	}

	// Читаем JSON
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid json"})
		return
	}

	// Логируем вход в терминал
	fmt.Printf("[multicard-callback] success payload: %+v\n", payload)

	// Проверка подписи включается только если MULTICARD_CALLBACK_SIGN_CHECK_ENABLE=1
	doSignCheck := os.Getenv("MULTICARD_CALLBACK_SIGN_CHECK_ENABLE") == "1"
	secret := os.Getenv("MULTICARD_SECRET")
	if doSignCheck && secret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "server secret not configured"})
		return
	}

	// Пытаемся определить формат подписи. Документация упоминает 2 варианта:
	// md5: {store_id}{invoice_id}{amount}{secret}
	// sha1: {uuid}{invoice_id}{amount}{secret}
	recvSign, _ := payload["sign"].(string)
	amountStr := toString(payload["amount"])     // конвертация к строке без форматирования
	invoiceID := toString(payload["invoice_id"]) // может отсутствовать в success-callback
	if invoiceID == "" {
		invoiceID = toString(payload["store_invoice_id"]) // fallback
	}

	var valid bool
	if doSignCheck && recvSign != "" {
		// md5 вариант
		storeID := toString(payload["store_id"])
		md5sum := md5.Sum([]byte(storeID + invoiceID + amountStr + secret))
		md5hex := hex.EncodeToString(md5sum[:])
		if strings.EqualFold(md5hex, recvSign) {
			valid = true
		}

		// sha1 вариант
		if !valid {
			uuid := toString(payload["uuid"])
			h := sha1.New()
			io.WriteString(h, uuid+invoiceID+amountStr+secret)
			sha1hex := hex.EncodeToString(h.Sum(nil))
			if strings.EqualFold(sha1hex, recvSign) {
				valid = true
			}
		}
		// Лог детальной сверки в терминал
		h := sha1.New()
		io.WriteString(h, toString(payload["uuid"])+invoiceID+amountStr+secret)
		sha1hex := hex.EncodeToString(h.Sum(nil))
		secFP := md5.Sum([]byte(secret))
		fmt.Printf("[multicard-callback] sign recv=%s md5=%s sha1=%s | components: store_id=%s invoice_id=%s amount=%s uuid=%s secret_md5=%s\n", recvSign, md5hex, sha1hex, toString(payload["store_id"]), invoiceID, amountStr, toString(payload["uuid"]), hex.EncodeToString(secFP[:]))
		if !valid {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid sign"})
			return
		}
	}

	// Идемпотентность должна обеспечиваться на стороне БД; здесь просто возвращаем success
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// CallbackWebhooks — POST /payment/callback/webhooks (эхо без изменений)
func (pc *PaymentMulticardController) CallbackWebhooks(c *gin.Context) {
	// Вебхуки статусов: допускаем success на 2xx, валидация подписи включается только если MULTICARD_CALLBACK_SIGN_CHECK_ENABLE=1
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid json"})
		return
	}

	fmt.Printf("[multicard-webhook] payload: %+v\n", payload)

	if os.Getenv("MULTICARD_CALLBACK_SIGN_CHECK_ENABLE") == "1" {
		recvSign, _ := payload["sign"].(string)
		if recvSign != "" {
			secret := os.Getenv("MULTICARD_SECRET")
			amountStr := toString(payload["amount"])
			invoiceID := toString(payload["invoice_id"]) // webhooks обычно присылают invoice_id
			if invoiceID == "" {
				invoiceID = toString(payload["store_invoice_id"]) // fallback на всякий случай
			}
			uuid := toString(payload["uuid"])
			h := sha1.New()
			io.WriteString(h, uuid+invoiceID+amountStr+secret)
			if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), recvSign) {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid sign"})
				return
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// buildSplitArray создает массив split на основе invoice_id и amount
// Для hotel и avia используются разные параметры из .env
// Проценты разделения настраиваются через .env переменные
func (pc *PaymentMulticardController) buildSplitArray(invoiceID string, amount int64) []map[string]interface{} {
	var prefix string
	if strings.HasPrefix(invoiceID, "hotel") {
		prefix = "HOTEL"
	} else if strings.HasPrefix(invoiceID, "avia") {
		prefix = "AVIA"
	} else {
		return nil
	}

	// Читаем процент для первого split из .env
	// Если процент не указан, split не создается
	split1PercentStr := os.Getenv(prefix + "_SPLIT_1_PERCENT")
	if split1PercentStr == "" {
		return nil
	}

	// Парсим процент, если невалидный - не создаем split
	split1Percent, err := strconv.ParseFloat(split1PercentStr, 64)
	if err != nil || split1Percent <= 0 || split1Percent > 100 {
		return nil
	}

	// Читаем параметры для первого split
	split1Type := os.Getenv(prefix + "_SPLIT_1_TYPE")
	split1Recipient := os.Getenv(prefix + "_SPLIT_1_RECIPIENT")
	split1Details := os.Getenv(prefix + "_SPLIT_1_DETAILS")

	// Читаем параметры для второго split
	split2Type := os.Getenv(prefix + "_SPLIT_2_TYPE")
	split2Recipient := os.Getenv(prefix + "_SPLIT_2_RECIPIENT")
	split2Details := os.Getenv(prefix + "_SPLIT_2_DETAILS")

	// Проверяем, что все необходимые параметры заданы
	if split1Type == "" || split1Recipient == "" || split1Details == "" ||
		split2Type == "" || split2Recipient == "" || split2Details == "" {
		return nil
	}

	// Рассчитываем суммы на основе процента из .env
	split1Amount := int64(float64(amount) * split1Percent / 100.0)
	split2Amount := amount - split1Amount // Остаток идет второму split

	// Формируем массив split
	splitArray := []map[string]interface{}{
		{
			"type":      split1Type,
			"recipient": split1Recipient,
			"amount":    split1Amount,
			"details":   split1Details,
		},
		{
			"type":      split2Type,
			"recipient": split2Recipient,
			"amount":    split2Amount,
			"details":   split2Details,
		},
	}

	return splitArray
}

// toString — аккуратное преобразование числовых/строковых значений к строке без форматирования
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatInt(int64(val), 10)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case json.Number:
		return string(val)
	default:
		return ""
	}
}
