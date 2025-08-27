package routes

import (
	"kliro/controllers"
	"kliro/middleware"
	"kliro/services"
	"kliro/utils"

	"kliro/config"
	insurancectl "kliro/controllers/insurance"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// SetupRouter создаёт gin.Engine, регистрирует все маршруты и возвращает роутер
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// CORS middleware ДО роутов
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://kliro.uz", "https://www.kliro.uz", "https://kliro-frontend.vercel.app"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Инициализируем БД
	db := utils.GetDB()

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

	// Инициализируем сервисы
	currencyService := services.NewCurrencyService(db)

	// Инициализируем контроллеры
	parserController := controllers.NewParserController(currencyService)
	microcreditController := controllers.NewMicrocreditController()
	autocreditController := controllers.NewAutocreditController()
	transferController := controllers.NewTransferController()
	mortgageController := controllers.NewMortgageController(db)
	depositController := controllers.NewDepositController()
	cardController := controllers.NewCardController()
	currencyController := controllers.NewCurrencyController(currencyService)

	// Конфиг и контроллер страхования
	cfg := config.LoadConfig()
	kaskoController := insurancectl.NewKaskoController(cfg)
	osagoController := insurancectl.NewOsagoController(cfg)
	travelController := insurancectl.NewTravelController(cfg)

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
	// Bank group for all bank-related endpoints
	bankGroup := r.Group("/bank")
	{
		// Parser endpoints
		bankGroup.GET("/parse", parserController.ParsePage)
		bankGroup.GET("/parse-currency", parserController.ParseCurrencyPage)
		bankGroup.GET("/parse-autocredit", parserController.ParseAutocreditPage)
		bankGroup.GET("/parse-transfer", parserController.ParseTransferPage)
		bankGroup.GET("/parse-transfer-goquery", transferController.ParseTransfer)
		bankGroup.GET("/parse-mortgage", parserController.ParseMortgagePage)
		bankGroup.GET("/parse-mortgage-goquery", mortgageController.ParseMortgage)
		bankGroup.GET("/parse-deposit", parserController.ParseDepositPage)
		bankGroup.GET("/parse-card", parserController.ParseCardPage)
		bankGroup.GET("/update-transfers", parserController.ParseTransferAndUpdateDatabase)

		// Data endpoints
		bankGroup.GET("/microcredits/new", microcreditController.GetNewMicrocredits)
		bankGroup.GET("/autocredits/new", autocreditController.GetNewAutocredits)
		bankGroup.GET("/transfers/new", transferController.GetNewTransfers)
		bankGroup.GET("/mortgages/new", mortgageController.GetNewMortgages)
		bankGroup.GET("/deposits/new", depositController.GetNewDeposits)
		bankGroup.GET("/cards/new", cardController.GetNewCards)
		bankGroup.GET("/credit-cards/new", cardController.GetNewCreditCards)
		bankGroup.GET("/currencies/new", currencyController.GetLatestCurrencyRates)
		bankGroup.GET("/currencies/by-date", currencyController.GetCurrencyRatesByDate)
	}

	// Insurance group (proxy to NeoInsurance APIs)
	insuranceGroup := r.Group("/insurance")
	{
		kasko := insuranceGroup.Group("/kasko")
		{
			kasko.GET("/cars", kaskoController.Cars)
			kasko.GET("/rates", kaskoController.GetTarif)
			kasko.POST("/car-price", kaskoController.CarPriceCalc)
			kasko.POST("/calculate", kaskoController.Calculate)
			kasko.POST("/save", kaskoController.Save)
			kasko.POST("/payment-link", kaskoController.GetPaymentLink)
			kasko.POST("/check-payment", kaskoController.CheckPayment)
			kasko.POST("/image-upload", kaskoController.ImageUpload)
		}

		osago := insuranceGroup.Group("/osago")
		{
			osago.POST("/calculate", osagoController.Calc)
			osago.POST("/legal", osagoController.Juridik)
			osago.POST("/check-person", osagoController.CheckPerson)
			osago.POST("/save-policy", osagoController.SavePolicy)
			osago.POST("/confirm", osagoController.ConfirmPolicy)
			osago.POST("/status", osagoController.ConfirmCheck)
		}

		// Travel group (no JWT)
		travel := insuranceGroup.Group("/travel")
		{
			// Simple Travel API (упрощенный продукт)
			travel.GET("/simple/get-data", travelController.RiskGetData)
			travel.GET("/simple/get-country", travelController.RiskGetCountry)
			travel.POST("/simple/calculator", travelController.RiskCalculator)
			travel.POST("/simple/save", travelController.RiskSave)

			// Full Travel API (полноценный продукт)
			travel.GET("/full/get-data", travelController.TravelGetData)
			travel.POST("/full/calculator", travelController.TravelCalculatorTotal)
			travel.POST("/full/save", travelController.TravelSavePolis)
			travel.POST("/full/check", travelController.TravelCheckPolis)
			travel.POST("/full/passport-person", travelController.TravelPassportPerson)
		}
	}

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

// SetupInsuranceRouterOnly поднимает только страховые маршруты (без БД/кронов)
func SetupInsuranceRouterOnly() *gin.Engine {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "https://kliro.uz", "https://www.kliro.uz", "https://kliro-frontend.vercel.app"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	cfg := config.LoadConfig()
	kaskoController := insurancectl.NewKaskoController(cfg)
	osagoController := insurancectl.NewOsagoController(cfg)

	insuranceGroup := r.Group("/insurance")
	{
		kasko := insuranceGroup.Group("/kasko")
		{
			kasko.GET("/cars", kaskoController.Cars)
			kasko.GET("/rates", kaskoController.GetTarif)
			kasko.POST("/car-price", kaskoController.CarPriceCalc)
			kasko.POST("/calculate", kaskoController.Calculate)
			kasko.POST("/save", kaskoController.Save)
			kasko.POST("/payment-link", kaskoController.GetPaymentLink)
			kasko.POST("/check-payment", kaskoController.CheckPayment)
			kasko.POST("/image-upload", kaskoController.ImageUpload)
		}

		osago := insuranceGroup.Group("/osago")
		{
			osago.POST("/calculate", osagoController.Calc)
			osago.POST("/legal", osagoController.Juridik)
			osago.POST("/check-person", osagoController.CheckPerson)
			osago.POST("/save-policy", osagoController.SavePolicy)
			osago.POST("/confirm", osagoController.ConfirmPolicy)
			osago.POST("/status", osagoController.ConfirmCheck)
		}
	}

	return r
}
