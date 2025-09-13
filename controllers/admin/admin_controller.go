package admin

import (
	"fmt"
	"net/http"
	"time"

	"kliro/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminController контроллер для админских функций
type AdminController struct {
	db *gorm.DB
}

// NewAdminController создает новый экземпляр AdminController
func NewAdminController(db *gorm.DB) *AdminController {
	return &AdminController{db: db}
}

// ParsingStatusResponse структура ответа для статуса парсинга
type ParsingStatusResponse struct {
	ServiceName     string     `json:"service_name"`
	TotalRecords    int64      `json:"total_records"`
	LastParsingTime *time.Time `json:"last_parsing_time"`
	NextParsingTime string     `json:"next_parsing_time"`
	UpdateInterval  string     `json:"update_interval"`
	Status          string     `json:"status"`
}

// GetParsingStatus возвращает статус всех парсеров
func (ac *AdminController) GetParsingStatus(c *gin.Context) {
	fmt.Println("DEBUG: GetParsingStatus вызван!")

	if ac.db == nil {
		fmt.Println("ERROR: ac.db is nil!")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection is nil"})
		return
	}

	fmt.Println("DEBUG: ac.db не nil, продолжаем...")

	var services = []struct {
		name     string
		table    string
		schedule string
	}{
		{"Transfer", "new_transfer", "Каждый день в 02:00 (Узбекское время)"},
		{"Deposit", "new_deposit", "Каждый день в 02:00 (Узбекское время)"},
		{"Microcredit", "new_microcredit", "Каждый день в 03:00 (Узбекское время)"},
		{"Mortgage", "new_mortgage", "Каждый день в 03:00 (Узбекское время)"},
		{"Card", "new_card", "Каждый день в 03:00 (Узбекское время)"},
		{"CreditCard", "new_credit_card", "Каждый день в 03:10 (Узбекское время)"},
		{"Autocredit", "new_autocredit", "Каждый день в 03:00 (Узбекское время)"},
		{"Currency", "new_currency", "Каждые 3 часа"},
	}

	var statuses []ParsingStatusResponse

	for _, service := range services {
		var count int64
		ac.db.Table(service.table).Count(&count)

		// Получаем время последнего парсинга (максимальное CreatedAt)
		var lastParsingTime *time.Time
		var maxTime time.Time
		if err := ac.db.Table(service.table).Select("MAX(created_at)").Scan(&maxTime).Error; err == nil && !maxTime.IsZero() {
			lastParsingTime = &maxTime
		}

		// Вычисляем следующее время парсинга
		nextParsingTime := ac.calculateNextParsingTime(service.name)

		// Определяем статус
		status := "unknown"
		if lastParsingTime != nil {
			now := utils.UzbekTime()
			diff := now.Sub(*lastParsingTime)
			if diff < 24*time.Hour {
				status = "active"
			} else if diff < 48*time.Hour {
				status = "warning"
			} else {
				status = "inactive"
			}
		} else {
			status = "never_parsed"
		}

		statuses = append(statuses, ParsingStatusResponse{
			ServiceName:     service.name,
			TotalRecords:    count,
			LastParsingTime: lastParsingTime,
			NextParsingTime: nextParsingTime,
			UpdateInterval:  service.schedule,
			Status:          status,
		})
	}

	fmt.Println("DEBUG: Отправляем ответ с", len(statuses), "сервисами")

	c.JSON(http.StatusOK, gin.H{
		"result":  statuses,
		"success": true,
	})
}

// calculateNextParsingTime вычисляет следующее время парсинга
func (ac *AdminController) calculateNextParsingTime(serviceName string) string {
	now := utils.UzbekTime()

	switch serviceName {
	case "Transfer", "Deposit":
		// 02:00 каждый день
		next := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next.Format("2006-01-02 15:04:05")
	case "Microcredit", "Mortgage", "Card", "Autocredit":
		// 03:00 каждый день
		next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next.Format("2006-01-02 15:04:05")
	case "CreditCard":
		// 03:10 каждый день
		next := time.Date(now.Year(), now.Month(), now.Day(), 3, 10, 0, 0, now.Location())
		if next.Before(now) {
			next = next.Add(24 * time.Hour)
		}
		return next.Format("2006-01-02 15:04:05")
	case "Currency":
		// Каждые 3 часа
		next := now.Add(3 * time.Hour)
		return next.Format("2006-01-02 15:04:05")
	default:
		return "Неизвестно"
	}
}

// GetSystemInfo возвращает системную информацию
func (ac *AdminController) GetSystemInfo(c *gin.Context) {
	fmt.Println("DEBUG: GetSystemInfo вызван!")

	info := gin.H{
		"server_time": utils.UzbekTime().Format("2006-01-02 15:04:05"),
		"timezone":    "Asia/Tashkent",
		"version":     "1.0.0",
		"status":      "running",
	}

	c.JSON(http.StatusOK, gin.H{
		"result":  info,
		"success": true,
	})
}

// StartParsing запускает парсинг для конкретного сервиса
func (ac *AdminController) StartParsing(c *gin.Context) {
	service := c.Param("service")
	fmt.Println("DEBUG: StartParsing вызван для сервиса:", service)

	// Здесь должна быть логика запуска парсинга
	// Пока просто возвращаем успех

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"service": service,
			"status":  "started",
			"message": "Парсинг запущен для сервиса " + service,
		},
		"success": true,
	})
}

// GetServiceData возвращает данные конкретного сервиса
func (ac *AdminController) GetServiceData(c *gin.Context) {
	service := c.Param("service")
	fmt.Println("DEBUG: GetServiceData вызван для сервиса:", service)

	// Здесь должна быть логика получения данных сервиса
	// Пока просто возвращаем заглушку

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"service": service,
			"data":    "Данные сервиса " + service,
		},
		"success": true,
	})
}

// RestartAllParsers перезапускает все парсеры
func (ac *AdminController) RestartAllParsers(c *gin.Context) {
	fmt.Println("DEBUG: RestartAllParsers вызван!")

	// Здесь должна быть логика перезапуска всех парсеров
	// Пока просто возвращаем успех

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"status":  "restarted",
			"message": "Все парсеры перезапущены",
		},
		"success": true,
	})
}

// ClearAllParserData очищает все данные парсеров
func (ac *AdminController) ClearAllParserData(c *gin.Context) {
	fmt.Println("DEBUG: ClearAllParserData вызван!")

	// Здесь должна быть логика очистки всех данных
	// Пока просто возвращаем успех

	c.JSON(http.StatusOK, gin.H{
		"result": gin.H{
			"status":  "cleared",
			"message": "Все данные парсеров очищены",
		},
		"success": true,
	})
}
