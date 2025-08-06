package controllers

import (
	"kliro/models"
	"kliro/utils"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type CardController struct{}

func NewCardController() *CardController {
	return &CardController{}
}

// CardResponseByPagination структура ответа с пагинацией для карт
type CardResponseByPagination struct {
	TotalPages       int           `json:"totalPages"`
	TotalElements    int64         `json:"totalElements"`
	First            bool          `json:"first"`
	Last             bool          `json:"last"`
	Size             int           `json:"size"`
	Content          []models.Card `json:"content"`
	Number           int           `json:"number"`
	Sort             []Sort        `json:"sort"`
	NumberOfElements int           `json:"numberOfElements"`
	Pageable         Pageable      `json:"pageable"`
	Empty            bool          `json:"empty"`
}

// GetNewCards godoc
func (cc *CardController) GetNewCards(c *gin.Context) {
	cc.getCardsWithPagination(c, "new_card")
}



// getCardsWithPagination общая функция для получения карт с пагинацией и фильтрацией
func (cc *CardController) getCardsWithPagination(c *gin.Context, tableName string) {
	db := utils.GetDB()

	// Параметры пагинации
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	sortBy := c.DefaultQuery("sort", "bank_name")
	sortDir := c.DefaultQuery("direction", "asc")

	// Параметры фильтрации
	currency := c.Query("currency")
	system := c.Query("system")

	// Валидация параметров
	if page < 0 {
		page = 0
	}
	if size < 1 || size > 100 {
		size = 10
	}

	// Подсчет общего количества записей с фильтрацией
	var totalElements int64
	query := db.Table(tableName)

	// Применение фильтров
	if currency != "" {
		query = query.Where("currency ILIKE ?", "%"+currency+"%")
	}
	if system != "" {
		query = query.Where("system ILIKE ?", "%"+system+"%")
	}

	query.Count(&totalElements)

	// Вычисление пагинации
	totalPages := int((totalElements + int64(size) - 1) / int64(size))
	// Проверяем, есть ли данные на последней странице
	if totalPages > 0 {
		lastPageOffset := (totalPages - 1) * size
		var lastPageCount int64
		lastPageQuery := db.Table(tableName)
		// Применяем те же фильтры для проверки последней страницы
		if currency != "" {
			lastPageQuery = lastPageQuery.Where("currency ILIKE ?", "%"+currency+"%")
		}
		if system != "" {
			lastPageQuery = lastPageQuery.Where("system ILIKE ?", "%"+system+"%")
		}
		lastPageQuery.Offset(lastPageOffset).Limit(size).Count(&lastPageCount)
		if lastPageCount == 0 {
			totalPages = totalPages - 1
		}
	}
	offset := page * size

	// Проверка на пустой результат
	if totalElements == 0 {
		response := CardResponseByPagination{
			TotalPages:       0,
			TotalElements:    0,
			First:            true,
			Last:             true,
			Size:             size,
			Content:          []models.Card{},
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

	// Валидация поля сортировки
	allowedSortFields := map[string]string{
		"bank_name":    "bank_name",
		"title":        "title",
		"currency":     "currency",
		"system":       "system",
		"opening_type": "opening_type",
		"created_at":   "created_at",
	}

	sortField, exists := allowedSortFields[sortBy]
	if !exists {
		sortField = "bank_name"
	}

	// Выполнение запроса с пагинацией, сортировкой и фильтрацией
	var cards []models.Card
	query = db.Table(tableName)

	// Применение фильтров
	if currency != "" {
		query = query.Where("currency ILIKE ?", "%"+currency+"%")
	}
	if system != "" {
		query = query.Where("system ILIKE ?", "%"+system+"%")
	}

	// Применение сортировки
	if sortField == "bank_name" || sortField == "title" || sortField == "currency" || sortField == "system" || sortField == "opening_type" {
		// Для текстовых полей добавляем COLLATE для правильной сортировки
		if sortDirection == "ASC" {
			query = query.Order(sortField + " COLLATE \"C\" ASC")
		} else {
			query = query.Order(sortField + " COLLATE \"C\" DESC")
		}
	} else {
		query = query.Order(sortField + " " + sortDirection)
	}

	// Применение пагинации
	query = query.Offset(offset).Limit(size)

	if err := query.Find(&cards).Error; err != nil {
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
	response := CardResponseByPagination{
		TotalPages:       totalPages,
		TotalElements:    totalElements,
		First:            page == 0,
		Last:             page >= totalPages-1,
		Size:             size,
		Content:          cards,
		Number:           page,
		Sort:             []Sort{sortObj},
		NumberOfElements: len(cards),
		Pageable: Pageable{
			Offset:     offset,
			Sort:       []Sort{sortObj},
			Paged:      true,
			PageNumber: page,
			PageSize:   size,
			Unpaged:    false,
		},
		Empty: len(cards) == 0,
	}

	c.JSON(http.StatusOK, gin.H{"result": response, "success": true})
}
