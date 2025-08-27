package controllers

import (
	"net/http"
	"time"

	"kliro/services"

	"github.com/gin-gonic/gin"
)

type CurrencyController struct {
	currencyService *services.CurrencyService
}

func NewCurrencyController(currencyService *services.CurrencyService) *CurrencyController {
	return &CurrencyController{
		currencyService: currencyService,
	}
}

// GetLatestCurrencyRates получает последние курсы валют (split: buy/sell, отсортированные)
func (cc *CurrencyController) GetLatestCurrencyRates(c *gin.Context) {
	data, err := cc.currencyService.GetSplitSortedCurrencyRates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"result":  nil,
			"success": false,
			"error":   "Failed to get currency rates",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  data,
		"success": true,
	})
}

// GetCurrencyRatesByDate получает курсы валют за определенную дату
func (cc *CurrencyController) GetCurrencyRatesByDate(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"result":  nil,
			"success": false,
			"error":   "date parameter is required (format: YYYY-MM-DD)",
		})
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"result":  nil,
			"success": false,
			"error":   "Invalid date format. Use YYYY-MM-DD",
		})
		return
	}

	rates, err := cc.currencyService.GetCurrencyRatesByDate(date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"result":  nil,
			"success": false,
			"error":   "Failed to get currency rates for date",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  rates,
		"success": true,
	})
}
