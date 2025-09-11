package routes

import (
	hotels "kliro/controllers/hotels"

	"github.com/gin-gonic/gin"
)

func SetupHotelRoutes(router *gin.Engine) {
	hotelController := hotels.NewHotelController()

	// Группа маршрутов для отелей
	hotelGroup := router.Group("/api/hotels")
	{
		// Справочники
		hotelGroup.GET("/countries", hotelController.GetCountryList)
		hotelGroup.GET("/regions", hotelController.GetRegionList)
		hotelGroup.GET("/cities", hotelController.GetCityList)
		hotelGroup.GET("/types", hotelController.GetHotelTypeList)
		hotelGroup.GET("/facilities", hotelController.GetFacilityList)
		hotelGroup.GET("/equipment", hotelController.GetEquipmentList)
		hotelGroup.GET("/nearby-places", hotelController.GetNearbyPlacesList)
		hotelGroup.GET("/free-services", hotelController.GetFreeServicesList)
		hotelGroup.GET("/bed-types", hotelController.GetBedTypeList)
		hotelGroup.GET("/stars", hotelController.GetHotelStarList)
		hotelGroup.GET("/currencies", hotelController.GetCurrencyList)
		hotelGroup.GET("/price-range", hotelController.GetMinMaxPrice)

		// Информация об отелях
		hotelGroup.GET("/", hotelController.GetHotelList)
		hotelGroup.GET("/photos", hotelController.GetHotelPhotosList)
		hotelGroup.GET("/facilities-list", hotelController.GetHotelFacilitiesList)
		hotelGroup.GET("/services", hotelController.GetHotelServicesInRoomList)
		hotelGroup.GET("/nearby", hotelController.GetHotelNearbyPlacesList)

		// Информация о номерах
		hotelGroup.GET("/room-types", hotelController.GetHotelRoomTypeList)
		hotelGroup.GET("/room-photos", hotelController.GetHotelRoomTypesPhotoList)
		hotelGroup.GET("/room-equipment", hotelController.GetRoomTypeEquipmentList)

		// Резервация
		hotelGroup.POST("/search", hotelController.SearchHotels)
		hotelGroup.POST("/available-rooms", hotelController.GetAvailableRoomsByHotel)
		hotelGroup.POST("/reservation", hotelController.MakeReservation)
		hotelGroup.POST("/reservation/status", hotelController.GetReservationStatus)
		hotelGroup.POST("/reservation/details", hotelController.GetReservationDetails)
		hotelGroup.POST("/reservation/confirm", hotelController.ConfirmReservation)
		hotelGroup.POST("/reservation/cancel", hotelController.CancelReservation)
	}
}
