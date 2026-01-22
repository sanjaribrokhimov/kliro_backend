package bank

import (
	"kliro/models"
	"kliro/utils"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type DepositController struct{}

func NewDepositController() *DepositController {
	return &DepositController{}
}

// DepositResponseByPagination структура ответа с пагинацией для вкладов
type DepositResponseByPagination struct {
	TotalPages       int              `json:"totalPages"`
	TotalElements    int64            `json:"totalElements"`
	First            bool             `json:"first"`
	Last             bool             `json:"last"`
	Size             int              `json:"size"`
	Content          []models.Deposit `json:"content"`
	Number           int              `json:"number"`
	Sort             []Sort           `json:"sort"`
	NumberOfElements int              `json:"numberOfElements"`
	Pageable         Pageable         `json:"pageable"`
	Empty            bool             `json:"empty"`
}

// TranslatedDepositResponseByPagination структура ответа с переводами (как у microcredit)
type TranslatedDepositResponseByPagination struct {
	TotalPages       int                    `json:"totalPages"`
	TotalElements    int64                  `json:"totalElements"`
	First            bool                   `json:"first"`
	Last             bool                   `json:"last"`
	Size             int                    `json:"size"`
	Content          []utils.TranslatedDeposit `json:"content"`
	Number           int                    `json:"number"`
	Sort             []Sort                 `json:"sort"`
	NumberOfElements int                    `json:"numberOfElements"`
	Pageable         Pageable               `json:"pageable"`
	Empty            bool                   `json:"empty"`
}

// GetNewDeposits godoc
func (dc *DepositController) GetNewDeposits(c *gin.Context) {
	dc.getDepositsWithPagination(c, "new_deposit")
}

// getDepositsWithPagination общая функция для получения вкладов с пагинацией
func (dc *DepositController) getDepositsWithPagination(c *gin.Context, tableName string) {
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

	// Фильтры
	bank := c.Query("bank")
	search := c.Query("search")
	currency := strings.ToLower(strings.TrimSpace(c.DefaultQuery("currency", ""))) // usd|eur|rub|uzs|sum
	rateFromStr := c.DefaultQuery("rate_from", "")
	termFromStr := c.DefaultQuery("term_months_from", "")
	amountFromStr := c.DefaultQuery("amount_from", "") // в валюте строки (so'm/usd/eur)

	// Маппинг банков (camelCase -> точное имя)
	bankCamelMap := map[string]string{
		"agroBank":             "Agro Bank",
		"aloqaBank":            "Aloqa Bank",
		"anorBank":             "Anor Bank",
		"asakaBank":            "Asaka Bank",
		"asiaAllianceBank":     "Asia Alliance Bank",
		"avoBank":              "AVO Bank",
		"davrBank":             "Davr Bank",
		"garantBank":           "Garant Bank",
		"hamkorBank":           "Hamkor Bank",
		"infinBank":            "Infin Bank",
		"ipakYoliBank":         "Ipak Yo'li Banki",
		"kapitalBank":          "Kapital Bank",
		"mkBank":               "MK Bank",
		"orientFinansBank":     "Orient Finans Bank",
		"ozbekistonMilliyBank": "O‘zbekiston Milliy Banki",
		"ozsanoatqurilishBank": "O‘zsanoatqurilish Bank",
		"tbcBank":              "TBC Bank",
		"tengeBank":            "Tenge Bank",
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

	// Синонимы валют
	currencySyn := map[string][]string{
		"usd": {"aqsh dollar", "usd", "dollar"},
		"eur": {"yevro", "euro"},
		"rub": {"rubl", "rub"},
		"uzs": {"so'm", "som", "sum", "uzs"},
		"sum": {"so'm", "som", "sum", "uzs"},
	}

	// Базовый SQL фильтр по банку и валюте (по строке MinAmount)
	baseQ := db.Table(tableName)
	if search != "" {
		baseQ = baseQ.Where("bank_name ILIKE ?", "%"+search+"%")
	} else if bankFilter != "" {
		baseQ = baseQ.Where("bank_name ILIKE ?", "%"+bankFilter+"%")
	}
	if currency != "" {
		if words, ok := currencySyn[currency]; ok {
			or := baseQ
			first := true
			for _, w := range words {
				cond := "min_amount ILIKE ?"
				if first {
					baseQ = baseQ.Where(cond, "%"+w+"%")
					first = false
				} else {
					or = or.Or(cond, "%"+w+"%")
				}
			}
			baseQ = or
		}
	}

	// Загружаем кандидатов
	var all []models.Deposit
	if err := baseQ.Find(&all).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
		return
	}

	// Числовые фильтры в памяти
	rateFrom := utils.ParseFloatSafe(rateFromStr)
	termFrom := utils.ParseIntSafe(termFromStr)
	amountFrom := utils.ParseInt64Safe(amountFromStr)

	filtered := make([]models.Deposit, 0, len(all))
	for _, d := range all {
		if rateFromStr != "" {
			minRate := utils.ExtractFirstFloat(d.Rate)
			if minRate > rateFrom {
				continue
			}
		}
		if termFromStr != "" {
			minMonths := utils.ExtractMinMonths(d.TermYears)
			if minMonths < termFrom {
				continue
			}
		}
		if amountFromStr != "" && currency != "" {
			// Применяем amount_from ТОЛЬКО если валюта запроса совпадает с валютой продукта
			amtCur := utils.DetectCurrencyFromAmount(d.MinAmount)
			if amtCur == currency {
				minAmt := utils.ExtractMinAmount(d.MinAmount)
				if amountFrom < minAmt {
					continue
				}
			}
		}
		filtered = append(filtered, d)
	}

	// Сортировка ДО пагинации: приоритет у rate_from, если нет - то по bank_name
	if rateFromStr != "" {
		// Автоматическая сортировка по ставкам от меньшего к большему
		sort.SliceStable(filtered, func(i, j int) bool {
			rateI := utils.ExtractFirstFloat(filtered[i].Rate)
			rateJ := utils.ExtractFirstFloat(filtered[j].Rate)
			return rateI > rateJ
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

	// Применяем переводы к каждому элементу (uz/ru/en/oz)
	translator := utils.GetDepositTranslator()
	translatedContent := make([]utils.TranslatedDeposit, 0, len(pageItems))
	for _, item := range pageItems {
		translated := translator.TranslateDeposit(
			item.BankName,
			item.Title,
			item.Rate,
			item.TermYears,
			item.MinAmount,
		)
		translated.ID = item.ID
		translated.CreatedAt = item.CreatedAt.Format("2006-01-02T15:04:05.000000Z")
		translatedContent = append(translatedContent, translated)
	}

	sortObj := Sort{Direction: strings.ToUpper(sortDir), NullHandling: "NATIVE", Ascending: strings.ToLower(sortDir) == "asc", Property: sortBy, IgnoreCase: false}
	response := TranslatedDepositResponseByPagination{
		TotalPages:       int((int64(len(filtered)) + int64(size) - 1) / int64(size)),
		TotalElements:    int64(len(filtered)),
		First:            page == 0,
		Last:             (page+1)*size >= len(filtered) && len(filtered) > 0,
		Size:             size,
		Content:          translatedContent,
		Number:           page,
		Sort:             []Sort{sortObj},
		NumberOfElements: len(translatedContent),
		Pageable:         Pageable{Offset: offset, Sort: []Sort{sortObj}, Paged: true, PageNumber: page, PageSize: size, Unpaged: false},
		Empty:            len(translatedContent) == 0,
	}

	c.JSON(http.StatusOK, gin.H{"result": response, "success": true})
}

var _ = regexp.MustCompile // keep import if not used elsewhere
