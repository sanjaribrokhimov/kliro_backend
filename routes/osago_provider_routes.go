package routes

import (
	"kliro/config"
	"kliro/controllers"

	"github.com/gin-gonic/gin"
)

// SetupOsagoProviderRoutes регистрирует новые тестовые ручки расчёта ОСАГО по провайдерам.
func SetupOsagoProviderRoutes(r *gin.Engine) {
	cfg := config.LoadConfig()
	ctrl := controllers.NewOsagoProviderController(cfg)

	group := r.Group("/osago/providers")
	{
		group.POST("/calc/all", ctrl.CalcAll)
		group.POST("/calc/neo", ctrl.CalcNeo)
		group.POST("/calc/euroasia", ctrl.CalcEuroasia)
		group.POST("/calc/gross", ctrl.CalcGross)
	}
}
