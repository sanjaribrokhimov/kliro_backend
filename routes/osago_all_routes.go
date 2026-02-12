package routes

import (
	"kliro/config"
	"kliro/controllers"
	"kliro/controllers/osagoCreate"

	"github.com/gin-gonic/gin"
)

func SetupOsagoAllRoutes(r *gin.Engine) {
	cfg := config.LoadConfig()
	osagoAllController := controllers.NewOsagoAllController(cfg)
	osagoCreateController := osagoCreate.NewOsagoCreateController(cfg)

	// OSAGO All API routes
	osagoAllGroup := r.Group("/osago-all")
	{
		osagoAllGroup.POST("/find", osagoAllController.Find)
		osagoAllGroup.POST("/calculate", osagoAllController.Calculate)
		osagoAllGroup.POST("/create", osagoCreateController.Create)
	}
}
