package controllers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"kliro/models"
	"kliro/utils"
)

var validDirections = map[string]bool{
	"deposit":     true,
	"card":        true,
	"credit":      true,
	"mortgage":    true,
	"microcredit": true,
	"autocredit":  true,
	"transfer":    true,
}

type AnalyticsController struct{}

func NewAnalyticsController() *AnalyticsController {
	return &AnalyticsController{}
}

type TrackClickRequest struct {
	Key       string `json:"key" binding:"required"`
	Direction string `json:"direction" binding:"required"`
	URL       string `json:"url" binding:"required"`
}

func (ac *AnalyticsController) TrackClick(c *gin.Context) {
	var req TrackClickRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !validDirections[strings.ToLower(req.Direction)] {
		validDirs := []string{"deposit", "card", "credit", "mortgage", "microcredit", "autocredit", "transfer"}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid direction. Valid directions are: %s", strings.Join(validDirs, ", ")),
		})
		return
	}

	db := utils.GetDB()

	var productClick models.ProductClick
	result := db.Where("key = ? AND direction = ?", req.Key, req.Direction).First(&productClick)

	if result.Error == gorm.ErrRecordNotFound {
		productClick = models.ProductClick{
			Key:        req.Key,
			Direction:  req.Direction,
			URL:        req.URL,
			ClickCount: 1,
		}
		if err := db.Create(&productClick).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create click record"})
			return
		}
	} else if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	} else {
		productClick.ClickCount++
		productClick.URL = req.URL
		if err := db.Save(&productClick).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update click count"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"key":         productClick.Key,
			"direction":   productClick.Direction,
			"click_count": productClick.ClickCount,
			"message":     "Click tracked successfully",
		},
	})
}

func (ac *AnalyticsController) GetAllClicks(c *gin.Context) {
	db := utils.GetDB()

	var clicks []models.ProductClick
	if err := db.Order("click_count DESC").Find(&clicks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch clicks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": clicks,
	})
}

func (ac *AnalyticsController) GetClicksByDirection(c *gin.Context) {
	direction := c.Param("direction")

	if !validDirections[strings.ToLower(direction)] {
		validDirs := []string{"deposit", "card", "credit", "mortgage", "microcredit", "autocredit", "transfer"}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid direction. Valid directions are: %s", strings.Join(validDirs, ", ")),
		})
		return
	}

	db := utils.GetDB()

	var clicks []models.ProductClick
	if err := db.Where("direction = ?", direction).Order("click_count DESC").Find(&clicks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch clicks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": clicks,
	})
}

func (ac *AnalyticsController) GetClicksByDirectionAndDate(c *gin.Context) {
	direction := c.Param("direction")
	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")
	key := c.Query("key")

	if dateFrom == "" || dateTo == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_from and date_to are required in format YYYY-MM-DD"})
		return
	}

	if !validDirections[strings.ToLower(direction)] {
		validDirs := []string{"deposit", "card", "credit", "mortgage", "microcredit", "autocredit", "transfer"}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("invalid direction. Valid directions are: %s", strings.Join(validDirs, ", ")),
		})
		return
	}

	fromTime, err := time.Parse("2006-01-02", dateFrom)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_from must be YYYY-MM-DD"})
		return
	}

	toTime, err := time.Parse("2006-01-02", dateTo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_to must be YYYY-MM-DD"})
		return
	}

	if toTime.Before(fromTime) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date_to must be >= date_from"})
		return
	}

	toTime = toTime.Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	db := utils.GetDB()

	var clicks []models.ProductClick
	query := db.Where("direction = ? AND created_at BETWEEN ? AND ?", direction, fromTime, toTime)
	if key != "" {
		query = query.Where("key = ?", key)
	}

	if err := query.Order("created_at DESC").Find(&clicks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch clicks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": clicks,
	})
}

func (ac *AnalyticsController) GetTopClicks(c *gin.Context) {
	db := utils.GetDB()

	var clicks []models.ProductClick
	if err := db.Order("click_count DESC").Limit(10).Find(&clicks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch top clicks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": clicks,
	})
}

type DirectionStats struct {
	Direction   string `json:"direction"`
	TotalClicks int    `json:"total_clicks"`
}

func (ac *AnalyticsController) GetStatsByDirection(c *gin.Context) {
	db := utils.GetDB()

	var stats []DirectionStats
	if err := db.Model(&models.ProductClick{}).
		Select("direction, SUM(click_count) as total_clicks").
		Group("direction").
		Order("total_clicks DESC").
		Find(&stats).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result": stats,
	})
}
