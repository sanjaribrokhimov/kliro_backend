package bank

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

// TranslatedCardResponseByPagination структура ответа с переводами (как у deposit)
type TranslatedCardResponseByPagination struct {
	TotalPages       int                  `json:"totalPages"`
	TotalElements    int64                `json:"totalElements"`
	First            bool                 `json:"first"`
	Last             bool                 `json:"last"`
	Size             int                 `json:"size"`
	Content          []utils.TranslatedCard `json:"content"`
	Number           int                 `json:"number"`
	Sort             []Sort              `json:"sort"`
	NumberOfElements int                 `json:"numberOfElements"`
	Pageable         Pageable            `json:"pageable"`
	Empty            bool                `json:"empty"`
}

// TranslatedCreditCardResponseByPagination структура ответа с переводами для кредитных карт
type TranslatedCreditCardResponseByPagination struct {
	TotalPages       int                        `json:"totalPages"`
	TotalElements    int64                      `json:"totalElements"`
	First            bool                       `json:"first"`
	Last             bool                       `json:"last"`
	Size             int                        `json:"size"`
	Content          []utils.TranslatedCreditCard `json:"content"`
	Number           int                        `json:"number"`
	Sort             []Sort                     `json:"sort"`
	NumberOfElements int                        `json:"numberOfElements"`
	Pageable         Pageable                   `json:"pageable"`
	Empty            bool                       `json:"empty"`
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
	bank := c.Query("bank")
	search := c.Query("search")
	opening := strings.ToLower(strings.TrimSpace(c.DefaultQuery("opening", "all"))) // bank|online|all

	// Нормализация bank camelCase -> точное название (если передан ключ)
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

	// Подготовка синонимов валюты к данным в БД
	currencySynonyms := []string{}
	if currency != "" {
		lc := strings.ToLower(strings.TrimSpace(currency))
		switch lc {
		case "usd", "dollar", "dollar_usd":
			currencySynonyms = []string{"AQSH dollari", "USD", "Dollar"}
		case "eur", "euro":
			currencySynonyms = []string{"Yevro", "EUR", "Euro"}
		case "sum", "uzs", "so'm", "som", "soum":
			currencySynonyms = []string{"So'm", "UZS", "Sum"}
		default:
			currencySynonyms = []string{currency}
		}
	}

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
	if len(currencySynonyms) > 0 {
		for i, syn := range currencySynonyms {
			pattern := "%" + syn + "%"
			if i == 0 {
				query = query.Where("currency ILIKE ?", pattern)
			} else {
				query = query.Or("currency ILIKE ?", pattern)
			}
		}
	}
	if system != "" {
		query = query.Where("system ILIKE ?", "%"+system+"%")
	}
	if search != "" {
		query = query.Where("bank_name ILIKE ?", "%"+search+"%")
	} else if bankFilter != "" {
		query = query.Where("bank_name ILIKE ?", "%"+bankFilter+"%")
	}
	if opening == "bank" {
		query = query.Where("opening_type ILIKE '%Bank%'").Where("opening_type NOT ILIKE '%Onlayn%'")
	} else if opening == "online" || opening == "onlayn" {
		query = query.Where("opening_type ILIKE '%Onlayn%'").Where("opening_type NOT ILIKE '%Bank%'")
	} else if opening == "all" {
		query = query.Where("opening_type ILIKE '%Bank%'").Where("opening_type ILIKE '%Onlayn%'")
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
		if len(currencySynonyms) > 0 {
			for i, syn := range currencySynonyms {
				pattern := "%" + syn + "%"
				if i == 0 {
					lastPageQuery = lastPageQuery.Where("currency ILIKE ?", pattern)
				} else {
					lastPageQuery = lastPageQuery.Or("currency ILIKE ?", pattern)
				}
			}
		}
		if system != "" {
			lastPageQuery = lastPageQuery.Where("system ILIKE ?", "%"+system+"%")
		}
		if bankFilter != "" {
			lastPageQuery = lastPageQuery.Where("bank_name ILIKE ?", "%"+bankFilter+"%")
		}
		if opening == "bank" {
			lastPageQuery = lastPageQuery.Where("opening_type ILIKE '%Bank%'").Where("opening_type NOT ILIKE '%Onlayn%'")
		} else if opening == "online" || opening == "onlayn" {
			lastPageQuery = lastPageQuery.Where("opening_type ILIKE '%Onlayn%'").Where("opening_type NOT ILIKE '%Bank%'")
		} else if opening == "all" {
			lastPageQuery = lastPageQuery.Where("opening_type ILIKE '%Bank%'").Where("opening_type ILIKE '%Onlayn%'")
		}
		lastPageQuery.Offset(lastPageOffset).Limit(size).Count(&lastPageCount)
		if lastPageCount == 0 {
			totalPages = totalPages - 1
		}
	}
	offset := page * size

	// Проверка на пустой результат
	if totalElements == 0 {
		sortObj := Sort{
			Direction:    strings.ToUpper(sortDir),
			NullHandling: "NATIVE",
			Ascending:    strings.ToLower(sortDir) == "asc",
			Property:     sortBy,
			IgnoreCase:   false,
		}
		response := TranslatedCardResponseByPagination{
			TotalPages:       0,
			TotalElements:    0,
			First:            true,
			Last:             true,
			Size:             size,
			Content:          []utils.TranslatedCard{},
			Number:           page,
			Sort:             []Sort{sortObj},
			NumberOfElements: 0,
			Pageable: Pageable{
				Offset:     offset,
				Sort:       []Sort{sortObj},
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
	if len(currencySynonyms) > 0 {
		for i, syn := range currencySynonyms {
			pattern := "%" + syn + "%"
			if i == 0 {
				query = query.Where("currency ILIKE ?", pattern)
			} else {
				query = query.Or("currency ILIKE ?", pattern)
			}
		}
	}
	if system != "" {
		query = query.Where("system ILIKE ?", "%"+system+"%")
	}
	if search != "" {
		query = query.Where("bank_name ILIKE ?", "%"+search+"%")
	} else if bankFilter != "" {
		query = query.Where("bank_name ILIKE ?", "%"+bankFilter+"%")
	}
	if opening == "bank" {
		query = query.Where("opening_type ILIKE '%Bank%'").Where("opening_type NOT ILIKE '%Onlayn%'")
	} else if opening == "online" || opening == "onlayn" {
		query = query.Where("opening_type ILIKE '%Onlayn%'").Where("opening_type NOT ILIKE '%Bank%'")
	} else if opening == "all" {
		query = query.Where("opening_type ILIKE '%Bank%'").Where("opening_type ILIKE '%Onlayn%'")
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

	// Применяем переводы к каждому элементу (uz/ru/en/oz)
	translator := utils.GetCardTranslator()
	translatedContent := make([]utils.TranslatedCard, 0, len(cards))
	for _, item := range cards {
		translated := translator.TranslateCard(
			item.BankName,
			item.Title,
			item.Currency,
			item.System,
			item.OpeningType,
		)
		translated.ID = item.ID
		translated.CreatedAt = item.CreatedAt.Format("2006-01-02T15:04:05.000000Z")
		translatedContent = append(translatedContent, translated)
	}

	// Создание объекта сортировки
	sortObj := Sort{
		Direction:    strings.ToUpper(sortDir),
		NullHandling: "NATIVE",
		Ascending:    strings.ToLower(sortDir) == "asc",
		Property:     sortBy,
		IgnoreCase:   false,
	}

	// Формирование ответа с переводами
	response := TranslatedCardResponseByPagination{
		TotalPages:       totalPages,
		TotalElements:    totalElements,
		First:            page == 0,
		Last:             page >= totalPages-1,
		Size:             size,
		Content:          translatedContent,
		Number:           page,
		Sort:             []Sort{sortObj},
		NumberOfElements: len(translatedContent),
		Pageable: Pageable{
			Offset:     offset,
			Sort:       []Sort{sortObj},
			Paged:      true,
			PageNumber: page,
			PageSize:   size,
			Unpaged:    false,
		},
		Empty: len(translatedContent) == 0,
	}

	c.JSON(http.StatusOK, gin.H{"result": response, "success": true})
}

// GetNewCreditCards возвращает кредитные карты с пагинацией
func (cc *CardController) GetNewCreditCards(c *gin.Context) {
	cc.getCreditCardsWithPagination(c, "new_credit_card")
}

func (cc *CardController) getCreditCardsWithPagination(c *gin.Context, tableName string) {
	db := utils.GetDB()

	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	search := c.Query("search")

	if page < 0 {
		page = 0
	}
	if size < 1 || size > 100 {
		size = 10
	}
	offset := page * size

	query := db.Table(tableName)
	if search != "" {
		query = query.Where("bank_name ILIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var items []models.CreditCard
	dataQuery := db.Table(tableName)
	if search != "" {
		dataQuery = dataQuery.Where("bank_name ILIKE ?", "%"+search+"%")
	}

	if err := dataQuery.Order("created_at DESC").Offset(offset).Limit(size).Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при получении данных"})
		return
	}

	// Применяем переводы к каждому элементу (uz/ru/en/oz)
	translator := utils.GetCardTranslator()
	translatedContent := make([]utils.TranslatedCreditCard, 0, len(items))
	for _, item := range items {
		translated := translator.TranslateCreditCard(
			item.BankName,
			item.Title,
			item.Rate,
			item.Term,
			item.Amount,
		)
		translated.ID = item.ID
		translated.CreatedAt = item.CreatedAt.Format("2006-01-02T15:04:05.000000Z")
		translatedContent = append(translatedContent, translated)
	}

	totalPages := int((total + int64(size) - 1) / int64(size))
	response := TranslatedCreditCardResponseByPagination{
		TotalPages:       totalPages,
		TotalElements:    total,
		First:            page == 0,
		Last:             page >= totalPages-1,
		Size:             size,
		Number:           page,
		Sort:             []Sort{},
		NumberOfElements: len(translatedContent),
		Pageable:         Pageable{Offset: offset, Sort: []Sort{}, Paged: true, PageNumber: page, PageSize: size, Unpaged: false},
		Empty:            len(translatedContent) == 0,
		Content:          translatedContent,
	}

	c.JSON(http.StatusOK, gin.H{"result": response, "success": true})
}
