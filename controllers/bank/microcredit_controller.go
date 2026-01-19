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

// TranslatedResponseByPagination структура ответа с переводами
type TranslatedResponseByPagination struct {
	TotalPages       int                           `json:"totalPages"`
	TotalElements    int64                         `json:"totalElements"`
	First            bool                          `json:"first"`
	Last             bool                          `json:"last"`
	Size             int                           `json:"size"`
	Content          []utils.TranslatedMicrocredit `json:"content"`
	Number           int                           `json:"number"`
	Sort             []Sort                        `json:"sort"`
	NumberOfElements int                           `json:"numberOfElements"`
	Pageable         Pageable                      `json:"pageable"`
	Empty            bool                          `json:"empty"`
}

// GetNewMicrocredits godoc
func (mc *MicrocreditController) GetNewMicrocredits(c *gin.Context) {
	mc.getMicrocreditsWithPagination(c, "new_microcredit")
}

// getMicrocreditsWithPagination общая функция для получения микрофинансов с пагинацией
func (mc *MicrocreditController) getMicrocreditsWithPagination(c *gin.Context, tableName string) {
	db := utils.GetDB()

	// Пагинация и сортировка
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

	// Фильтры (строковые)
	bank := c.Query("bank")
	search := c.Query("search")
	rateFilter := c.Query("rate")
	termFilter := c.Query("term")
	amountFilter := c.Query("amount")
	opening := strings.ToLower(strings.TrimSpace(c.DefaultQuery("opening", "")))

	// Числовые фильтры "от"
	rateFromStr := c.DefaultQuery("rate_from", "")        // проценты, 24.5
	termFromStr := c.DefaultQuery("term_months_from", "") // месяцы, например 60
	amountFromStr := c.DefaultQuery("amount_from", "")    // сумма в so'm, например 100000000
	rateFrom := utils.ParseFloatSafe(rateFromStr)
	termFrom := utils.ParseIntSafe(termFromStr)
	amountFrom := utils.ParseInt64Safe(amountFromStr)

	// Маппинг camelCase названий банков -> точное имя
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

	// Базовый запрос (строковые фильтры + opening)
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

	// Загружаем кандидатов (без пагинации), затем применяем числовые фильтры в памяти
	var all []models.Microcredit
	if err := baseQ.Find(&all).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
		return
	}

	filtered := make([]models.Microcredit, 0, len(all))
	for _, m := range all {
		// rate_from: сравниваем с минимальным значением в строке ставки
		if rateFromStr != "" {
			minRate := utils.ExtractFirstFloat(m.Rate)
			if minRate < rateFrom {
				continue
			}
		}
		// term_months_from: сравниваем с минимальным сроком в месяцах
		if termFromStr != "" {
			minMonths := utils.ExtractMinMonths(m.Term)
			if minMonths < termFrom {
				continue
			}
		}
		// amount_from: сравниваем с максимальной суммой, найденной в строке
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
			if strings.ToLower(sortDir) == "desc" {
				return filtered[i].BankName > filtered[j].BankName
			}
			return filtered[i].BankName < filtered[j].BankName
		})
	}
	// Иные sort поля оставляем как есть (при необходимости можно расширить)

	// total после числовых фильтров
	totalElements := int64(len(filtered))
	totalPages := int((totalElements + int64(size) - 1) / int64(size))
	if totalPages > 0 {
		lastPageOffset := (totalPages - 1) * size
		if lastPageOffset >= len(filtered) {
			totalPages = totalPages - 1
		}
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

	// Применяем переводы к каждому элементу
	translator := utils.GetMicrocreditTranslator()
	translatedContent := make([]utils.TranslatedMicrocredit, 0, len(pageItems))
	
	for _, item := range pageItems {
		translated := translator.TranslateMicrocredit(
			item.BankName,
			item.Description,
			item.Rate,
			item.Term,
			item.Amount,
			item.Channel,
		)
		// Заполняем остальные поля
		translated.ID = item.ID
		translated.URL = item.URL
		translated.CreatedAt = item.CreatedAt.Format("2006-01-02T15:04:05.000000Z")
		translatedContent = append(translatedContent, translated)
	}

	sortObj := Sort{Direction: strings.ToUpper(sortDir), NullHandling: "NATIVE", Ascending: strings.ToLower(sortDir) == "asc", Property: sortBy, IgnoreCase: false}
	response := TranslatedResponseByPagination{
		TotalPages:       totalPages,
		TotalElements:    totalElements,
		First:            page == 0,
		Last:             page >= totalPages-1,
		Size:             size,
		Content:          translatedContent,
		Number:           page,
		Sort:             []Sort{sortObj},
		NumberOfElements: len(pageItems),
		Pageable:         Pageable{Offset: offset, Sort: []Sort{sortObj}, Paged: true, PageNumber: page, PageSize: size, Unpaged: false},
		Empty:            len(pageItems) == 0,
	}

	c.JSON(http.StatusOK, gin.H{"result": response, "success": true})
}
