package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"kliro/models"
	"kliro/utils"
)

type UserHumansController struct {
	db *gorm.DB
}

func NewUserHumansController() *UserHumansController {
	return &UserHumansController{db: utils.GetDB()}
}

type humanPayload struct {
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	MiddleName     string `json:"middle_name"`
	BirthDate      string `json:"birth_date"`
	Gender         string `json:"gender"`
	Citizenship    string `json:"citizenship"`
	PassportNumber string `json:"passport_number"`
	PassportExpiry string `json:"passport_expiry"`
	Phone          string `json:"phone"`
}

func (ctl *UserHumansController) CreateHuman(c *gin.Context) {
	userID := c.GetInt("user_id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "unauthorized"})
		return
	}
	var req humanPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}
	h := models.UserHuman{
		UserID:         uint(userID),
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		MiddleName:     req.MiddleName,
		BirthDate:      req.BirthDate,
		Gender:         req.Gender,
		Citizenship:    req.Citizenship,
		PassportNumber: req.PassportNumber,
		PassportExpiry: req.PassportExpiry,
		Phone:          req.Phone,
	}
	if err := ctl.db.Create(&h).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "failed to create"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"result": ctl.toResponse(h), "success": true})
}

func (ctl *UserHumansController) UpdateHuman(c *gin.Context) {
	userID := c.GetInt("user_id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "unauthorized"})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid id"})
		return
	}
	var h models.UserHuman
	if err := ctl.db.Where("id = ? AND user_id = ?", id, userID).First(&h).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"result": nil, "success": false, "error": "not found"})
		return
	}
	var req humanPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid request"})
		return
	}
	h.FirstName = req.FirstName
	h.LastName = req.LastName
	h.MiddleName = req.MiddleName
	h.BirthDate = req.BirthDate
	h.Gender = req.Gender
	h.Citizenship = req.Citizenship
	h.PassportNumber = req.PassportNumber
	h.PassportExpiry = req.PassportExpiry
	h.Phone = req.Phone
	if err := ctl.db.Save(&h).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "failed to update"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": ctl.toResponse(h), "success": true})
}

func (ctl *UserHumansController) DeleteHuman(c *gin.Context) {
	userID := c.GetInt("user_id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "unauthorized"})
		return
	}
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "invalid id"})
		return
	}
	if err := ctl.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.UserHuman{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "failed to delete"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": gin.H{"id": id}, "success": true})
}

func (ctl *UserHumansController) ListHumans(c *gin.Context) {
	userID := c.GetInt("user_id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "unauthorized"})
		return
	}
	var list []models.UserHuman
	if err := ctl.db.Where("user_id = ?", userID).Order("created_at desc").Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "failed to fetch"})
		return
	}
	resp := make([]gin.H, 0, len(list))
	for _, h := range list {
		resp = append(resp, ctl.toResponse(h))
	}
	c.JSON(http.StatusOK, gin.H{"result": resp, "success": true})
}

func (ctl *UserHumansController) SearchByName(c *gin.Context) {
	userID := c.GetInt("user_id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"result": nil, "success": false, "error": "unauthorized"})
		return
	}
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"result": nil, "success": false, "error": "name is required"})
		return
	}
	p := "%" + strings.ToLower(name) + "%"
	var list []models.UserHuman
	if err := ctl.db.Where("user_id = ? AND LOWER(first_name) LIKE ?", userID, p).Order("created_at desc").Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "failed to search"})
		return
	}
	resp := make([]gin.H, 0, len(list))
	for _, h := range list {
		resp = append(resp, ctl.toResponse(h))
	}
	c.JSON(http.StatusOK, gin.H{"result": resp, "success": true})
}

func (ctl *UserHumansController) toResponse(h models.UserHuman) gin.H {
	return gin.H{
		"id":              h.ID,
		"first_name":      h.FirstName,
		"last_name":       h.LastName,
		"middle_name":     h.MiddleName,
		"birth_date":      h.BirthDate,
		"gender":          h.Gender,
		"citizenship":     h.Citizenship,
		"passport_number": h.PassportNumber,
		"passport_expiry": h.PassportExpiry,
		"phone":           h.Phone,
		"created_at":      h.CreatedAt,
		"updated_at":      h.UpdatedAt,
	}
}
