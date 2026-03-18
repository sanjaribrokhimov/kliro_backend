package routes

import (
	"kliro/config"
	"kliro/controllers"

	"github.com/gin-gonic/gin"
)

func SetupKaskoAllRoutes(r *gin.Engine) {
	cfg := config.LoadConfig()
	kaskoAll := controllers.NewKaskoAllController(cfg)

	g := r.Group("/kasko-all")
	{
		g.POST("/start", kaskoAll.Start)
		g.POST("/calculate", kaskoAll.Calculate)

		lookups := g.Group("/lookups")
		{
			lookups.GET("/providers", kaskoAll.LookupsProviders)

			lookups.GET("/neo/cars", kaskoAll.LookupsNeoCars)
			lookups.GET("/neo/tariffs", kaskoAll.LookupsNeoTariffs)
			lookups.GET("/neo/mini-tariffs", kaskoAll.LookupsNeoMiniTariffs)

			lookups.GET("/trust/marks", kaskoAll.LookupsTrustMarks)
			lookups.POST("/trust/models", kaskoAll.LookupsTrustModels) // expects query ?id=MARK_ID for now

			lookups.GET("/gross/brands", kaskoAll.LookupsGrossBrands)
			lookups.POST("/gross/models", kaskoAll.LookupsGrossModels) // ?autobrand_id=
			lookups.POST("/gross/comps", kaskoAll.LookupsGrossComps)   // ?automodel_id=
			lookups.GET("/gross/years", kaskoAll.LookupsGrossYears)
			lookups.GET("/gross/tariffs", kaskoAll.LookupsGrossTariffs)

			lookups.GET("/euroasia/risks", kaskoAll.LookupsEuroAsiaRisks)
		}
	}
}

