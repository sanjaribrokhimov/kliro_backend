package insurance

import (
	"kliro/config"
	trustInsurancectl "kliro/controllers/trustInsurance"
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetupTrustInsuranceRoutes(r *gin.Engine) {
	cfg := config.LoadConfig()
	osagoController := trustInsurancectl.NewOsagoController(cfg)

	trustInsuranceGroup := r.Group("/trustInsurance")
	{
		// Test endpoint
		trustInsuranceGroup.GET("/test", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "Trust Insurance API is working!"})
		})

		// Test Trust Insurance API connectivity
		trustInsuranceGroup.GET("/test-api", func(c *gin.Context) {
			cfg := config.LoadConfig()
			resp, err := http.Get(cfg.TrustBaseURL)
			if err != nil {
				c.JSON(500, gin.H{"error": "Cannot reach Trust Insurance API", "details": err.Error()})
				return
			}
			defer resp.Body.Close()

			c.JSON(200, gin.H{
				"message": "Trust Insurance API is reachable",
				"status":  resp.StatusCode,
				"url":     cfg.TrustBaseURL,
			})
		})

		auth := trustInsuranceGroup.Group("/auth")
		{
			auth.POST("/login", osagoController.Login)
		}

		osago := trustInsuranceGroup.Group("/osago")
		{
			osago.POST("/create", osagoController.Create)
			osago.POST("/calc-prem", osagoController.CalcPrem)
		}

		reference := trustInsuranceGroup.Group("/reference")
		{
			reference.GET("/relatives", osagoController.Relatives)
		}

		provider := trustInsuranceGroup.Group("/provider")
		{
			provider.GET("/vehicle", osagoController.Vehicle)
			provider.GET("/passport-pinfl", osagoController.PassportPinfl)
			provider.GET("/passport-birth-date", osagoController.PassportBirthDate)
			provider.GET("/driver-summary", osagoController.DriverSummary)
		}
	}
}

func SetupTrustInsuranceRouterOnly() *gin.Engine {
	r := gin.Default()

	cfg := config.LoadConfig()
	osagoController := trustInsurancectl.NewOsagoController(cfg)

	trustInsuranceGroup := r.Group("/trustInsurance")
	{
		osago := trustInsuranceGroup.Group("/osago")
		{
			osago.POST("/create", osagoController.Create)
			osago.POST("/calc-prem", osagoController.CalcPrem)
		}

		reference := trustInsuranceGroup.Group("/reference")
		{
			reference.GET("/relatives", osagoController.Relatives)
		}

		provider := trustInsuranceGroup.Group("/provider")
		{
			provider.GET("/vehicle", osagoController.Vehicle)
			provider.GET("/passport-pinfl", osagoController.PassportPinfl)
			provider.GET("/passport-birth-date", osagoController.PassportBirthDate)
			provider.GET("/driver-summary", osagoController.DriverSummary)
		}
	}

	return r
}
