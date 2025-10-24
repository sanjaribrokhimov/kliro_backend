package routes

import (
	"kliro/controllers"
	"kliro/middleware"

	"github.com/gin-gonic/gin"
)

// SetupInsuranceProfileRoutes настраивает маршруты для профиля страховки
func SetupInsuranceProfileRoutes(r *gin.Engine) {
	// Создаем контроллер
	insuranceProfileController := controllers.NewInsuranceProfileController()

	// Группа маршрутов для профиля страховки с JWT аутентификацией
	insuranceProfileGroup := r.Group("/api/insurance-profile", middleware.JWTAuthMiddleware())
	{
		// POST /api/insurance-profile - создание профиля страховки
		insuranceProfileGroup.POST("", insuranceProfileController.CreateInsuranceProfile)

		// GET /api/insurance-profile - получение всех профилей страховки пользователя
		insuranceProfileGroup.GET("", insuranceProfileController.GetInsuranceProfiles)

		// GET /api/insurance-profile/:id - получение конкретного профиля страховки
		insuranceProfileGroup.GET("/:id", insuranceProfileController.GetInsuranceProfileByID)
	}
}
