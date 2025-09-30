package routes

import (
	"kliro/config"
	"kliro/controllers"

	"github.com/gin-gonic/gin"
)

func SetupUnifiedRoutes(r *gin.Engine) {
	cfg := config.LoadConfig()
	unifiedController := controllers.NewUnifiedController(cfg)

	unifiedGroup := r.Group("/osago")
	{
		osago := unifiedGroup.Group("/unified")
		{
			osago.POST("/check", unifiedController.Nacalo)
			osago.POST("/calc", unifiedController.Calc)
			osago.POST("/create", unifiedController.InitCon)
			osago.POST("/save", unifiedController.Submit)
			osago.POST("/check-payment", unifiedController.CheckPayment)
			osago.GET("/session/:id", unifiedController.GetSession)
		}
	}
}
