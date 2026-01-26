package routes

import (
	"kliro/config"
	"kliro/controllers"

	"github.com/gin-gonic/gin"
)

func SetupOsagoAllRoutes(r *gin.Engine) {
	cfg := config.LoadConfig()
	osagoAllController := controllers.NewOsagoAllController(cfg)

	// OSAGO All API routes
	osagoAllGroup := r.Group("/osago-all")
	{
		osagoAllGroup.POST("/find", osagoAllController.Find)
		osagoAllGroup.POST("/calc", osagoAllController.Calc)
		osagoAllGroup.POST("/create-policy", osagoAllController.CreatePolicy)
	}
}
