package routes

import (
	"kliro/controllers"

	"github.com/gin-gonic/gin"
)

// SetupPaymentMulticardRoutes настраивает прозрачные прокси-роуты для Multicard Payment Page
func SetupPaymentMulticardRoutes(router *gin.Engine) {
	ctrl := controllers.NewPaymentMulticardController()
	ui := controllers.NewPaymentMulticardUIController()

	// 1) Создание инвойса (POST)
	router.POST("/payment/invoice", ctrl.CreateInvoice)
	// 2) Получение информации об инвойсе (GET)
	router.GET("/payment/invoice/:uuid", ctrl.GetInvoice)
	// 3) Удаление (аннулирование) инвойса (DELETE)
	router.DELETE("/payment/invoice/:uuid", ctrl.DeleteInvoice)
	// 4) Быстрая оплата (PUT)
	router.PUT("/payment/:uuid/scanpay", ctrl.QuickPay)
	// 5) Callback (success)
	router.POST("/payment/callback/success", ctrl.CallbackSuccess)
	// 6) Callback (webhooks)
	router.POST("/payment/callback/webhooks", ctrl.CallbackWebhooks)

	// UI вспомогательные точки
	router.GET("/payment/ui/status", ui.GetStatus)
	router.GET("/payment/return", ui.Return)
	router.GET("/payment/error", ui.Error)
}
