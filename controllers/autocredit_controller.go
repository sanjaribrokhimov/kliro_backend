package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"kliro/utils"
)

type AutocreditController struct{}

func NewAutocreditController() *AutocreditController {
	return &AutocreditController{}
}

// GetNewAutocredits получает новые автокредиты
func (ac *AutocreditController) GetNewAutocredits(c *gin.Context) {
	db := utils.GetDB()

	var autocredits []map[string]interface{}
	if err := db.Table("new_autocredit").Find(&autocredits).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"result":  nil,
			"success": false,
			"error":   "Failed to get autocredits",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  autocredits,
		"success": true,
	})
}

// GetOldAutocredits получает старые автокредиты
func (ac *AutocreditController) GetOldAutocredits(c *gin.Context) {
	db := utils.GetDB()

	var autocredits []map[string]interface{}
	if err := db.Table("old_autocredit").Find(&autocredits).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"result":  nil,
			"success": false,
			"error":   "Failed to get old autocredits",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  autocredits,
		"success": true,
	})
}
