package routes

import (
	"fmt"
	"kliro/controllers"
	"kliro/middleware"
	"kliro/utils"

	"github.com/gin-gonic/gin"
)

// SetupOrderRoutes настраивает маршруты для работы с заказами
func SetupOrderRoutes(r *gin.Engine) {
	fmt.Println("==========================================")
	fmt.Println("DEBUG: Настраиваем order routes...")
	fmt.Println("==========================================")

	// Получаем DB из utils
	db := utils.GetDB()
	if db == nil {
		fmt.Println("ERROR: DB is nil!")
		return
	}
	fmt.Println("DEBUG: DB получен успешно для orders")

	// Создаем контроллер заказов
	orderController := controllers.NewOrderController(db)
	fmt.Println("DEBUG: OrderController создан")

	// Группа маршрутов для заказов (требует авторизации)
	orderGroup := r.Group("/user/orders", middleware.JWTAuthMiddleware())
	{
		// Создание заказа
		orderGroup.POST("/create", orderController.CreateOrder)

		// Получение заказов пользователя с пагинацией и фильтрами
		orderGroup.GET("/my-orders", orderController.GetUserOrders)

		// Получение статистики заказов пользователя
		orderGroup.GET("/my-stats", orderController.GetOrderStats)

		// Получение заказа по order_id
		orderGroup.GET("/:order_id", orderController.GetOrderByID)

		// Обновление статуса заказа
		orderGroup.PUT("/:order_id/status", orderController.UpdateOrderStatus)

		// Удаление заказа (soft delete)
		orderGroup.DELETE("/:order_id", orderController.DeleteOrder)
	}

	fmt.Println("DEBUG: Order routes настроены успешно")
	fmt.Println("==========================================")
}
