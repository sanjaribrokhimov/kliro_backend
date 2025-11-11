package bank

import (
	"kliro/models"
	"kliro/utils"
	"net/http"
	"sort"
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

	if page < 0 {
		page = 0
	}
	if size < 1 || size > 100 {
		size = 10
	}

	// Фильтры (строки)
	bank := c.Query("bank")
	search := c.Query("search")
	rateFilter := c.Query("rate")
	termFilter := c.Query("term")
	amountFilter := c.Query("amount")
	opening := strings.ToLower(strings.TrimSpace(c.DefaultQuery("opening", ""))) // bank|online|all

	// Числовые фильтры "от"
	rateFrom := utils.ParseFloatSafe(c.DefaultQuery("rate_from", ""))
	termFrom := utils.ParseIntSafe(c.DefaultQuery("term_months_from", ""))
	amountFrom := utils.ParseInt64Safe(c.DefaultQuery("amount_from", ""))
	useRateFrom := c.DefaultQuery("rate_from", "") != ""
	useTermFrom := c.DefaultQuery("term_months_from", "") != ""
	useAmountFrom := c.DefaultQuery("amount_from", "") != ""

	// Маппинг camelCase банк -> точное название
	bankCamelMap := map[string]string{
		"agroBank":             "Agro Bank",
		"aloqaBank":            "Aloqa Bank",
		"anorBank":             "Anor Bank",
		"asakaBank":            "Asaka Bank",
		"asiaAllianceBank":     "Asia Alliance Bank",
		"brb":                  "BRB",
		"davrBank":             "Davr Bank",
		"garantBank":           "Garant Bank",
		"hamkorBank":           "Hamkor Bank",
		"hayotBank":            "Hayot Bank",
		"infinBank":            "Infin Bank",
		"ipakYoliBank":         "Ipak Yo'li Banki",
		"ipotekaBank":          "Ipoteka Bank",
		"kapitalBank":          "Kapital Bank",
		"mkBank":               "MK Bank",
		"octoBank":             "Octo Bank",
		"orientFinansBank":     "Orient Finans Bank",
		"ozbekistonMilliyBank": "O‘zbekiston Milliy Banki",
		"ozsanoatqurilishBank": "O‘zsanoatqurilish Bank",
		"poytaxtBank":          "Poytaxt Bank",
		"smartBank":            "Smart Bank",
		"tengeBank":            "Tenge Bank",
		"trastBank":            "Trast Bank",
		"turonBank":            "Turon Bank",
		"universalBank":        "Universal Bank",
		"xalqBank":             "Xalq Banki",
	}
	bankFilter := strings.TrimSpace(bank)
	if bankFilter != "" {
		if v, ok := bankCamelMap[bankFilter]; ok {
			bankFilter = v
		}
	}

	// Базовый запрос по строковым фильтрам и каналу
	baseQ := db.Table(tableName)
	if search != "" {
		baseQ = baseQ.Where("bank_name ILIKE ?", "%"+search+"%")
	} else if bankFilter != "" {
		baseQ = baseQ.Where("bank_name ILIKE ?", "%"+bankFilter+"%")
	}
	if rateFilter != "" {
		baseQ = baseQ.Where("rate ILIKE ?", "%"+rateFilter+"%")
	}
	if termFilter != "" {
		baseQ = baseQ.Where("term ILIKE ?", "%"+termFilter+"%")
	}
	if amountFilter != "" {
		baseQ = baseQ.Where("amount ILIKE ?", "%"+amountFilter+"%")
	}
	if opening == "bank" {
		baseQ = baseQ.Where("channel ILIKE '%Bank%'").Where("channel NOT ILIKE '%Onlayn%'")
	} else if opening == "online" || opening == "onlayn" {
		baseQ = baseQ.Where("channel ILIKE '%Onlayn%'").Where("channel NOT ILIKE '%Bank%'")
	} else if opening == "all" {
		baseQ = baseQ.Where("channel ILIKE '%Bank%'").Where("channel ILIKE '%Onlayn%'")
	}

	// Грузим кандидатов, применяем числовые фильтры в памяти
	var all []models.Autocredit
	if err := baseQ.Find(&all).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
		return
	}

	filtered := make([]models.Autocredit, 0, len(all))
	for _, a := range all {
		if useRateFrom {
			minRate := utils.ExtractFirstFloat(a.Rate)
			if minRate < rateFrom {
				continue
			}
		}
		if useTermFrom {
			minMonths := utils.ExtractMinMonths(a.Term)
			if minMonths < termFrom {
				continue
			}
		}
		if useAmountFrom {
			maxAmt := utils.ExtractMaxAmount(a.Amount)
			if maxAmt < amountFrom {
				continue
			}
		}
		filtered = append(filtered, a)
	}

	// Сортировка ДО пагинации: приоритет у rate_from, если нет - то по bank_name
	if useRateFrom {
		// Автоматическая сортировка по ставкам от меньшего к большему
		sort.SliceStable(filtered, func(i, j int) bool {
			rateI := utils.ExtractFirstFloat(filtered[i].Rate)
			rateJ := utils.ExtractFirstFloat(filtered[j].Rate)
			return rateI < rateJ
		})
	} else if strings.EqualFold(sortBy, "bank_name") {
		// Сортировка по банку только если нет фильтра по ставкам
		sort.SliceStable(filtered, func(i, j int) bool {
			if strings.ToLower(sortDir) == "desc" {
				return filtered[i].BankName > filtered[j].BankName
			}
			return filtered[i].BankName < filtered[j].BankName
		})
	}

	// Итоги и пагинация
	totalElements := int64(len(filtered))
	totalPages := int((totalElements + int64(size) - 1) / int64(size))
	if totalPages > 0 {
		lastPageOffset := (totalPages - 1) * size
		if lastPageOffset >= len(filtered) {
			totalPages = totalPages - 1
		}
	}
	offset := page * size
	end := offset + size
	if offset > len(filtered) {
		offset = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	pageItems := filtered[offset:end]

	sortObj := Sort{Direction: strings.ToUpper(sortDir), NullHandling: "NATIVE", Ascending: strings.ToLower(sortDir) == "asc", Property: sortBy, IgnoreCase: false}
	response := AutocreditResponseByPagination{
		TotalPages:       totalPages,
		TotalElements:    totalElements,
		First:            page == 0,
		Last:             page >= totalPages-1,
		Size:             size,
		Content:          pageItems,
		Number:           page,
		Sort:             []Sort{sortObj},
		NumberOfElements: len(pageItems),
		Pageable:         Pageable{Offset: offset, Sort: []Sort{sortObj}, Paged: true, PageNumber: page, PageSize: size, Unpaged: false},
		Empty:            len(pageItems) == 0,
	}

	c.JSON(http.StatusOK, gin.H{"result": response, "success": true})
}
