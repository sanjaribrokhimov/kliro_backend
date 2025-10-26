package controllers

import (
	"net/http"

	"kliro/utils"

	"github.com/gin-gonic/gin"
)

func TestError(c *gin.Context) {
	utils.LogError(nil, "Test error logging")
	c.JSON(http.StatusOK, gin.H{"message": "Error logged successfully"})
}

func TestPanic(c *gin.Context) {
	panic("Test panic for logging")
}
