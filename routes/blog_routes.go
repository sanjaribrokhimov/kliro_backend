package routes

import (
	"kliro/controllers"

	"github.com/gin-gonic/gin"
)

func SetupBlogRoutes(r *gin.Engine) {
	blogController := controllers.NewBlogController()
	grp := r.Group("/blog")
	{
		grp.POST("", blogController.Create)
		grp.GET("", blogController.List)
		grp.GET("/:id", blogController.GetByID)
		grp.PUT("/:id", blogController.Update)
		grp.DELETE("/:id", blogController.Delete)
	}
}


