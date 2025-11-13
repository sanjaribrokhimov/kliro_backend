package routes

import (
	"kliro/controllers/hotels"

	"github.com/gin-gonic/gin"
)

func SetupHotelRoutes(router *gin.Engine) {
	hotelController := hotels.NewHotelController()

	// Hotelios API routes (точные как в оригинале, но сгруппированы под /hotels)
	hotelsGroup := router.Group("/hotels")
	{
		// ===== СПРАВОЧНЫЕ МЕТОДЫ (v1.0) - оригинальные пути =====

		// Справочники - уникальные пути для каждого метода
		hotelsGroup.POST("/countries", hotelController.GetCountryList)                          // GetCountryList
		hotelsGroup.POST("/regions", hotelController.GetRegionList)                             // GetRegionList
		hotelsGroup.POST("/cities", hotelController.GetCityList)                                // GetCityList
		hotelsGroup.POST("/types", hotelController.GetHotelTypeList)                            // GetHotelTypeList
		hotelsGroup.POST("/list", hotelController.GetHotelList)                                 // GetHotelList
		hotelsGroup.POST("/photos", hotelController.GetHotelPhotoList)                          // GetHotelPhotoList
		hotelsGroup.POST("/room-types", hotelController.GetHotelRoomTypeList)                   // GetHotelRoomTypeList
		hotelsGroup.POST("/room-photos", hotelController.GetHotelRoomTypesPhotoList)            // GetHotelRoomTypesPhotoList
		hotelsGroup.POST("/facilities", hotelController.GetFacilityList)                        // GetFacilityList
		hotelsGroup.POST("/hotel-facilities", hotelController.GetHotelFacilityList)             // GetHotelFacilityList
		hotelsGroup.POST("/equipment", hotelController.GetEquipmentList)                        // GetEquipmentList
		hotelsGroup.POST("/room-equipment", hotelController.GetRoomTypeEquipmentList)           // GetRoomTypeEquipmentList
		hotelsGroup.POST("/price-range", hotelController.GetPriceRange)                         // GetPriceRange
		hotelsGroup.POST("/stars", hotelController.GetStarList)                                 // GetStarList
		hotelsGroup.POST("/nearby-places-types", hotelController.GetNearbyPlacesTypeList)       // GetNearbyPlacesTypeList
		hotelsGroup.POST("/hotel-nearby-places", hotelController.GetHotelNearbyPlacesList)      // GetHotelNearbyPlacesList
		hotelsGroup.POST("/services-in-room", hotelController.GetServicesInRoomList)            // GetServicesInRoomList
		hotelsGroup.POST("/hotel-services-in-room", hotelController.GetHotelServicesInRoomList) // GetHotelServicesInRoomList
		hotelsGroup.POST("/bed-types", hotelController.GetBedTypeList)                          // GetBedTypeList
		hotelsGroup.POST("/currencies", hotelController.GetCurrencyList)                        // GetCurrencyList

	}

	// ===== BOOKING-FLOW API (v1.1.0) - оригинальные пути =====
	bookingFlowGroup := router.Group("/hotels")
	{
		bookingFlowGroup.POST("/search", hotelController.BookingFlowSearch)
		bookingFlowGroup.POST("/quote", hotelController.BookingFlowQuote)
		bookingFlowGroup.POST("/booking/create", hotelController.BookingFlowCreate)
		bookingFlowGroup.POST("/booking/confirm", hotelController.BookingFlowConfirm)
		bookingFlowGroup.POST("/booking/cancel", hotelController.BookingFlowCancel)
		bookingFlowGroup.POST("/booking/read", hotelController.BookingFlowRead)
	}
}
