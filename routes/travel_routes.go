package routes

import (
	"kliro/controllers"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

func SetupTravelRoutes(r *gin.Engine, rdb *redis.Client) {
	travelController := controllers.NewTravelController(rdb)

	r.POST("/travel/purpose", travelController.SetTravelPurpose)
	r.POST("/travel/details", travelController.SetTravelDetails)
	r.POST("/travel/calculate", travelController.CalculateTravel)
}
