package routes

import (
	"kliro/controllers"

	"github.com/gin-gonic/gin"
)

func SetupBlogRoutes(r *gin.Engine) {
	blogController := controllers.NewBlogController()
	grp := r.Group("/blog")
	{
		// Загрузка фото блога (multipart/form-data, поле "file"), возвращает URL
		grp.POST("upload-photo", blogController.UploadPhoto)

		grp.POST("create", blogController.Create)
		grp.GET("list", blogController.List)
		grp.GET("get/:id", blogController.GetByID)
		grp.PUT("update/:id", blogController.Update)
		grp.DELETE("delete/:id", blogController.Delete)
	}
}


