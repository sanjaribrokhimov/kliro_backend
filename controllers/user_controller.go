package controllers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"kliro/models"
	"kliro/utils"

	"encoding/json"

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите только email или только phone"})
		return
	}

	// Проверка на существование пользователя
	db := utils.GetDB()
	var userCount int64
	if req.Email != "" {
		db.Model(&models.User{}).Where("email = ?", req.Email).Count(&userCount)
	} else {
		db.Model(&models.User{}).Where("phone = ?", req.Phone).Count(&userCount)
	}
	if userCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Пользователь уже существует"})
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
		c.JSON(http.StatusTooManyRequests, gin.H{"error": msg})
		return
	}

	otp := utils.GenerateOTP()
	utils.MarkOTPSent(uc.RDB, redisKey)
	uc.RDB.Set(ctx, redisKey+":otp", otp, 5*time.Minute)

	msg := fmt.Sprintf("KLIRO: Ваш код подтверждения для регистрации на сайте: %s", otp)
	if channel == "email" {
		err := utils.SendEmail(to, "KLIRO: Код подтверждения", msg, os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT"), os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка отправки email"})
			return
		}
	} else {
		token, err := utils.GetEskizToken(os.Getenv("ESKIZ_EMAIL"), os.Getenv("ESKIZ_PASSWORD"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка Eskiz авторизации"})
			return
		}
		err = utils.SendEskizSMS(token, to, msg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка отправки SMS"})
			return
		}
	}

	// Сохраняем временные данные (можно расширить по ТЗ)
	uc.RDB.Set(ctx, redisKey+":data", "pending", 5*time.Minute)

	c.JSON(http.StatusOK, gin.H{"status": "otp sent deploy"})
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
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		c.JSON(400, gin.H{"error": "Укажите только email или только phone"})
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
		c.JSON(400, gin.H{"error": "Неверный или истёкший код"})
		return
	}
	// Помечаем как подтверждённый (флаг в Redis)
	uc.RDB.Set(ctx, redisKey+":confirmed", "1", 10*time.Minute)
	c.JSON(200, gin.H{"status": "otp confirmed"})
}

// POST /confirm-otp-create
func (uc *UserController) ConfirmOTPCreate(c *gin.Context) {
	var req ConfirmOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		c.JSON(400, gin.H{"error": "Укажите только email или только phone"})
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
		c.JSON(400, gin.H{"error": "Неверный или истёкший код"})
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
		c.JSON(500, gin.H{"error": "Ошибка сохранения пользователя"})
		return
	}
	// Очищаем временные данные
	uc.RDB.Del(ctx, redisKey+":otp", redisKey+":confirmed", redisKey+":data")
	c.JSON(200, gin.H{"status": "user created, set region and password"})
}

type SetRegionPasswordRequest struct {
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	RegionID uint   `json:"region_id"`
	Password string `json:"password"`
}

// POST /set-region-password-final
func (uc *UserController) SetRegionPasswordFinal(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		RegionID uint   `json:"region_id"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		c.JSON(400, gin.H{"error": "Укажите только email или только phone"})
		return
	}
	if req.RegionID == 0 || req.Password == "" {
		c.JSON(400, gin.H{"error": "region_id и password обязательны"})
		return
	}
	ctx := context.Background()
	var redisKey string
	if req.Email != "" {
		redisKey = "reg:email:" + strings.ToLower(req.Email)
	} else {
		redisKey = "reg:phone:" + req.Phone
	}
	confirmed, err := uc.RDB.Get(ctx, redisKey+":confirmed").Result()
	if err != nil || confirmed != "1" {
		c.JSON(400, gin.H{"error": "Сначала подтвердите OTP"})
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
		c.JSON(400, gin.H{"error": "Пользователь уже существует"})
		return
	}
	// Хэшируем пароль
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": "Ошибка хэширования пароля"})
		return
	}
	// Создаём пользователя
	user := &models.User{
		Email:     nil,
		Phone:     nil,
		RegionID:  &req.RegionID,
		Password:  hash,
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
		c.JSON(500, gin.H{"error": "Ошибка сохранения пользователя"})
		return
	}
	// Очищаем временные данные из Redis
	uc.RDB.Del(ctx, redisKey+":otp", redisKey+":confirmed", redisKey+":data")
	c.JSON(200, gin.H{"status": "user created"})
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
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		c.JSON(400, gin.H{"error": "Укажите только email или только phone"})
		return
	}
	if req.Password == "" {
		c.JSON(400, gin.H{"error": "Пароль обязателен"})
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
		c.JSON(404, gin.H{"error": "Пользователь не найден"})
		return
	}
	// Проверка: если это Google-аккаунт (GoogleID заполнен, пароль пустой или дефолтный)
	if user.GoogleID != nil && *user.GoogleID != "" && (user.Password == "" || user.Password == "-") {
		c.JSON(400, gin.H{"error": "Этот аккаунт зарегистрирован через Google. Войдите через Google OAuth."})
		return
	}
	if !utils.CheckPasswordHash(req.Password, user.Password) {
		c.JSON(401, gin.H{"error": "Пароль неверный"})
		return
	}
	jwt, err := utils.GenerateJWT(user.ID, user.Role, os.Getenv("JWT_SECRET"))
	if err != nil {
		c.JSON(500, gin.H{"error": "Ошибка генерации токена"})
		return
	}
	c.JSON(200, gin.H{"token": jwt})
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
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		c.JSON(400, gin.H{"error": "Укажите только email или только phone"})
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
		c.JSON(429, gin.H{"error": msg})
		return
	}
	otp := utils.GenerateOTP()
	utils.MarkOTPSent(uc.RDB, redisKey)
	uc.RDB.Set(ctx, redisKey+":otp", otp, 5*time.Minute)
	msg := fmt.Sprintf("KLIRO: Ваш код подтверждения для восстановления пароля на сайте: %s", otp)
	if channel == "email" {
		err := utils.SendEmail(to, "KLIRO: Восстановление пароля", msg, os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT"), os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"))
		if err != nil {
			c.JSON(500, gin.H{"error": "Ошибка отправки email"})
			return
		}
	} else {
		token, err := utils.GetEskizToken(os.Getenv("ESKIZ_EMAIL"), os.Getenv("ESKIZ_PASSWORD"))
		if err != nil {
			c.JSON(500, gin.H{"error": "Ошибка Eskiz авторизации"})
			return
		}
		err = utils.SendEskizSMS(token, to, msg)
		if err != nil {
			c.JSON(500, gin.H{"error": "Ошибка отправки SMS"})
			return
		}
	}
	uc.RDB.Set(ctx, redisKey+":data", "pending", 5*time.Minute)
	c.JSON(200, gin.H{"status": "otp sent deploy"})
}

// POST /reset-password
func (uc *UserController) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if (req.Email == "" && req.Phone == "") || (req.Email != "" && req.Phone != "") {
		c.JSON(400, gin.H{"error": "Укажите только email или только phone"})
		return
	}
	if req.Password == "" || req.OTP == "" {
		c.JSON(400, gin.H{"error": "otp и password обязательны"})
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
		c.JSON(400, gin.H{"error": "Неверный или истёкший код"})
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
		c.JSON(404, gin.H{"error": "Пользователь не найден или не подтверждён"})
		return
	}
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": "Ошибка хэширования пароля"})
		return
	}
	user.Password = hash
	if err := db.Save(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Ошибка обновления пароля"})
		return
	}
	uc.RDB.Del(ctx, redisKey+":otp", redisKey+":data")
	c.JSON(200, gin.H{"status": "password updated"})
}

type googleUserInfo struct {
	Email string `json:"email"`
	Id    string `json:"id"`
	Name  string `json:"name"`
}

// GET /auth/google
func (uc *UserController) GoogleLogin(c *gin.Context) {
	url := googleOauthConfig.AuthCodeURL("state", oauth2.AccessTypeOffline)
	c.Redirect(302, url)
}

// GET /auth/google/callback
func (uc *UserController) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(400, gin.H{"error": "code not found"})
		return
	}
	token, err := googleOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		c.JSON(400, gin.H{"error": "token exchange failed"})
		return
	}
	client := googleOauthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo?alt=json")
	if err != nil || resp.StatusCode != 200 {
		c.JSON(400, gin.H{"error": "failed to get user info"})
		return
	}
	defer resp.Body.Close()
	var userInfo googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		c.JSON(400, gin.H{"error": "failed to decode user info"})
		return
	}
	if userInfo.Email == "" {
		c.JSON(400, gin.H{"error": "email not found in Google profile"})
		return
	}
	db := utils.GetDB()
	var user models.User
	result := db.Where("email = ?", userInfo.Email).First(&user)
	if result.Error == nil {
		// Пользователь найден — выдаём JWT
		jwt, err := utils.GenerateJWT(user.ID, user.Role, os.Getenv("JWT_SECRET"))
		if err != nil {
			c.JSON(500, gin.H{"error": "Ошибка генерации токена"})
			return
		}
		c.JSON(200, gin.H{"token": jwt})
		return
	}
	// Новый пользователь — сохраняем данные в Redis
	sessionID := utils.GenerateSessionID()
	ctx := context.Background()
	redisKey := "google:session:" + sessionID
	userData := map[string]string{
		"email":     userInfo.Email,
		"google_id": userInfo.Id,
		"name":      userInfo.Name,
	}
	userDataJson, _ := json.Marshal(userData)
	uc.RDB.Set(ctx, redisKey, userDataJson, 10*time.Minute)
	c.JSON(200, gin.H{"need_region": true, "session_id": sessionID})
}

// POST /auth/google/complete
func (uc *UserController) GoogleComplete(c *gin.Context) {
	type CompleteReq struct {
		SessionID string `json:"session_id"`
		RegionID  uint   `json:"region_id"`
	}
	var req CompleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.SessionID == "" || req.RegionID == 0 {
		c.JSON(400, gin.H{"error": "session_id и region_id обязательны"})
		return
	}
	ctx := context.Background()
	redisKey := "google:session:" + req.SessionID
	userDataJson, err := uc.RDB.Get(ctx, redisKey).Result()
	if err != nil {
		c.JSON(400, gin.H{"error": "session not found or expired"})
		return
	}
	var userData map[string]string
	if err := json.Unmarshal([]byte(userDataJson), &userData); err != nil {
		c.JSON(500, gin.H{"error": "failed to parse session data"})
		return
	}
	db := utils.GetDB()
	// Проверяем, что пользователь не был создан в обход
	var user models.User
	result := db.Where("email = ?", userData["email"]).First(&user)
	if result.Error == nil {
		c.JSON(400, gin.H{"error": "user already exists"})
		return
	}
	email := userData["email"]
	name := userData["name"]
	googleID := userData["google_id"]
	user = models.User{
		Email:     &email,
		Name:      &name,
		GoogleID:  &googleID,
		RegionID:  &req.RegionID,
		Confirmed: true,
		Role:      "user",
	}
	if err := db.Create(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "Ошибка сохранения пользователя"})
		return
	}
	uc.RDB.Del(ctx, redisKey)
	jwt, err := utils.GenerateJWT(user.ID, user.Role, os.Getenv("JWT_SECRET"))
	if err != nil {
		c.JSON(500, gin.H{"error": "Ошибка генерации токена"})
		return
	}
	c.JSON(200, gin.H{"token": jwt})
}
