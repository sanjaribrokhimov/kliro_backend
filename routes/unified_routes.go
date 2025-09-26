package routes

import (
	"kliro/config"
	"kliro/controllers"

	"github.com/gin-gonic/gin"
)

func SetupUnifiedRoutes(r *gin.Engine) {
	cfg := config.LoadConfig()
	unifiedController := controllers.NewUnifiedController(cfg)

	unifiedGroup := r.Group("/unified")
	{
		osago := unifiedGroup.Group("/osago")
		{
			osago.POST("/nacalo", unifiedController.Nacalo)
			osago.POST("/calc", unifiedController.Calc)
			osago.GET("/session/:id", unifiedController.GetSession)
		}
	}
}
