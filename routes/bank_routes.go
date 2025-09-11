package routes

import (
	bank "kliro/controllers/bank"
	bankServices "kliro/services/bank"
	"kliro/utils"

	"github.com/gin-gonic/gin"
)

func SetupBankRoutes(router *gin.Engine) {
	// Инициализируем БД
	db := utils.GetDB()

	// Инициализируем сервисы
	currencyService := bankServices.NewCurrencyService(db)

	// Инициализируем контроллеры
	microcreditController := bank.NewMicrocreditController()
	autocreditController := bank.NewAutocreditController()
	transferController := bank.NewTransferController()
	mortgageController := bank.NewMortgageController(db)
	depositController := bank.NewDepositController()
	cardController := bank.NewCardController()
	currencyController := bank.NewCurrencyController(currencyService)

	// Bank group for all bank-related endpoints
	bankGroup := router.Group("/bank")
	{
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
}
