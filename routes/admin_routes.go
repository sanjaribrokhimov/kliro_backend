package routes

import (
	"fmt"
	"kliro/controllers/admin"
	"kliro/utils"

	"github.com/gin-gonic/gin"
)

// SetupAdminRoutes настраивает админские маршруты
func SetupAdminRoutes(r *gin.Engine) {
	fmt.Println("==========================================")
	fmt.Println("DEBUG: Настраиваем админские routes...")
	fmt.Println("==========================================")

	// Получаем DB из utils
	db := utils.GetDB()
	if db == nil {
		fmt.Println("ERROR: DB is nil!")
		return
	}
	fmt.Println("DEBUG: DB получен успешно")

	// Создаем админский контроллер
	adminController := admin.NewAdminController(db)
	fmt.Println("DEBUG: AdminController создан")

	// Админские routes
	adminGroup := r.Group("/admin")
	{
		// Простой тестовый endpoint
		adminGroup.GET("/test", func(c *gin.Context) {
			fmt.Println("DEBUG: Тестовый endpoint вызван!")
			c.JSON(200, gin.H{"message": "Admin API работает!", "success": true})
		})

		// Статус парсинга всех сервисов
		adminGroup.GET("/parsing-status", adminController.GetParsingStatus)

		// Запуск парсинга для конкретного сервиса
		adminGroup.POST("/start-parsing/:service", adminController.StartParsing)

		// Получение данных конкретного сервиса
		adminGroup.GET("/service-data/:service", adminController.GetServiceData)

		// Перезапуск всех парсеров
		adminGroup.POST("/restart-all-parsers", adminController.RestartAllParsers)

		// Очистка всех данных парсеров
		adminGroup.DELETE("/clear-all-data", adminController.ClearAllParserData)

		// Системная информация
		adminGroup.GET("/system-info", adminController.GetSystemInfo)
	}

	fmt.Println("DEBUG: Админские routes настроены успешно")
	fmt.Println("==========================================")
}
