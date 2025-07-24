package controllers

import (
	"kliro/models"
	"kliro/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

type MicrocreditController struct{}

func NewMicrocreditController() *MicrocreditController {
	return &MicrocreditController{}
}

func (mc *MicrocreditController) GetNewMicrocredits(c *gin.Context) {
	db := utils.GetDB()
	var credits []models.Microcredit
	db.Table("new_microcredit").Order("bank_name").Find(&credits)
	c.JSON(http.StatusOK, gin.H{"result": credits, "success": true})
}

func (mc *MicrocreditController) GetOldMicrocredits(c *gin.Context) {
	db := utils.GetDB()
	var credits []models.Microcredit
	db.Table("old_microcredit").Order("bank_name").Find(&credits)
	c.JSON(http.StatusOK, gin.H{"result": credits, "success": true})
}
