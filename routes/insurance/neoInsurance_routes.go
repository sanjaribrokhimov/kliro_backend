package insurance

import (
	"kliro/config"
	neoInsurancectl "kliro/controllers/neoInsurance"

	"github.com/gin-gonic/gin"
)

func SetupNeoInsuranceRoutes(r *gin.Engine) {
	cfg := config.LoadConfig()
	kaskoController := neoInsurancectl.NewKaskoController(cfg)
	osagoController := neoInsurancectl.NewOsagoController(cfg)
	travelController := neoInsurancectl.NewTravelController(cfg)

	neoInsuranceGroup := r.Group("/insurance")
	{
		kasko := neoInsuranceGroup.Group("/kasko")
		{
			kasko.GET("/cars", kaskoController.Cars)
			kasko.GET("/rates", kaskoController.GetTarif)
			kasko.POST("/car-price", kaskoController.CarPriceCalc)
			kasko.POST("/calculate", kaskoController.Calculate)
			kasko.POST("/save", kaskoController.Save)
			kasko.POST("/payment-link", kaskoController.GetPaymentLink)
			kasko.POST("/check-payment", kaskoController.CheckPayment)
			kasko.POST("/image-upload", kaskoController.ImageUpload)
		}

		osago := neoInsuranceGroup.Group("/osago")
		{
			osago.POST("/calculate", osagoController.Calc)
			osago.POST("/legal", osagoController.Juridik)
			osago.POST("/check-person", osagoController.CheckPerson)
			osago.POST("/save-policy", osagoController.SavePolicy)
			osago.POST("/confirm", osagoController.ConfirmPolicy)
			osago.POST("/status", osagoController.ConfirmCheck)
		}

		travel := neoInsuranceGroup.Group("/travel")
		{
			travel.GET("/simple/get-data", travelController.RiskGetData)
			travel.GET("/simple/get-country", travelController.RiskGetCountry)
			travel.POST("/simple/calculator", travelController.RiskCalculator)
			travel.POST("/simple/save", travelController.RiskSave)

			travel.GET("/full/get-data", travelController.TravelGetData)
			travel.POST("/full/calculator", travelController.TravelCalculatorTotal)
			travel.POST("/full/save", travelController.TravelSavePolis)
			travel.POST("/full/check", travelController.TravelCheckPolis)
			travel.POST("/full/passport-person", travelController.TravelPassportPerson)
		}
	}
}

func SetupNeoInsuranceRouterOnly() *gin.Engine {
	r := gin.Default()

	cfg := config.LoadConfig()
	kaskoController := neoInsurancectl.NewKaskoController(cfg)
	osagoController := neoInsurancectl.NewOsagoController(cfg)
	travelController := neoInsurancectl.NewTravelController(cfg)

	neoInsuranceGroup := r.Group("/neoInsurance")
	{
		kasko := neoInsuranceGroup.Group("/kasko")
		{
			kasko.GET("/cars", kaskoController.Cars)
			kasko.GET("/rates", kaskoController.GetTarif)
			kasko.POST("/car-price", kaskoController.CarPriceCalc)
			kasko.POST("/calculate", kaskoController.Calculate)
			kasko.POST("/save", kaskoController.Save)
			kasko.POST("/payment-link", kaskoController.GetPaymentLink)
			kasko.POST("/check-payment", kaskoController.CheckPayment)
			kasko.POST("/image-upload", kaskoController.ImageUpload)
		}

		osago := neoInsuranceGroup.Group("/osago")
		{
			osago.POST("/calculate", osagoController.Calc)
			osago.POST("/legal", osagoController.Juridik)
			osago.POST("/check-person", osagoController.CheckPerson)
			osago.POST("/save-policy", osagoController.SavePolicy)
			osago.POST("/confirm", osagoController.ConfirmPolicy)
			osago.POST("/status", osagoController.ConfirmCheck)
		}

		travel := neoInsuranceGroup.Group("/travel")
		{
			travel.GET("/simple/get-data", travelController.RiskGetData)
			travel.GET("/simple/get-country", travelController.RiskGetCountry)
			travel.POST("/simple/calculator", travelController.RiskCalculator)
			travel.POST("/simple/save", travelController.RiskSave)

			travel.GET("/full/get-data", travelController.TravelGetData)
			travel.POST("/full/calculator", travelController.TravelCalculatorTotal)
			travel.POST("/full/save", travelController.TravelSavePolis)
			travel.POST("/full/check", travelController.TravelCheckPolis)
			travel.POST("/full/passport-person", travelController.TravelPassportPerson)
		}
	}

	return r
}
