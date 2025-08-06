package controllers

import (
	"kliro/models"
	"kliro/utils"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type AutocreditController struct{}

func NewAutocreditController() *AutocreditController {
	return &AutocreditController{}
}

// AutocreditResponseByPagination структура ответа с пагинацией для автокредитов
type AutocreditResponseByPagination struct {
	TotalPages       int                 `json:"totalPages"`
	TotalElements    int64               `json:"totalElements"`
	First            bool                `json:"first"`
	Last             bool                `json:"last"`
	Size             int                 `json:"size"`
	Content          []models.Autocredit `json:"content"`
	Number           int                 `json:"number"`
	Sort             []Sort              `json:"sort"`
	NumberOfElements int                 `json:"numberOfElements"`
	Pageable         Pageable            `json:"pageable"`
	Empty            bool                `json:"empty"`
}

// GetNewAutocredits получает новые автокредиты с пагинацией
func (ac *AutocreditController) GetNewAutocredits(c *gin.Context) {
	ac.getAutocreditsWithPagination(c, "new_autocredit")
}



// getAutocreditsWithPagination общая функция для получения автокредитов с пагинацией
func (ac *AutocreditController) getAutocreditsWithPagination(c *gin.Context, tableName string) {
	db := utils.GetDB()

	// Параметры пагинации
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	sortBy := c.DefaultQuery("sort", "bank_name")
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
		response := AutocreditResponseByPagination{
			TotalPages:       0,
			TotalElements:    0,
			First:            true,
			Last:             true,
			Size:             size,
			Content:          []models.Autocredit{},
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

	// Валидация поля сортировки для автокредитов
	allowedSortFields := map[string]string{
		"bank_name":   "bank_name",
		"description": "description",
		"rate":        "rate",
		"term":        "term",
		"amount":      "amount",
		"channel":     "channel",
		"created_at":  "created_at",
	}

	sortField, exists := allowedSortFields[sortBy]
	if !exists {
		sortField = "bank_name"
	}

	// Выполнение запроса с пагинацией и сортировкой
	var credits []models.Autocredit
	query := db.Table(tableName)

	// Применение сортировки
	if sortField == "bank_name" {
		// Для текстовых полей добавляем COLLATE для правильной сортировки
		if sortDirection == "ASC" {
			query = query.Order("bank_name COLLATE \"C\" ASC")
		} else {
			query = query.Order("bank_name COLLATE \"C\" DESC")
		}
	} else {
		query = query.Order(sortField + " " + sortDirection)
	}

	// Применение пагинации
	query = query.Offset(offset).Limit(size)

	if err := query.Find(&credits).Error; err != nil {
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
	response := AutocreditResponseByPagination{
		TotalPages:       totalPages,
		TotalElements:    totalElements,
		First:            page == 0,
		Last:             page >= totalPages-1,
		Size:             size,
		Content:          credits,
		Number:           page,
		Sort:             []Sort{sortObj},
		NumberOfElements: len(credits),
		Pageable: Pageable{
			Offset:     offset,
			Sort:       []Sort{sortObj},
			Paged:      true,
			PageNumber: page,
			PageSize:   size,
			Unpaged:    false,
		},
		Empty: len(credits) == 0,
	}

	c.JSON(http.StatusOK, gin.H{"result": response, "success": true})
}
