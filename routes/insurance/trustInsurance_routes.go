package insurance

import (
	"kliro/config"
	trustInsurancectl "kliro/controllers/trustInsurance"

	"github.com/gin-gonic/gin"
)

func SetupTrustInsuranceRoutes(r *gin.Engine) {
	cfg := config.LoadConfig()
	accidentController := trustInsurancectl.NewAccidentController(cfg)

	trustInsuranceGroup := r.Group("/trust-insurance")
	{
		accident := trustInsuranceGroup.Group("/accident")
		{
			accident.GET("/tarifs", accidentController.GetTariffs)
			accident.POST("/create", accidentController.Create)
			accident.POST("/check-payment", accidentController.CheckPayment)
		}
	}
}
