package routes

import (
	"kliro/controllers"

	"github.com/gin-gonic/gin"
)

func SetupAnalyticsRoutes(r *gin.Engine) {
	analyticsController := controllers.NewAnalyticsController()

	analyticsGroup := r.Group("/analytics/bank")
	{
		analyticsGroup.POST("/track-click", analyticsController.TrackClick)
		analyticsGroup.GET("/clicks", analyticsController.GetAllClicks)
		analyticsGroup.GET("/clicks/:direction", analyticsController.GetClicksByDirection)
		analyticsGroup.GET("/clicks/:direction/by-date", analyticsController.GetClicksByDirectionAndDate)
		analyticsGroup.GET("/top-clicks", analyticsController.GetTopClicks)
		analyticsGroup.GET("/stats-by-direction", analyticsController.GetStatsByDirection)
	}
}
