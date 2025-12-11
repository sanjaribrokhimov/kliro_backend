package routes

import (
	"kliro/controllers/avia"

	"github.com/gin-gonic/gin"
)

func SetupAviaRoutes(router *gin.Engine) {
	aviaController := avia.NewAviaController()

	// Bukhara API routes (точные как в Postman коллекции)
	apiGroup := router.Group("/avia")
	{
		// Авторизация и баланс
		apiGroup.POST("/accounts/tokens", aviaController.Auth)
		apiGroup.GET("/accounts/check-balance", aviaController.CheckBalance)

		// Поиск рейсов
		apiGroup.GET("/offers", aviaController.SearchFlights)

		// Работа с офферами
		apiGroup.GET("/offers/:offer_id/fare-family", aviaController.GetFareFamily)
		apiGroup.GET("/offers/:offer_id", aviaController.CheckAvailability)
		apiGroup.GET("/offers/:offer_id/rules", aviaController.GetFareRules)
		apiGroup.POST("/offers/:offer_id/booking", aviaController.CreateBooking)

		// Работа с бронированиями offline
		apiGroup.GET("/booking/:booking_id", aviaController.GetBookingInfo)
		apiGroup.DELETE("/booking/:booking_id/cancel-unpaid", aviaController.CancelUnpaidBooking)
		apiGroup.GET("/booking/:booking_id/rules", aviaController.GetBookingRules)
		apiGroup.GET("/booking/:booking_id/check-price", aviaController.CheckPrice)
		apiGroup.GET("/booking/:booking_id/payment-permission", aviaController.CheckPaymentPermission)
		apiGroup.POST("/booking/:booking_id/payment", aviaController.PayBooking)
		apiGroup.DELETE("/booking/:booking_id/void", aviaController.VoidBooking)
		apiGroup.GET("/booking/:booking_id/get-refund-amounts", aviaController.GetRefundAmounts)
		apiGroup.DELETE("/booking/:booking_id/auto-cancel", aviaController.AutoCancel)
		apiGroup.GET("/booking/:booking_id/pdf-receipt", aviaController.GetPDFReceipt)
		apiGroup.POST("/booking/:booking_id/manual-refund", aviaController.ManualRefund)

		// Сервисы
		apiGroup.GET("/services/schedule", aviaController.GetSchedule)
		apiGroup.GET("/visa-types", aviaController.GetVisaTypes)
	}

	// Дополнительные routes
	aviaGroup := router.Group("/avia")
	{
		// Только airport-hints
		aviaGroup.GET("/airport-hints", aviaController.GetAirportHints)
	}
}
