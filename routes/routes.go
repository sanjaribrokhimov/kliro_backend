package routes

import (
	"kliro/controllers"
	"kliro/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// SetupRouter создаёт gin.Engine, регистрирует все маршруты и возвращает роутер
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// CORS middleware ДО роутов
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://kliro.uz"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Здесь инициализируй зависимости (например, Redis)
	// Для тестов можно использовать in-memory Redis или мок
	// Пример с реальным Redis:
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	userController := controllers.NewUserController(rdb)
	userProfileController := controllers.NewUserProfileController(rdb)

	r.POST("/auth/register", userController.Register)
	r.POST("/auth/confirm-otp", userController.ConfirmOTP)
	r.POST("/auth/confirm-otp-create", userController.ConfirmOTPCreate)
	r.POST("/auth/set-region-password-final", userController.SetRegionPasswordFinal)
	r.POST("/auth/login", userController.Login)
	r.POST("/auth/forgot-password", userController.ForgotPassword)
	r.POST("/auth/reset-password", userController.ResetPassword)
	r.GET("/auth/google", userController.GoogleLogin)
	r.GET("/auth/google/callback", userController.GoogleCallback)
	r.POST("/auth/google/complete", userController.GoogleComplete)

	userGroup := r.Group("/user", middleware.JWTAuthMiddleware())
	{
		userGroup.GET("/profile", userProfileController.GetProfile)
		userGroup.POST("/update-contact", userProfileController.UpdateContact)
		userGroup.POST("/confirm-update-contact", userProfileController.ConfirmUpdateContact)
		userGroup.POST("/change-password", userProfileController.ChangePassword)
		userGroup.POST("/change-region", userProfileController.ChangeRegion)
		userGroup.POST("/add-contact", userProfileController.AddContact)
		userGroup.POST("/logout", userProfileController.Logout)
	}

	return r
}
