package controllers

import (
	"kliro/models"
	"kliro/utils"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type MicrocreditController struct{}

func NewMicrocreditController() *MicrocreditController {
	return &MicrocreditController{}
}

// Sort структура для сортировки
type Sort struct {
	Direction    string `json:"direction"`
	NullHandling string `json:"nullHandling"`
	Ascending    bool   `json:"ascending"`
	Property     string `json:"property"`
	IgnoreCase   bool   `json:"ignoreCase"`
}

// Pageable структура для пагинации
type Pageable struct {
	Offset     int    `json:"offset"`
	Sort       []Sort `json:"sort"`
	Paged      bool   `json:"paged"`
	PageNumber int    `json:"pageNumber"`
	PageSize   int    `json:"pageSize"`
	Unpaged    bool   `json:"unpaged"`
}

// ResponseByPagination структура ответа с пагинацией
type ResponseByPagination struct {
	TotalPages       int                  `json:"totalPages"`
	TotalElements    int64                `json:"totalElements"`
	First            bool                 `json:"first"`
	Last             bool                 `json:"last"`
	Size             int                  `json:"size"`
	Content          []models.Microcredit `json:"content"`
	Number           int                  `json:"number"`
	Sort             []Sort               `json:"sort"`
	NumberOfElements int                  `json:"numberOfElements"`
	Pageable         Pageable             `json:"pageable"`
	Empty            bool                 `json:"empty"`
}

// GetNewMicrocredits godoc
func (mc *MicrocreditController) GetNewMicrocredits(c *gin.Context) {
	mc.getMicrocreditsWithPagination(c, "new_microcredit")
}

// GetOldMicrocredits godoc
func (mc *MicrocreditController) GetOldMicrocredits(c *gin.Context) {
	mc.getMicrocreditsWithPagination(c, "old_microcredit")
}

// getMicrocreditsWithPagination общая функция для получения микрофинансов с пагинацией
func (mc *MicrocreditController) getMicrocreditsWithPagination(c *gin.Context, tableName string) {
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
	offset := page * size

	// Проверка на пустой результат
	if totalElements == 0 {
		response := ResponseByPagination{
			TotalPages:       0,
			TotalElements:    0,
			First:            true,
			Last:             true,
			Size:             size,
			Content:          []models.Microcredit{},
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
		c.JSON(http.StatusOK, response)
		return
	}

	// Создание сортировки
	sortDirection := "ASC"
	if strings.ToLower(sortDir) == "desc" {
		sortDirection = "DESC"
	}

	// Валидация поля сортировки
	allowedSortFields := map[string]string{
		"bank_name":   "bank_name",
		"max_amount":  "max_amount",
		"rate_max":    "rate_max",
		"rate_min":    "rate_min",
		"term_months": "term_months",
		"created_at":  "created_at",
	}

	sortField, exists := allowedSortFields[sortBy]
	if !exists {
		sortField = "bank_name"
	}

	// Выполнение запроса с пагинацией и сортировкой
	var credits []models.Microcredit
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
	response := ResponseByPagination{
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

	c.JSON(http.StatusOK, response)
}
