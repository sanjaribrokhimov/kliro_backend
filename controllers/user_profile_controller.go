package controllers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"kliro/models"
	"kliro/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

type UserProfileController struct {
	RDB *redis.Client
}

func NewUserProfileController(rdb *redis.Client) *UserProfileController {
	return &UserProfileController{RDB: rdb}
}

// GET /user/profile
func (upc *UserProfileController) GetProfile(c *gin.Context) {
	userID := c.GetInt("user_id")
	db := utils.GetDB()
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          user.ID,
		"email":       user.Email,
		"phone":       user.Phone,
		"region_id":   user.RegionID,
		"name":        user.Name,
		"role":        user.Role,
		"category_id": user.CategoryID,
	})
}

// POST /user/update-contact
func (upc *UserProfileController) UpdateContact(c *gin.Context) {
	userID := c.GetInt("user_id")
	var req struct {
		Email string `json:"email"`
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Email == "" && req.Phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email или phone обязателен"})
		return
	}
	db := utils.GetDB()
	if req.Email != "" {
		var count int64
		db.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email уже используется"})
			return
		}
	}
	if req.Phone != "" {
		var count int64
		db.Model(&models.User{}).Where("phone = ?", req.Phone).Count(&count)
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Телефон уже используется"})
			return
		}
	}
	// Генерируем и отправляем OTP
	ctx := context.Background()
	otp := utils.GenerateOTP()
	var redisKey, to, channel string
	if req.Email != "" {
		redisKey = fmt.Sprintf("update:email:%d", userID)
		to = req.Email
		channel = "email"
	} else {
		redisKey = fmt.Sprintf("update:phone:%d", userID)
		to = req.Phone
		channel = "phone"
	}
	upc.RDB.Set(ctx, redisKey+":otp", otp, 5*time.Minute)
	msg := fmt.Sprintf("KLIRO: Ваш код для подтверждения смены контакта: %s", otp)
	if channel == "email" {
		err := utils.SendEmail(to, "KLIRO: Подтверждение контакта", msg, os.Getenv("SMTP_HOST"), os.Getenv("SMTP_PORT"), os.Getenv("SMTP_USER"), os.Getenv("SMTP_PASS"))
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
	upc.RDB.Set(ctx, redisKey+":data", to, 5*time.Minute)
	c.JSON(http.StatusOK, gin.H{"status": "otp sent"})
}

// POST /user/confirm-update-contact
func (upc *UserProfileController) ConfirmUpdateContact(c *gin.Context) {
	userID := c.GetInt("user_id")
	var req struct {
		OTP string `json:"otp"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	ctx := context.Background()
	db := utils.GetDB()
	var redisKey string
	var contactType string
	// Проверяем, какой контакт обновляется
	if val, err := upc.RDB.Get(ctx, fmt.Sprintf("update:email:%d:data", userID)).Result(); err == nil && val != "" {
		redisKey = fmt.Sprintf("update:email:%d", userID)
		contactType = "email"
	} else if val, err := upc.RDB.Get(ctx, fmt.Sprintf("update:phone:%d:data", userID)).Result(); err == nil && val != "" {
		redisKey = fmt.Sprintf("update:phone:%d", userID)
		contactType = "phone"
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Нет ожидающего подтверждения контакта"})
		return
	}
	otpInRedis, err := upc.RDB.Get(ctx, redisKey+":otp").Result()
	if err != nil || otpInRedis != req.OTP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный или истёкший код"})
		return
	}
	contactValue, _ := upc.RDB.Get(ctx, redisKey+":data").Result()
	var user models.User
	db.First(&user, userID)
	if contactType == "email" {
		user.Email = &contactValue
	} else {
		user.Phone = &contactValue
	}
	db.Save(&user)
	upc.RDB.Del(ctx, redisKey+":otp", redisKey+":data")
	c.JSON(http.StatusOK, gin.H{"status": "contact updated"})
}

// POST /user/change-password
func (upc *UserProfileController) ChangePassword(c *gin.Context) {
	userID := c.GetInt("user_id")
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Оба пароля обязательны"})
		return
	}
	db := utils.GetDB()
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}
	if !utils.CheckPasswordHash(req.OldPassword, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Старый пароль неверный"})
		return
	}
	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка хэширования пароля"})
		return
	}
	user.Password = hash
	db.Save(&user)
	c.JSON(http.StatusOK, gin.H{"status": "password changed"})
}

// POST /user/change-region
func (upc *UserProfileController) ChangeRegion(c *gin.Context) {
	userID := c.GetInt("user_id")
	var req struct {
		RegionID uint `json:"region_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.RegionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "region_id обязателен"})
		return
	}
	db := utils.GetDB()
	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}
	user.RegionID = &req.RegionID
	db.Save(&user)
	c.JSON(http.StatusOK, gin.H{"status": "region changed"})
}

// POST /user/add-contact
func (upc *UserProfileController) AddContact(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Email == "" && req.Phone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email или phone обязателен"})
		return
	}
	db := utils.GetDB()
	if req.Email != "" {
		var count int64
		db.Model(&models.User{}).Where("email = ?", req.Email).Count(&count)
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email уже используется"})
			return
		}
	}
	if req.Phone != "" {
		var count int64
		db.Model(&models.User{}).Where("phone = ?", req.Phone).Count(&count)
		if count > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Телефон уже используется"})
			return
		}
	}
	// Можно добавить контакт в отдельную таблицу, если нужно. Сейчас просто возвращаем успех.
	c.JSON(http.StatusOK, gin.H{"status": "contact added (mock)"})
}
