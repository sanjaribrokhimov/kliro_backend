package bank

import (
	"kliro/models"
	bankServices "kliro/services/bank"
	"kliro/utils"
	"net/http"
	"sort"
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
	search := c.Query("search")
	commissionFromStr := c.Query("commission_from")

	// Валидация параметров
	if page < 0 {
		page = 0
	}
	if size < 1 || size > 100 {
		size = 10
	}

	// Парсим commission_from
	commissionFrom := utils.ParseFloatSafe(commissionFromStr)

	// Загружаем все данные из БД
	var all []models.Transfer
	baseQuery := db.Table(tableName)
	if search != "" {
		baseQuery = baseQuery.Where("app_name ILIKE ?", "%"+search+"%")
	}

	if err := baseQuery.Find(&all).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
		return
	}

	// Фильтрация по commission_from
	// commission_from показывает записи с комиссией >= указанного значения
	filtered := make([]models.Transfer, 0, len(all))
	for _, t := range all {
		if commissionFromStr != "" {
			commissionValue := utils.ExtractFirstFloat(t.Commission)
			if commissionValue < commissionFrom {
				continue
			}
		}
		filtered = append(filtered, t)
	}

	// Сортировка ДО пагинации
	if commissionFromStr != "" {
		// Автоматическая сортировка по комиссии от большего к меньшему при наличии фильтра
		// чтобы сначала показывались записи с комиссией ближе к указанному значению
		sort.SliceStable(filtered, func(i, j int) bool {
			commI := utils.ExtractFirstFloat(filtered[i].Commission)
			commJ := utils.ExtractFirstFloat(filtered[j].Commission)
			return commI > commJ
		})
	} else if strings.EqualFold(sortBy, "commission") {
		// Сортировка по комиссии если явно указано
		sort.SliceStable(filtered, func(i, j int) bool {
			commI := utils.ExtractFirstFloat(filtered[i].Commission)
			commJ := utils.ExtractFirstFloat(filtered[j].Commission)
			if strings.ToLower(sortDir) == "desc" {
				return commI > commJ
			}
			return commI < commJ
		})
	} else if strings.EqualFold(sortBy, "app_name") {
		// Сортировка по имени приложения
		sort.SliceStable(filtered, func(i, j int) bool {
			if strings.ToLower(sortDir) == "desc" {
				return filtered[i].AppName > filtered[j].AppName
			}
			return filtered[i].AppName < filtered[j].AppName
		})
	} else if strings.EqualFold(sortBy, "created_at") {
		// Сортировка по дате создания
		sort.SliceStable(filtered, func(i, j int) bool {
			if strings.ToLower(sortDir) == "desc" {
				return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
			}
			return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
		})
	}

	// Подсчет общего количества после фильтрации
	totalElements := int64(len(filtered))
	totalPages := int((totalElements + int64(size) - 1) / int64(size))

	// Пагинация в памяти
	offset := page * size
	end := offset + size
	if offset > len(filtered) {
		offset = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	pageItems := filtered[offset:end]

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
		Content:          pageItems,
		Number:           page,
		Sort:             []Sort{sortObj},
		NumberOfElements: len(pageItems),
		Pageable: Pageable{
			Offset:     offset,
			Sort:       []Sort{sortObj},
			Paged:      true,
			PageNumber: page,
			PageSize:   size,
			Unpaged:    false,
		},
		Empty: len(pageItems) == 0,
	}

	c.JSON(http.StatusOK, gin.H{"result": response, "success": true})
}
