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
	"gorm.io/gorm"
)

type MortgageController struct {
	db *gorm.DB
}

func NewMortgageController(db *gorm.DB) *MortgageController {
	return &MortgageController{db: db}
}

// GetNewMortgages получает новые ипотечные кредиты с пагинацией
func (mc *MortgageController) GetNewMortgages(c *gin.Context) {
	getMortgagesWithPagination(c, mc.db, "new_mortgage")
}

// ParseMortgage парсит ипотечные кредиты с указанного URL
func (mc *MortgageController) ParseMortgage(c *gin.Context) {
	url := c.Query("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"result":  nil,
			"success": false,
			"error":   "url parameter is required",
		})
		return
	}

	parser := bankServices.NewMortgageParser()
	mortgages, err := parser.ParseURL(url)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"result":  nil,
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if len(mortgages) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"result":  nil,
			"success": true,
			"error":   "No mortgages found",
		})
		return
	}

	// Возвращаем первый найденный ипотечный кредит
	c.JSON(http.StatusOK, gin.H{
		"result":  mortgages[0],
		"success": true,
		"error":   nil,
	})
}

func getMortgagesWithPagination(c *gin.Context, db *gorm.DB, tableName string) {
	// Параметры пагинации (1-based)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	sortBy := c.DefaultQuery("sortBy", "created_at")
	sortOrder := c.DefaultQuery("sortOrder", "desc")
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	// Фильтры
	bank := c.Query("bank")
	search := c.Query("search")
	opening := strings.ToLower(strings.TrimSpace(c.DefaultQuery("opening", ""))) // bank|online|all
	rateFromStr := c.DefaultQuery("rate_from", "")
	termFromStr := c.DefaultQuery("term_months_from", "")
	amountFromStr := c.DefaultQuery("amount_from", "")

	// Маппинг банков (camelCase -> точное имя)
	bankCamelMap := map[string]string{
		"agroBank":             "Agro Bank",
		"aloqaBank":            "Aloqa Bank",
		"asakaBank":            "Asaka Bank",
		"brb":                  "BRB",
		"davrBank":             "Davr Bank",
		"hamkorBank":           "Hamkor Bank",
		"infinBank":            "Infin Bank",
		"ipakYoliBank":         "Ipak Yo'li Banki",
		"ipotekaBank":          "Ipoteka Bank",
		"orientFinansBank":     "Orient Finans Bank",
		"ozbekistonMilliyBank": "O‘zbekiston Milliy Banki",
		"ozsanoatqurilishBank": "O‘zsanoatqurilish Bank",
		"tbcBank":              "TBC Bank",
		"turonBank":            "Turon Bank",
		"ziraatBank":           "Ziraat Bank",
		"aloqaBanki":           "Aloqa Bank",
		"xalqBank":             "Xalq Banki",
	}
	bankFilter := strings.TrimSpace(bank)
	if bankFilter != "" {
		if v, ok := bankCamelMap[bankFilter]; ok {
			bankFilter = v
		}
	}

	// Базовый запрос по строковым фильтрам
	baseQ := db.Table(tableName)
	if search != "" {
		baseQ = baseQ.Where("bank_name ILIKE ?", "%"+search+"%")
	} else if bankFilter != "" {
		baseQ = baseQ.Where("bank_name ILIKE ?", "%"+bankFilter+"%")
	}
	if opening == "bank" {
		baseQ = baseQ.Where("channel ILIKE '%Bank%'").Where("channel NOT ILIKE '%Onlayn%'")
	} else if opening == "online" || opening == "onlayn" {
		baseQ = baseQ.Where("channel ILIKE '%Onlayn%'").Where("channel NOT ILIKE '%Bank%'")
	} else if opening == "all" {
		baseQ = baseQ.Where("channel ILIKE '%Bank%'").Where("channel ILIKE '%Onlayn%'")
	}

	// Загружаем кандидатов
	var all []models.Mortgage
	if err := baseQ.Find(&all).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"result": nil, "success": false, "error": "DB error"})
		return
	}

	// Числовые фильтры в памяти
	rateFrom := utils.ParseFloatSafe(rateFromStr)
	termFrom := utils.ParseIntSafe(termFromStr)
	amountFrom := utils.ParseInt64Safe(amountFromStr)

	filtered := make([]models.Mortgage, 0, len(all))
	for _, m := range all {
		if rateFromStr != "" {
			minRate := utils.ExtractFirstFloat(m.Rate)
			if minRate < rateFrom {
				continue
			}
		}
		if termFromStr != "" {
			minMonths := utils.ExtractMinMonths(m.Term)
			if minMonths < termFrom {
				continue
			}
		}
		if amountFromStr != "" {
			maxAmt := utils.ExtractMaxAmount(m.Amount)
			if maxAmt < amountFrom {
				continue
			}
		}
		filtered = append(filtered, m)
	}

	// Сортировка ДО пагинации: приоритет у rate_from, если нет - то по bank_name
	if rateFromStr != "" {
		// Автоматическая сортировка по ставкам от меньшего к большему
		sort.SliceStable(filtered, func(i, j int) bool {
			rateI := utils.ExtractFirstFloat(filtered[i].Rate)
			rateJ := utils.ExtractFirstFloat(filtered[j].Rate)
			return rateI < rateJ
		})
	} else if strings.EqualFold(sortBy, "bank_name") {
		// Сортировка по банку только если нет фильтра по ставкам
		sort.SliceStable(filtered, func(i, j int) bool {
			if strings.ToLower(sortOrder) == "desc" {
				return filtered[i].BankName > filtered[j].BankName
			}
			return filtered[i].BankName < filtered[j].BankName
		})
	}

	// Пагинация в памяти
	offset := (page - 1) * limit
	end := offset + limit
	if offset > len(filtered) {
		offset = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	pageItems := filtered[offset:end]

	// Подсчитываем total и totalPages
	total := int64(len(filtered))
	totalPages := (total + int64(limit) - 1) / int64(limit)
	if totalPages > 0 {
		lastPageOffset := int((totalPages - 1) * int64(limit))
		if lastPageOffset >= len(filtered) {
			totalPages = totalPages - 1
		}
	}

	response := gin.H{
		"result": gin.H{
			"totalPages":       totalPages,
			"totalElements":    total,
			"first":            page == 1,
			"last":             page >= int(totalPages),
			"size":             limit,
			"content":          pageItems,
			"number":           page - 1, // 0-based
			"numberOfElements": len(pageItems),
			"empty":            len(pageItems) == 0,
		},
		"success": true,
	}

	c.JSON(http.StatusOK, response)
}
