package routes

import (
	"kliro/config"
	"kliro/controllers"

	"github.com/gin-gonic/gin"
)

func SetupUnifiedRoutes(r *gin.Engine) {
	cfg := config.LoadConfig()
	unifiedController := controllers.NewUnifiedController(cfg)

	osago := r.Group("/osago")
	{
		osago.POST("/calc", unifiedController.Calculate)
		osago.POST("/create", unifiedController.Create)
		osago.POST("/check", unifiedController.CheckPaymentUnified)
	}
}
