package trustInsurance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"kliro/config"

	"github.com/gin-gonic/gin"
)

type OsagoController struct {
	config      *config.Config
	client      *http.Client
	token       string
	tokenExpiry time.Time
	mutex       sync.RWMutex
}

func NewOsagoController(cfg *config.Config) *OsagoController {
	return &OsagoController{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *OsagoController) getValidToken(login, password string) (string, error) {
	c.mutex.RLock()
	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		token := c.token
		c.mutex.RUnlock()
		return token, nil
	}
	c.mutex.RUnlock()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.token != "" && time.Now().Before(c.tokenExpiry) {
		return c.token, nil
	}

	token, err := c.authenticate(login, password)
	if err != nil {
		return "", err
	}

	c.token = token
	c.tokenExpiry = time.Now().Add(55 * time.Minute)
	return token, nil
}

func (c *OsagoController) authenticate(login, password string) (string, error) {
	authData := map[string]string{
		"login":    login,
		"password": password,
	}

	jsonData, err := json.Marshal(authData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal auth data: %v", err)
	}

	req, err := http.NewRequest("POST", c.config.TrustBaseURL+"/api/products/auth/login", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make auth request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("auth failed with status: %d", resp.StatusCode)
	}

	var authResponse struct {
		Result        int    `json:"result"`
		ResultMessage string `json:"result_message"`
		Roles         []struct {
			ID       int    `json:"id"`
			RoleName string `json:"roleName"`
		} `json:"roles"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read auth response: %v", err)
	}

	fmt.Printf("Auth response body: %s\n", string(body))

	if err := json.Unmarshal(body, &authResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal auth response: %v", err)
	}

	fmt.Printf("Parsed auth response: %+v\n", authResponse)

	if authResponse.Result == 0 && authResponse.ResultMessage != "" {
		// Убираем "Bearer " префикс если он есть
		token := authResponse.ResultMessage
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		return token, nil
	}

	return "", fmt.Errorf("no token found in auth response")
}

func (c *OsagoController) Login(ctx *gin.Context) {
	fmt.Println("=== Trust Insurance Login Request ===")

	var loginData struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}

	if err := ctx.ShouldBindJSON(&loginData); err != nil {
		fmt.Printf("JSON binding error: %v\n", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	fmt.Printf("Login attempt for user: %s\n", loginData.Login)

	authData := map[string]string{
		"login":    loginData.Login,
		"password": loginData.Password,
	}

	jsonData, err := json.Marshal(authData)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal auth data"})
		return
	}

	url := c.config.TrustBaseURL + "/api/products/auth/login"
	fmt.Printf("Making request to: %s\n", url)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create auth request"})
		return
	}

	req.Header.Set("Content-Type", "application/json")

	fmt.Println("Sending request to Trust Insurance API...")
	resp, err := c.client.Do(req)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make auth request"})
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Response status: %d\n", resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read response: %v\n", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	fmt.Printf("Response body: %s\n", string(body))

	// Если успешно, сохраняем токен для дальнейшего использования
	if resp.StatusCode == 200 {
		var authResponse struct {
			Result        int    `json:"result"`
			ResultMessage string `json:"result_message"`
			Roles         []struct {
				ID       int    `json:"id"`
				RoleName string `json:"roleName"`
			} `json:"roles"`
		}

		if err := json.Unmarshal(body, &authResponse); err == nil {
			if authResponse.Result == 0 && authResponse.ResultMessage != "" {
				// Убираем "Bearer " префикс если он есть
				token := authResponse.ResultMessage
				if len(token) > 7 && token[:7] == "Bearer " {
					token = token[7:]
				}
				c.token = token
				c.tokenExpiry = time.Now().Add(55 * time.Minute)
			}
		}
	}

	ctx.Data(resp.StatusCode, "application/json", body)
}

func (c *OsagoController) Create(ctx *gin.Context) {
	var requestBody interface{}
	if err := ctx.ShouldBindJSON(&requestBody); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal JSON"})
		return
	}

	fmt.Printf("=== INTERNAL TRUST API DEBUG ===\n")
	fmt.Printf("Received request body: %+v\n", requestBody)
	fmt.Printf("Marshaled JSON: %s\n", string(jsonData))
	fmt.Printf("JSON length: %d bytes\n", len(jsonData))

	// Проверяем, есть ли сохраненный токен, если нет - получаем автоматически
	c.mutex.RLock()
	token := c.token
	c.mutex.RUnlock()

	if token == "" {
		// Автоматически получаем токен используя credentials из .env
		fmt.Printf("No token found, attempting automatic auth with login: %s\n", c.config.TrustLogin)
		var err error
		token, err = c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
		if err != nil {
			fmt.Printf("Automatic auth failed: %v\n", err)
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to get auth token automatically", "details": err.Error()})
			return
		}
		fmt.Printf("Automatic auth successful, token obtained\n")
	}

	externalURL := c.config.TrustBaseURL + "/api/osgo/create"
	fmt.Printf("Sending to external Trust API: %s\n", externalURL)
	fmt.Printf("Request headers: Content-Type=application/json, Authorization=Bearer %s...\n", token[:20])

	req, err := http.NewRequest("POST", externalURL, bytes.NewBuffer(jsonData))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make request"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	fmt.Printf("External Trust API Response - Status: %d\n", resp.StatusCode)
	fmt.Printf("External Trust API Response Body: %s\n", string(body))

	ctx.Data(resp.StatusCode, "application/json", body)
}

func (c *OsagoController) CalcPrem(ctx *gin.Context) {
	var requestBody interface{}
	if err := ctx.ShouldBindJSON(&requestBody); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal JSON"})
		return
	}

	req, err := http.NewRequest("POST", c.config.TrustBaseURL+"/api/osgo/calc-prem", bytes.NewBuffer(jsonData))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	req.Header.Set("Content-Type", "application/json")
	// Проверяем, есть ли сохраненный токен, если нет - получаем автоматически
	c.mutex.RLock()
	token := c.token
	c.mutex.RUnlock()

	if token == "" {
		// Автоматически получаем токен используя credentials из .env
		fmt.Printf("No token found, attempting automatic auth with login: %s\n", c.config.TrustLogin)
		var err error
		token, err = c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
		if err != nil {
			fmt.Printf("Automatic auth failed: %v\n", err)
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to get auth token automatically", "details": err.Error()})
			return
		}
		fmt.Printf("Automatic auth successful, token obtained\n")
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make request"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	ctx.Data(resp.StatusCode, "application/json", body)
}

func (c *OsagoController) Relatives(ctx *gin.Context) {
	req, err := http.NewRequest("GET", c.config.TrustBaseURL+"/api/reference/relatives", nil)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Проверяем, есть ли сохраненный токен, если нет - получаем автоматически
	c.mutex.RLock()
	token := c.token
	c.mutex.RUnlock()

	if token == "" {
		// Автоматически получаем токен используя credentials из .env
		fmt.Printf("No token found, attempting automatic auth with login: %s\n", c.config.TrustLogin)
		var err error
		token, err = c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
		if err != nil {
			fmt.Printf("Automatic auth failed: %v\n", err)
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to get auth token automatically", "details": err.Error()})
			return
		}
		fmt.Printf("Automatic auth successful, token obtained\n")
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make request"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	ctx.Data(resp.StatusCode, "application/json", body)
}

func (c *OsagoController) Vehicle(ctx *gin.Context) {
	req, err := http.NewRequest("GET", c.config.TrustBaseURL+"/api/provider/vehicle", nil)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Проверяем, есть ли сохраненный токен, если нет - получаем автоматически
	c.mutex.RLock()
	token := c.token
	c.mutex.RUnlock()

	if token == "" {
		// Автоматически получаем токен используя credentials из .env
		fmt.Printf("No token found, attempting automatic auth with login: %s\n", c.config.TrustLogin)
		var err error
		token, err = c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
		if err != nil {
			fmt.Printf("Automatic auth failed: %v\n", err)
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to get auth token automatically", "details": err.Error()})
			return
		}
		fmt.Printf("Automatic auth successful, token obtained\n")
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make request"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	ctx.Data(resp.StatusCode, "application/json", body)
}

func (c *OsagoController) PassportPinfl(ctx *gin.Context) {
	req, err := http.NewRequest("GET", c.config.TrustBaseURL+"/api/provider/passport-pinfl", nil)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Проверяем, есть ли сохраненный токен, если нет - получаем автоматически
	c.mutex.RLock()
	token := c.token
	c.mutex.RUnlock()

	if token == "" {
		// Автоматически получаем токен используя credentials из .env
		fmt.Printf("No token found, attempting automatic auth with login: %s\n", c.config.TrustLogin)
		var err error
		token, err = c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
		if err != nil {
			fmt.Printf("Automatic auth failed: %v\n", err)
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to get auth token automatically", "details": err.Error()})
			return
		}
		fmt.Printf("Automatic auth successful, token obtained\n")
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make request"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	ctx.Data(resp.StatusCode, "application/json", body)
}

func (c *OsagoController) PassportBirthDate(ctx *gin.Context) {
	req, err := http.NewRequest("GET", c.config.TrustBaseURL+"/api/provider/passport-birth-date", nil)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Проверяем, есть ли сохраненный токен, если нет - получаем автоматически
	c.mutex.RLock()
	token := c.token
	c.mutex.RUnlock()

	if token == "" {
		// Автоматически получаем токен используя credentials из .env
		fmt.Printf("No token found, attempting automatic auth with login: %s\n", c.config.TrustLogin)
		var err error
		token, err = c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
		if err != nil {
			fmt.Printf("Automatic auth failed: %v\n", err)
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to get auth token automatically", "details": err.Error()})
			return
		}
		fmt.Printf("Automatic auth successful, token obtained\n")
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make request"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	ctx.Data(resp.StatusCode, "application/json", body)
}

func (c *OsagoController) DriverSummary(ctx *gin.Context) {
	req, err := http.NewRequest("GET", c.config.TrustBaseURL+"/api/provider/driver-summary", nil)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Проверяем, есть ли сохраненный токен, если нет - получаем автоматически
	c.mutex.RLock()
	token := c.token
	c.mutex.RUnlock()

	if token == "" {
		// Автоматически получаем токен используя credentials из .env
		fmt.Printf("No token found, attempting automatic auth with login: %s\n", c.config.TrustLogin)
		var err error
		token, err = c.getValidToken(c.config.TrustLogin, c.config.TrustPassword)
		if err != nil {
			fmt.Printf("Automatic auth failed: %v\n", err)
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to get auth token automatically", "details": err.Error()})
			return
		}
		fmt.Printf("Automatic auth successful, token obtained\n")
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make request"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response"})
		return
	}

	ctx.Data(resp.StatusCode, "application/json", body)
}
