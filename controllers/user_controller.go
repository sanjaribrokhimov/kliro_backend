package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"kliro/models"
	"kliro/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

var googleOauthConfig *oauth2.Config

func InitGoogleOAuth() {
	googleOauthConfig = &oauth2.Config{
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URI"),
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
		Endpoint:     google.Endpoint,
	}
}

type UserRegisterRequest struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type UserController struct {
	RDB *redis.Client
}

func NewUserController(rdb *redis.Client) *UserController {
	return &UserController{RDB: rdb}
}

func (uc *UserController) Register(c *gin.Context) {
	var req UserRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[REGISTER] Invalid request: %v", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}

	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		log.Printf("[REGISTER] Invalid credentials: email=%s, phone=%s", req.Email, req.Phone)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Укажите только email или только phone"})
		return
	}

	log.Printf("[REGISTER] Starting registration for: email=%s, phone=%s", req.Email, req.Phone)

	// Проверка на существование пользователя
	db := utils.GetDB()
	var userCount int64
	if req.Email != "" {
		db.Model(&models.User{}).Where("email = ?", req.Email).Count(&userCount)
	} else {
		db.Model(&models.User{}).Where("phone = ?", req.Phone).Count(&userCount)
	}
	if userCount > 0 {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Пользователь уже существует"})
		return
	}

	ctx := context.Background()
	var channel, to, redisKey string
	if req.Email != "" {
		channel = "email"
		to = req.Email
		redisKey = "reg:email:" + strings.ToLower(req.Email)
	} else {
		channel = "phone"
		to = req.Phone
		redisKey = "reg:phone:" + req.Phone
	}

	// Лимиты
	if ok, msg := utils.CanSendOTP(uc.RDB, redisKey); !ok {
		c.JSON(429, gin.H{"result": nil, "success": false, "error": msg})
		return
	}

	otp := utils.GenerateOTP()
	utils.MarkOTPSent(uc.RDB, redisKey)
	uc.RDB.Set(ctx, redisKey+":otp", otp, 5*time.Minute)

	msg := fmt.Sprintf("KLIRO: Ваш код подтверждения для регистрации на сайте: %s", otp)
	if channel == "email" {
		err := utils.SendEmail(to, "KLIRO: Код подтверждения", msg, os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT"), os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"))
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка отправки email"})
			return
		}
	} else {
		token, err := utils.GetEskizToken(os.Getenv("ESKIZ_EMAIL"), os.Getenv("ESKIZ_PASSWORD"))
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка Eskiz авторизации"})
			return
		}
		err = utils.SendEskizSMS(token, to, msg)
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка отправки SMS"})
			return
		}
	}

	// Сохраняем временные данные (можно расширить по ТЗ)
	uc.RDB.Set(ctx, redisKey+":data", "pending", 5*time.Minute)

	log.Printf("[REGISTER] OTP sent successfully via %s to: %s", channel, to)
	c.JSON(200, gin.H{"result": gin.H{"status": "otp sent deploy"}, "success": true})
}

type ConfirmOTPRequest struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
	OTP   string `json:"otp"`
}

// POST /confirm-otp
func (uc *UserController) ConfirmOTP(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
		Phone string `json:"phone"`
		OTP   string `json:"otp"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Укажите только email или только phone"})
		return
	}
	ctx := context.Background()
	var redisKey string
	if req.Email != "" {
		redisKey = "reg:email:" + strings.ToLower(req.Email)
	} else {
		redisKey = "reg:phone:" + req.Phone
	}
	otpInRedis, err := uc.RDB.Get(ctx, redisKey+":otp").Result()
	if err != nil || otpInRedis != req.OTP {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Неверный или истёкший код"})
		return
	}
	// Помечаем как подтверждённый (флаг в Redis)
	uc.RDB.Set(ctx, redisKey+":confirmed", "1", 10*time.Minute)
	c.JSON(200, gin.H{"result": gin.H{"status": "otp confirmed"}, "success": true})
}

// POST /confirm-otp-create
func (uc *UserController) ConfirmOTPCreate(c *gin.Context) {
	var req ConfirmOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[CONFIRM_OTP_CREATE] Invalid request: %v", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		log.Printf("[CONFIRM_OTP_CREATE] Invalid credentials: email=%s, phone=%s", req.Email, req.Phone)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Укажите только email или только phone"})
		return
	}

	log.Printf("[CONFIRM_OTP_CREATE] Starting user creation for: email=%s, phone=%s", req.Email, req.Phone)
	ctx := context.Background()
	var redisKey string
	if req.Email != "" {
		redisKey = "reg:email:" + strings.ToLower(req.Email)
	} else {
		redisKey = "reg:phone:" + req.Phone
	}
	otpInRedis, err := uc.RDB.Get(ctx, redisKey+":otp").Result()
	if err != nil || otpInRedis != req.OTP {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Неверный или истёкший код"})
		return
	}
	// Создаём пользователя с confirmed=true, без пароля и региона
	db := utils.GetDB()
	user := &models.User{
		Email:     nil,
		Phone:     nil,
		Confirmed: true,
		Role:      "user",
	}
	if req.Email != "" {
		email := req.Email
		user.Email = &email
	}
	if req.Phone != "" {
		phone := req.Phone
		user.Phone = &phone
	}
	if err := db.Create(user).Error; err != nil {
		log.Printf("[CONFIRM_OTP_CREATE] Error creating user: %v", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка сохранения пользователя"})
		return
	}
	log.Printf("[CONFIRM_OTP_CREATE] User created successfully with ID: %d, email=%s, phone=%s", user.ID, req.Email, req.Phone)
	uc.RDB.Del(ctx, redisKey+":otp", redisKey+":confirmed", redisKey+":data")
	c.JSON(200, gin.H{"result": gin.H{"status": "user created, set region and password"}, "success": true})
}

type SetRegionPasswordRequest struct {
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	RegionID  uint   `json:"region_id"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// POST /set-region-password-final
func (uc *UserController) SetRegionPasswordFinal(c *gin.Context) {
	var req struct {
		Email     string `json:"email"`
		Phone     string `json:"phone"`
		RegionID  uint   `json:"region_id"`
		Password  string `json:"password"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[SET_REGION_PASSWORD] Invalid request: %v", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		log.Printf("[SET_REGION_PASSWORD] Invalid credentials: email=%s, phone=%s", req.Email, req.Phone)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Укажите только email или только phone"})
		return
	}
	if req.RegionID == 0 || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		log.Printf("[SET_REGION_PASSWORD] Missing required fields for: email=%s, phone=%s", req.Email, req.Phone)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "region_id, password, first_name и last_name обязательны"})
		return
	}

	log.Printf("[SET_REGION_PASSWORD] Starting final registration for: email=%s, phone=%s, first_name=%s, last_name=%s, region_id=%d",
		req.Email, req.Phone, req.FirstName, req.LastName, req.RegionID)
	ctx := context.Background()
	var redisKey string
	if req.Email != "" {
		redisKey = "reg:email:" + strings.ToLower(req.Email)
	} else {
		redisKey = "reg:phone:" + req.Phone
	}
	confirmed, err := uc.RDB.Get(ctx, redisKey+":confirmed").Result()
	if err != nil || confirmed != "1" {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Сначала подтвердите OTP"})
		return
	}
	// Проверяем, что пользователь с таким email/phone ещё не существует
	db := utils.GetDB()
	var userCount int64
	if req.Email != "" {
		db.Model(&models.User{}).Where("email = ?", req.Email).Count(&userCount)
	} else {
		db.Model(&models.User{}).Where("phone = ?", req.Phone).Count(&userCount)
	}
	if userCount > 0 {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Пользователь уже существует"})
		return
	}
	// Хэшируем пароль
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка хэширования пароля"})
		return
	}
	// Создаём пользователя
	user := &models.User{
		Email:     nil,
		Phone:     nil,
		RegionID:  &req.RegionID,
		Password:  hash,
		FirstName: &req.FirstName,
		LastName:  &req.LastName,
		Confirmed: true,
		Role:      "user",
	}
	if req.Email != "" {
		emailVal := req.Email
		user.Email = &emailVal
	}
	if req.Phone != "" {
		phoneVal := req.Phone
		user.Phone = &phoneVal
	}
	if err := db.Create(user).Error; err != nil {
		log.Printf("[SET_REGION_PASSWORD] Error creating user: %v", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка сохранения пользователя"})
		return
	}

	log.Printf("[SET_REGION_PASSWORD] User registered successfully with ID: %d, email=%s, phone=%s, first_name=%s, last_name=%s",
		user.ID, req.Email, req.Phone, req.FirstName, req.LastName)

	uc.RDB.Del(ctx, redisKey+":otp", redisKey+":confirmed", redisKey+":data")

	accessToken, err := utils.GenerateJWT(user.ID, user.Role, os.Getenv("JWT_SECRET"))
	if err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка генерации токена"})
		return
	}
	refreshToken, refreshExp, err := utils.GenerateRefreshToken(user.ID, os.Getenv("JWT_SECRET"))
	if err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка генерации refresh токена"})
		return
	}
	accessClaims, _ := utils.ParseJWT(accessToken, os.Getenv("JWT_SECRET"))
	accessExp := int64(accessClaims["exp"].(float64))
	c.JSON(200, gin.H{"result": gin.H{
		"accessToken":        accessToken,
		"refreshToken":       refreshToken,
		"accessTokenExpiry":  accessExp,
		"refreshTokenExpiry": refreshExp,
	}, "success": true})
}

type LoginRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

// POST /login
func (uc *UserController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[LOGIN] Invalid request: %v", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		log.Printf("[LOGIN] Invalid credentials: email=%s, phone=%s", req.Email, req.Phone)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Укажите только email или только phone"})
		return
	}
	if req.Password == "" {
		log.Printf("[LOGIN] Password missing for: email=%s, phone=%s", req.Email, req.Phone)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Пароль обязателен"})
		return
	}

	log.Printf("[LOGIN] Login attempt for: email=%s, phone=%s", req.Email, req.Phone)
	db := utils.GetDB()
	var user models.User
	var result *gorm.DB
	if req.Email != "" {
		result = db.Where("email = ? AND confirmed = ?", req.Email, true).First(&user)
	} else {
		result = db.Where("phone = ? AND confirmed = ?", req.Phone, true).First(&user)
	}
	if result.Error != nil {
		log.Printf("[LOGIN] User not found: email=%s, phone=%s", req.Email, req.Phone)
		c.JSON(404, gin.H{"result": nil, "success": false, "error": "Пользователь не найден"})
		return
	}
	if user.GoogleID != nil && *user.GoogleID != "" && (user.Password == "" || user.Password == "-") {
		log.Printf("[LOGIN] Google OAuth user tried to login with password: user_id=%d, email=%s", user.ID, req.Email)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Этот аккаунт зарегистрирован через Google. Войдите через Google OAuth."})
		return
	}
	if !utils.CheckPasswordHash(req.Password, user.Password) {
		log.Printf("[LOGIN] Invalid password for user_id=%d, email=%s, phone=%s", user.ID, req.Email, req.Phone)
		c.JSON(401, gin.H{"result": nil, "success": false, "error": "Пароль неверный"})
		return
	}

	log.Printf("[LOGIN] User logged in successfully: user_id=%d, email=%s, phone=%s, role=%s", user.ID, req.Email, req.Phone, user.Role)
	accessToken, err := utils.GenerateJWT(user.ID, user.Role, os.Getenv("JWT_SECRET"))
	if err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка генерации токена"})
		return
	}
	refreshToken, refreshExp, err := utils.GenerateRefreshToken(user.ID, os.Getenv("JWT_SECRET"))
	if err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка генерации refresh токена"})
		return
	}
	accessClaims, _ := utils.ParseJWT(accessToken, os.Getenv("JWT_SECRET"))
	accessExp := int64(accessClaims["exp"].(float64))
	c.JSON(200, gin.H{"result": gin.H{
		"accessToken":        accessToken,
		"refreshToken":       refreshToken,
		"accessTokenExpiry":  accessExp,
		"refreshTokenExpiry": refreshExp,
	}, "success": true})
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type ResetPasswordRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	OTP      string `json:"otp"`
	Password string `json:"password"`
}

// POST /forgot-password
func (uc *UserController) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Укажите только email или только phone"})
		return
	}
	ctx := context.Background()
	var channel, to, redisKey string
	if req.Email != "" {
		channel = "email"
		to = req.Email
		redisKey = "reset:email:" + strings.ToLower(req.Email)
	} else {
		channel = "phone"
		to = req.Phone
		redisKey = "reset:phone:" + req.Phone
	}
	// Лимиты
	if ok, msg := utils.CanSendOTP(uc.RDB, redisKey); !ok {
		c.JSON(429, gin.H{"result": nil, "success": false, "error": msg})
		return
	}
	otp := utils.GenerateOTP()
	utils.MarkOTPSent(uc.RDB, redisKey)
	uc.RDB.Set(ctx, redisKey+":otp", otp, 5*time.Minute)
	msg := fmt.Sprintf("KLIRO: Ваш код подтверждения для восстановления пароля на сайте: %s", otp)
	if channel == "email" {
		err := utils.SendEmail(to, "KLIRO: Восстановление пароля", msg, os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT"), os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"))
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка отправки email"})
			return
		}
	} else {
		token, err := utils.GetEskizToken(os.Getenv("ESKIZ_EMAIL"), os.Getenv("ESKIZ_PASSWORD"))
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка Eskiz авторизации"})
			return
		}
		err = utils.SendEskizSMS(token, to, msg)
		if err != nil {
			c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка отправки SMS"})
			return
		}
	}
	uc.RDB.Set(ctx, redisKey+":data", "pending", 5*time.Minute)
	c.JSON(200, gin.H{"result": gin.H{"status": "otp sent deploy"}, "success": true})
}

// POST /reset-password
func (uc *UserController) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Укажите только email или только phone"})
		return
	}
	if req.Password == "" || req.OTP == "" {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "otp и password обязательны"})
		return
	}
	ctx := context.Background()
	var redisKey string
	if req.Email != "" {
		redisKey = "reset:email:" + strings.ToLower(req.Email)
	} else {
		redisKey = "reset:phone:" + req.Phone
	}
	otpInRedis, err := uc.RDB.Get(ctx, redisKey+":otp").Result()
	if err != nil || otpInRedis != req.OTP {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "Неверный или истёкший код"})
		return
	}
	db := utils.GetDB()
	var user models.User
	var result *gorm.DB
	if req.Email != "" {
		result = db.Where("email = ? AND confirmed = ?", req.Email, true).First(&user)
	} else {
		result = db.Where("phone = ? AND confirmed = ?", req.Phone, true).First(&user)
	}
	if result.Error != nil {
		c.JSON(404, gin.H{"result": nil, "success": false, "error": "Пользователь не найден или не подтверждён"})
		return
	}
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка хэширования пароля"})
		return
	}
	user.Password = hash
	if err := db.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка обновления пароля"})
		return
	}
	uc.RDB.Del(ctx, redisKey+":otp", redisKey+":data")
	c.JSON(200, gin.H{"result": gin.H{"status": "password updated"}, "success": true})
}

type googleUserInfo struct {
	Email string `json:"email"`
	Id    string `json:"id"`
	Name  string `json:"name"`
}

// GET /auth/google
func (uc *UserController) GoogleLogin(c *gin.Context) {
	redirectURL := c.Query("redirect_url")
	if redirectURL == "" {
		redirectURL = "https://kliro.uz/auth/google-complete" // default frontend page
	}
	state := base64.URLEncoding.EncodeToString([]byte(redirectURL))
	url := googleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(302, url)
}

// GET /auth/google/callback
func (uc *UserController) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	redirectURL := "https://kliro.uz/auth-callback"
	if state != "" {
		decoded, err := base64.URLEncoding.DecodeString(state)
		if err == nil {
			redirectURL = string(decoded)
		}
	}
	if code == "" {
		c.Redirect(302, redirectURL+"?error=code_not_found")
		return
	}
	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.Redirect(302, redirectURL+"?error=token_exchange_failed")
		return
	}
	client := googleOauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo?alt=json")
	if err != nil || resp.StatusCode != 200 {
		c.Redirect(302, redirectURL+"?error=failed_to_get_user_info")
		return
	}
	defer resp.Body.Close()
	var userInfo googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		c.Redirect(302, redirectURL+"?error=failed_to_decode_user_info")
		return
	}
	if userInfo.Email == "" {
		c.Redirect(302, redirectURL+"?error=email_not_found")
		return
	}
	db := utils.GetDB()
	var user models.User
	result := db.Where("email = ?", userInfo.Email).First(&user)
	if result.Error == nil {
		// Пользователь найден — выдаём JWT через редирект
		accessToken, err := utils.GenerateJWT(user.ID, user.Role, os.Getenv("JWT_SECRET"))
		if err != nil {
			c.Redirect(302, redirectURL+"?error=token_generation_failed")
			return
		}
		refreshToken, refreshExp, err := utils.GenerateRefreshToken(user.ID, os.Getenv("JWT_SECRET"))
		if err != nil {
			c.Redirect(302, redirectURL+"?error=refresh_token_generation_failed")
			return
		}
		accessClaims, _ := utils.ParseJWT(accessToken, os.Getenv("JWT_SECRET"))
		accessExp := int64(accessClaims["exp"].(float64))
		// Собираем query string
		params := fmt.Sprintf("?accessToken=%s&refreshToken=%s&accessTokenExpiry=%d&refreshTokenExpiry=%d", accessToken, refreshToken, accessExp, refreshExp)
		c.Redirect(302, redirectURL+params)
		return
	}
	// Новый пользователь — сохраняем данные в Redis
	sessionID := utils.GenerateSessionID()
	ctx := context.Background()
	redisKey := "google:session:" + sessionID
	userData := map[string]string{
		"email":     userInfo.Email,
		"google_id": userInfo.Id,
	}
	userDataJson, _ := json.Marshal(userData)
	uc.RDB.Set(ctx, redisKey, userDataJson, 10*time.Minute)
	// Редиректим на фронт с need_region и session_id
	params := fmt.Sprintf("?need_region=true&session_id=%s", sessionID)
	c.Redirect(302, redirectURL+params)
}

// POST /auth/google/complete
func (uc *UserController) GoogleComplete(c *gin.Context) {
	type CompleteReq struct {
		SessionID string `json:"session_id"`
		RegionID  uint   `json:"region_id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	var req CompleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[GOOGLE_COMPLETE] Invalid request: %v", err)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}
	if req.SessionID == "" || req.RegionID == 0 || req.FirstName == "" || req.LastName == "" {
		log.Printf("[GOOGLE_COMPLETE] Missing required fields: session_id=%s, region_id=%d", req.SessionID, req.RegionID)
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "session_id, region_id, first_name и last_name обязательны"})
		return
	}

	log.Printf("[GOOGLE_COMPLETE] Starting Google registration completion: session_id=%s, first_name=%s, last_name=%s, region_id=%d",
		req.SessionID, req.FirstName, req.LastName, req.RegionID)
	ctx := context.Background()
	redisKey := "google:session:" + req.SessionID
	userDataJson, err := uc.RDB.Get(ctx, redisKey).Result()
	if err != nil {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "session not found or expired"})
		return
	}
	var userData map[string]string
	if err := json.Unmarshal([]byte(userDataJson), &userData); err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "failed to parse session data"})
		return
	}
	db := utils.GetDB()
	// Проверяем, что пользователь не был создан в обход
	var user models.User
	result := db.Where("email = ?", userData["email"]).First(&user)
	if result.Error == nil {
		c.JSON(400, gin.H{"result": nil, "success": false, "error": "user already exists"})
		return
	}
	email := userData["email"]
	googleID := userData["google_id"]
	user = models.User{
		Email:     &email,
		FirstName: &req.FirstName,
		LastName:  &req.LastName,
		GoogleID:  &googleID,
		RegionID:  &req.RegionID,
		Confirmed: true,
		Role:      "user",
	}
	if err := db.Create(&user).Error; err != nil {
		log.Printf("[GOOGLE_COMPLETE] Error creating user: %v", err)
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка сохранения пользователя"})
		return
	}

	log.Printf("[GOOGLE_COMPLETE] User registered successfully via Google with ID: %d, email=%s, first_name=%s, last_name=%s",
		user.ID, userData["email"], req.FirstName, req.LastName)

	uc.RDB.Del(ctx, redisKey)
	accessToken, err := utils.GenerateJWT(user.ID, user.Role, os.Getenv("JWT_SECRET"))
	if err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка генерации токена"})
		return
	}
	refreshToken, refreshExp, err := utils.GenerateRefreshToken(user.ID, os.Getenv("JWT_SECRET"))
	if err != nil {
		c.JSON(500, gin.H{"result": nil, "success": false, "error": "Ошибка генерации refresh токена"})
		return
	}
	accessClaims, _ := utils.ParseJWT(accessToken, os.Getenv("JWT_SECRET"))
	accessExp := int64(accessClaims["exp"].(float64))
	c.JSON(200, gin.H{"result": gin.H{
		"accessToken":        accessToken,
		"refreshToken":       refreshToken,
		"accessTokenExpiry":  accessExp,
		"refreshTokenExpiry": refreshExp,
	}, "success": true})
}
