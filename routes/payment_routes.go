package routes

import (
	"kliro/controllers"

	"github.com/gin-gonic/gin"
)

// SetupPaymentRoutes настраивает маршруты для платежей
func SetupPaymentRoutes(router *gin.Engine) {
	paymentCtrl := controllers.NewPaymentController()

	api := router.Group("/api")
	{
		// Платежи
		api.POST("/payment/create", paymentCtrl.CreatePayment)       // Создать платеж
		api.GET("/payment/:id", paymentCtrl.GetPaymentStatus)        // Статус платежа
		api.POST("/payment/:id/confirm", paymentCtrl.ConfirmPayment) // Подтвердить с OTP
		api.POST("/payment/:id/cancel", paymentCtrl.CancelPayment)   // Отменить платеж
		api.GET("/payments", paymentCtrl.GetPayments)                // Список платежей
		api.POST("/payment/callback", paymentCtrl.Callback)         // Callback от Multicard

		// Карты
		api.POST("/card/bind", paymentCtrl.BindCard)                    // Привязать карту
		api.GET("/card/bind/:session_id", paymentCtrl.GetCardBindingStatus) // Статус привязки
		api.GET("/cards", paymentCtrl.GetCards)                      // Список карт
		api.DELETE("/card/:id", paymentCtrl.DeleteCard)               // Удалить карту
	}
}

