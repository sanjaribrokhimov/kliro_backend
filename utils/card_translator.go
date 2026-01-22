package utils

import (
	"regexp"
	"strings"
)

type CardLangData struct {
	Title       string `json:"title"`
	Currency    string `json:"currency"`
	System      string `json:"system"`
	OpeningType string `json:"opening_type"`
}

type TranslatedCard struct {
	ID        uint         `json:"id"`
	BankName  string       `json:"bank_name"`
	Uz        CardLangData `json:"uz"`
	Ru        CardLangData `json:"ru"`
	En        CardLangData `json:"en"`
	Oz        CardLangData `json:"oz"`
	CreatedAt string       `json:"created_at"`
}

type CreditCardLangData struct {
	Title  string `json:"title"`
	Rate   string `json:"rate"`
	Term   string `json:"term"`
	Amount string `json:"amount"`
}

type TranslatedCreditCard struct {
	ID        uint               `json:"id"`
	BankName  string             `json:"bank_name"`
	Uz        CreditCardLangData `json:"uz"`
	Ru        CreditCardLangData `json:"ru"`
	En        CreditCardLangData `json:"en"`
	Oz        CreditCardLangData `json:"oz"`
	CreatedAt string             `json:"created_at"`
}

type CardTranslator struct {
	translationService *TranslationService
}

var globalCardTranslator *CardTranslator

func GetCardTranslator() *CardTranslator {
	if globalCardTranslator == nil {
		globalCardTranslator = &CardTranslator{translationService: nil}
	}
	return globalCardTranslator
}

func (ct *CardTranslator) SetTranslationService(service *TranslationService) {
	ct.translationService = service
}

func (ct *CardTranslator) TranslateCard(bankName, title, currency, system, openingType string) TranslatedCard {
	titleTrans := ct.translateText(title)
	currencyTrans := ct.translateCurrency(currency)
	systemTrans := ct.translateSystem(system)
	openTrans := ct.translateOpening(openingType)

	return TranslatedCard{
		BankName: bankName,
		Uz: CardLangData{
			Title:       titleTrans["uz"],
			Currency:    currencyTrans["uz"],
			System:      systemTrans["uz"],
			OpeningType: openTrans["uz"],
		},
		Ru: CardLangData{
			Title:       titleTrans["ru"],
			Currency:    currencyTrans["ru"],
			System:      systemTrans["ru"],
			OpeningType: openTrans["ru"],
		},
		En: CardLangData{
			Title:       titleTrans["en"],
			Currency:    currencyTrans["en"],
			System:      systemTrans["en"],
			OpeningType: openTrans["en"],
		},
		Oz: CardLangData{
			Title:       titleTrans["oz"],
			Currency:    currencyTrans["oz"],
			System:      systemTrans["oz"],
			OpeningType: openTrans["oz"],
		},
	}
}

func (ct *CardTranslator) TranslateCreditCard(bankName, title, rate, term, amount string) TranslatedCreditCard {
	titleTrans := ct.translateText(title)
	rateTrans := ct.translateRate(rate)
	termTrans := ct.translateTerm(term)
	amountTrans := ct.translateAmount(amount)

	return TranslatedCreditCard{
		BankName: bankName,
		Uz: CreditCardLangData{Title: titleTrans["uz"], Rate: rateTrans["uz"], Term: termTrans["uz"], Amount: amountTrans["uz"]},
		Ru: CreditCardLangData{Title: titleTrans["ru"], Rate: rateTrans["ru"], Term: termTrans["ru"], Amount: amountTrans["ru"]},
		En: CreditCardLangData{Title: titleTrans["en"], Rate: rateTrans["en"], Term: termTrans["en"], Amount: amountTrans["en"]},
		Oz: CreditCardLangData{Title: titleTrans["oz"], Rate: rateTrans["oz"], Term: termTrans["oz"], Amount: amountTrans["oz"]},
	}
}

func (ct *CardTranslator) translateText(text string) map[string]string {
	// Title не переводим - оставляем оригинальное название для всех языков
	if text == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}
	return map[string]string{
		"uz": text,
		"ru": text,
		"en": text,
		"oz": text,
	}
}

func (ct *CardTranslator) translateCurrency(currency string) map[string]string {
	if currency == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}
	// Не ломаем значения типа "USD", "UZS"
	cur := strings.TrimSpace(currency)
	up := strings.ToUpper(cur)
	if regexp.MustCompile(`^[A-Z]{3}$`).MatchString(up) {
		return map[string]string{"uz": up, "ru": up, "en": up, "oz": up}
	}
	low := strings.ToLower(cur)
	switch {
	case strings.Contains(low, "aqsh") || strings.Contains(low, "usd") || strings.Contains(low, "dollar"):
		return map[string]string{"uz": "AQSH dollari", "ru": "Доллар США", "en": "US Dollar", "oz": "АҚШ доллари"}
	case strings.Contains(low, "yevro") || strings.Contains(low, "eur") || strings.Contains(low, "euro"):
		return map[string]string{"uz": "Yevro", "ru": "Евро", "en": "Euro", "oz": "Евро"}
	case strings.Contains(low, "so'm") || strings.Contains(low, "som") || strings.Contains(low, "sum") || strings.Contains(low, "uzs"):
		return map[string]string{"uz": "So'm", "ru": "Сум", "en": "UZS", "oz": "Сўм"}
	default:
		return map[string]string{"uz": cur, "ru": cur, "en": cur, "oz": TransliterateUzToOz(cur)}
	}
}

func (ct *CardTranslator) translateSystem(system string) map[string]string {
	if system == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}
	s := strings.TrimSpace(system)
	low := strings.ToLower(s)
	// Обычно Visa/Mastercard/Humo/Uzcard — оставляем как есть
	if regexp.MustCompile(`^[A-Za-z0-9\s\-]+$`).MatchString(s) && (strings.Contains(low, "visa") || strings.Contains(low, "master") || strings.Contains(low, "humo") || strings.Contains(low, "uzcard")) {
		return map[string]string{"uz": s, "ru": s, "en": s, "oz": s}
	}
	return ct.translateText(s)
}

func (ct *CardTranslator) translateOpening(opening string) map[string]string {
	if opening == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}
	low := strings.ToLower(strings.TrimSpace(opening))
	res := map[string]string{"uz": opening, "ru": opening, "en": opening, "oz": opening}

	// Частые шаблоны: "Bank", "Onlayn", "BankOnlayn"
	if strings.Contains(low, "bankonlayn") {
		res["uz"] = "BankOnlayn"
		res["ru"] = "БанкОнлайн"
		res["en"] = "BankOnline"
		res["oz"] = "БанкОнлайн"
		return res
	}
	if strings.Contains(low, "onlayn") || strings.Contains(low, "online") {
		res["ru"] = strings.NewReplacer("Onlayn", "Онлайн", "onlayn", "онлайн", "Online", "Online", "online", "online").Replace(res["ru"])
		res["en"] = strings.NewReplacer("Onlayn", "Online", "onlayn", "online").Replace(res["en"])
		res["oz"] = strings.NewReplacer("Onlayn", "Онлайн", "onlayn", "онлайн").Replace(res["oz"])
	}
	if strings.Contains(low, "bank") {
		res["ru"] = strings.NewReplacer("Bank", "Банк", "bank", "банк").Replace(res["ru"])
		res["oz"] = strings.NewReplacer("Bank", "Банк", "bank", "банк").Replace(res["oz"])
	}
	// Если все равно непонятно — переведем текст
	if ct.translationService != nil && res["ru"] == opening && res["en"] == opening {
		return ct.translateText(opening)
	}
	return res
}

func (ct *CardTranslator) translateRate(rate string) map[string]string {
	// Реиспользуем логику из deposit translator (достаточно для кредитных карт)
	dt := GetDepositTranslator()
	dt.translationService = ct.translationService
	return dt.translateRate(rate)
}

func (ct *CardTranslator) translateTerm(term string) map[string]string {
	dt := GetDepositTranslator()
	dt.translationService = ct.translationService
	return dt.translateTerm(term)
}

func (ct *CardTranslator) translateAmount(amount string) map[string]string {
	dt := GetDepositTranslator()
	dt.translationService = ct.translationService
	return dt.translateAmount(amount)
}

