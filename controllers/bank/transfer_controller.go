package bank

import (
	"kliro/models"
	bankServices "kliro/services/bank"
	"kliro/utils"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type TransferController struct{}

func NewTransferController() *TransferController {
	return &TransferController{}
}

// TransferResponseByPagination структура ответа с пагинацией для переводов
type TransferResponseByPagination struct {
	TotalPages       int               `json:"totalPages"`
	TotalElements    int64             `json:"totalElements"`
	First            bool              `json:"first"`
	Last             bool              `json:"last"`
	Size             int               `json:"size"`
	Content          []models.Transfer `json:"content"`
	Number           int               `json:"number"`
	Sort             []Sort            `json:"sort"`
	NumberOfElements int               `json:"numberOfElements"`
	Pageable         Pageable          `json:"pageable"`
	Empty            bool              `json:"empty"`
}

// GetNewTransfers получает новые переводы с пагинацией
func (tc *TransferController) GetNewTransfers(c *gin.Context) {
	tc.getTransfersWithPagination(c, "new_transfer")
}

// ParseTransfer парсит переводы с указанного URL
func (tc *TransferController) ParseTransfer(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"result":  nil,
			"success": false,
			"error":   "url parameter is required",
		})
		return
	}

	parser := bankServices.NewTransferParser()
	transfers, err := parser.ParseURL(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"result":  nil,
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if len(transfers) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"result":  nil,
			"success": true,
			"error":   "No transfers found",
		})
		return
	}

	// Возвращаем первый найденный перевод
	c.JSON(http.StatusOK, gin.H{
		"result":  transfers[0],
		"success": true,
		"error":   nil,
	})
}

// getTransfersWithPagination общая функция для получения переводов с пагинацией
func (tc *TransferController) getTransfersWithPagination(c *gin.Context, tableName string) {
	db := utils.GetDB()

	// Параметры пагинации
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	sortBy := c.DefaultQuery("sort", "app_name")
	sortDir := c.DefaultQuery("direction", "asc")

	// Валидация параметров
	if page < 0 {
		page = 0
	}
	if size < 1 || size > 100 {
		size = 10
	}

	// Подсчет общего количества записей
	var totalElements int64
	db.Table(tableName).Count(&totalElements)

	// Вычисление пагинации
	totalPages := int((totalElements + int64(size) - 1) / int64(size))
	// Проверяем, есть ли данные на последней странице
	if totalPages > 0 {
		lastPageOffset := (totalPages - 1) * size
		var lastPageCount int64
		db.Table(tableName).Offset(lastPageOffset).Limit(size).Count(&lastPageCount)
		if lastPageCount == 0 {
			totalPages = totalPages - 1
		}
	}
	offset := page * size

	// Проверка на пустой результат
	if totalElements == 0 {
		response := TransferResponseByPagination{
			TotalPages:       0,
			TotalElements:    0,
			First:            true,
			Last:             true,
			Size:             size,
			Content:          []models.Transfer{},
			Number:           page,
			Sort:             []Sort{},
			NumberOfElements: 0,
			Pageable: Pageable{
				Offset:     offset,
				Sort:       []Sort{},
				Paged:      true,
				PageNumber: page,
				PageSize:   size,
				Unpaged:    false,
			},
			Empty: true,
		}
		c.JSON(http.StatusOK, gin.H{"result": response, "success": true})
		return
	}

	// Создание сортировки
	sortDirection := "ASC"
	if strings.ToLower(sortDir) == "desc" {
		sortDirection = "DESC"
	}

	// Валидация поля сортировки для переводов
	allowedSortFields := map[string]string{
		"app_name":   "app_name",
		"commission": "commission",
		"limit_info": "limit_info",
		"created_at": "created_at",
	}

	sortField, exists := allowedSortFields[sortBy]
	if !exists {
		sortField = "app_name"
	}

	// Выполнение запроса с пагинацией и сортировкой
	var transfers []models.Transfer
	query := db.Table(tableName)

	// Применение сортировки
	if sortField == "app_name" {
		// Для текстовых полей добавляем COLLATE для правильной сортировки
		if sortDirection == "ASC" {
			query = query.Order("app_name COLLATE \"C\" ASC")
		} else {
			query = query.Order("app_name COLLATE \"C\" DESC")
		}
	} else {
		query = query.Order(sortField + " " + sortDirection)
	}

	// Применение пагинации
	query = query.Offset(offset).Limit(size)

	if err := query.Find(&transfers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
		return
	}

	// Создание объекта сортировки
	sortObj := Sort{
		Direction:    strings.ToUpper(sortDir),
		NullHandling: "NATIVE",
		Ascending:    strings.ToLower(sortDir) == "asc",
		Property:     sortBy,
		IgnoreCase:   false,
	}

	// Формирование ответа
	response := TransferResponseByPagination{
		TotalPages:       totalPages,
		TotalElements:    totalElements,
		First:            page == 0,
		Last:             page >= totalPages-1,
		Size:             size,
		Content:          transfers,
		Number:           page,
		Sort:             []Sort{sortObj},
		NumberOfElements: len(transfers),
		Pageable: Pageable{
			Offset:     offset,
			Sort:       []Sort{sortObj},
			Paged:      true,
			PageNumber: page,
			PageSize:   size,
			Unpaged:    false,
		},
		Empty: len(transfers) == 0,
	}

	c.JSON(http.StatusOK, gin.H{"result": response, "success": true})
}
