package utils

import (
	"regexp"
	"strconv"
	"strings"
)

type DepositLangData struct {
	Title     string `json:"title"`
	Rate      string `json:"rate"`
	Term      string `json:"term"`
	MinAmount string `json:"min_amount"`
}

type TranslatedDeposit struct {
	ID        uint            `json:"id"`
	BankName  string          `json:"bank_name"`
	Uz        DepositLangData `json:"uz"`
	Ru        DepositLangData `json:"ru"`
	En        DepositLangData `json:"en"`
	Oz        DepositLangData `json:"oz"`
	CreatedAt string          `json:"created_at"`
}

type DepositTranslator struct {
	translationService *TranslationService
}

var globalDepositTranslator *DepositTranslator

func GetDepositTranslator() *DepositTranslator {
	if globalDepositTranslator == nil {
		globalDepositTranslator = &DepositTranslator{translationService: nil}
	}
	return globalDepositTranslator
}

func (dt *DepositTranslator) SetTranslationService(service *TranslationService) {
	dt.translationService = service
}

func (dt *DepositTranslator) TranslateDeposit(bankName, title, rate, termYears, minAmount string) TranslatedDeposit {
	titleTrans := dt.translateTitle(title)
	rateTrans := dt.translateRate(rate)
	termTrans := dt.translateTerm(termYears)
	amountTrans := dt.translateAmount(minAmount)

	return TranslatedDeposit{
		BankName: bankName,
		Uz: DepositLangData{
			Title:     titleTrans["uz"],
			Rate:      rateTrans["uz"],
			Term:      termTrans["uz"],
			MinAmount: amountTrans["uz"],
		},
		Ru: DepositLangData{
			Title:     titleTrans["ru"],
			Rate:      rateTrans["ru"],
			Term:      termTrans["ru"],
			MinAmount: amountTrans["ru"],
		},
		En: DepositLangData{
			Title:     titleTrans["en"],
			Rate:      rateTrans["en"],
			Term:      termTrans["en"],
			MinAmount: amountTrans["en"],
		},
		Oz: DepositLangData{
			Title:     titleTrans["oz"],
			Rate:      rateTrans["oz"],
			Term:      termTrans["oz"],
			MinAmount: amountTrans["oz"],
		},
	}
}

func (dt *DepositTranslator) translateTitle(title string) map[string]string {
	if title == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}

	// Title не переводим - оставляем оригинальное название для всех языков
	return map[string]string{
		"uz": title,
		"ru": title,
		"en": title,
		"oz": title,
	}
}

func (dt *DepositTranslator) translateRate(rate string) map[string]string {
	if rate == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}

	translations := map[string]string{
		"uz": rate,
		"ru": rate,
		"en": rate,
		"oz": rate,
	}

	// Примеры:
	// - "17 % dan" -> "17 % от" / "17 % from" / "17 % дан"
	// - "22.52dan - 25 % gacha" -> "22.52 от - 25 % до" / "22.52 from - 25 % up to" / "22.52 дан - 25 % гача"

	// dan (с %), включая слитное "22.52dan"
	danWithPercent := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%\s*dan`)
	translations["ru"] = danWithPercent.ReplaceAllString(translations["ru"], "$1 % от")
	translations["en"] = danWithPercent.ReplaceAllString(translations["en"], "$1 % from")
	translations["oz"] = danWithPercent.ReplaceAllString(translations["oz"], "$1 % дан")

	// dan (без %), включая слитное "22.52dan"
	danNoPercent := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*dan`)
	translations["ru"] = danNoPercent.ReplaceAllString(translations["ru"], "$1 от")
	translations["en"] = danNoPercent.ReplaceAllString(translations["en"], "$1 from")
	translations["oz"] = danNoPercent.ReplaceAllString(translations["oz"], "$1 дан")

	// gacha - сначала обрабатываем с процентом "25 % gacha"
	gachaPatternWithPercent := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%\s*gacha`)
	translations["ru"] = gachaPatternWithPercent.ReplaceAllString(translations["ru"], "$1 % до")
	translations["en"] = gachaPatternWithPercent.ReplaceAllString(translations["en"], "$1 % up to")
	translations["oz"] = gachaPatternWithPercent.ReplaceAllString(translations["oz"], "$1 % гача")

	// Затем обрабатываем просто "gacha" как слово
	gachaWord := regexp.MustCompile(`\bgacha\b`)
	translations["ru"] = gachaWord.ReplaceAllString(translations["ru"], "до")
	translations["en"] = gachaWord.ReplaceAllString(translations["en"], "up to")
	translations["oz"] = gachaWord.ReplaceAllString(translations["oz"], "гача")

	return translations
}

func (dt *DepositTranslator) translateTerm(termYears string) map[string]string {
	if termYears == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}

	res := map[string]string{
		"uz": termYears,
		"ru": termYears,
		"en": termYears,
		"oz": termYears,
	}

	low := strings.ToLower(termYears)

	// Правильные формы для ru/en/oz: "1 yil" -> "1 год", "2 yil" -> "2 года", "5 yil" -> "5 лет"
	// и "1 oy" -> "1 мес.", "2 oy" -> "2 мес.", en: year/years, month/months
	reYear := regexp.MustCompile(`(\d+)\s*yil\b`)
	reMonth := regexp.MustCompile(`(\d+)\s*oy\b`)

	// Обрабатываем года
	if strings.Contains(low, "yil") {
		res["ru"] = reYear.ReplaceAllStringFunc(res["ru"], func(m string) string {
			sub := reYear.FindStringSubmatch(m)
			if len(sub) < 2 {
				return m
			}
			n, _ := strconv.Atoi(sub[1])
			// русские формы: 1 год, 2-4 года, 5+ лет
			word := "лет"
			if n%10 == 1 && n%100 != 11 {
				word = "год"
			} else if (n%10 >= 2 && n%10 <= 4) && !(n%100 >= 12 && n%100 <= 14) {
				word = "года"
			}
			return sub[1] + " " + word
		})
		res["en"] = reYear.ReplaceAllStringFunc(res["en"], func(m string) string {
			sub := reYear.FindStringSubmatch(m)
			if len(sub) < 2 {
				return m
			}
			n, _ := strconv.Atoi(sub[1])
			word := "years"
			if n == 1 {
				word = "year"
			}
			return sub[1] + " " + word
		})
		res["oz"] = reYear.ReplaceAllString(res["oz"], `$1 йил`)
	}

	// Обрабатываем месяцы
	if strings.Contains(low, "oy") {
		res["ru"] = reMonth.ReplaceAllStringFunc(res["ru"], func(m string) string {
			sub := reMonth.FindStringSubmatch(m)
			if len(sub) < 2 {
				return m
			}
			n, _ := strconv.Atoi(sub[1])
			word := "мес."
			if n == 1 {
				word = "мес."
			}
			return sub[1] + " " + word
		})
		res["en"] = reMonth.ReplaceAllStringFunc(res["en"], func(m string) string {
			sub := reMonth.FindStringSubmatch(m)
			if len(sub) < 2 {
				return m
			}
			n, _ := strconv.Atoi(sub[1])
			word := "months"
			if n == 1 {
				word = "month"
			}
			return sub[1] + " " + word
		})
		res["oz"] = reMonth.ReplaceAllString(res["oz"], `$1 ой`)
	}

	// Обрабатываем "dan" и "gacha" если они есть в term (иногда там попадаются неправильные данные)
	// "dan" - "от"
	danPattern := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*dan`)
	res["ru"] = danPattern.ReplaceAllString(res["ru"], "$1 от")
	res["en"] = danPattern.ReplaceAllString(res["en"], "$1 from")
	res["oz"] = danPattern.ReplaceAllString(res["oz"], "$1 дан")

	// "gacha" - "до"
	gachaPattern := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*gacha`)
	res["ru"] = gachaPattern.ReplaceAllString(res["ru"], "$1 до")
	res["en"] = gachaPattern.ReplaceAllString(res["en"], "$1 to")
	res["oz"] = gachaPattern.ReplaceAllString(res["oz"], "$1 гача")

	// "gacha" с процентом
	gachaPatternWithPercent := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%\s*gacha`)
	res["ru"] = gachaPatternWithPercent.ReplaceAllString(res["ru"], "$1 % до")
	res["en"] = gachaPatternWithPercent.ReplaceAllString(res["en"], "$1 % to")
	res["oz"] = gachaPatternWithPercent.ReplaceAllString(res["oz"], "$1 % гача")

	// Если term уже на русском — oz делаем транслитом
	if res["ru"] != termYears && res["ru"] != "" && res["oz"] == termYears {
		res["oz"] = TransliterateRuToOz(res["ru"])
	} else if res["oz"] == termYears {
		res["oz"] = TransliterateUzToOz(termYears)
	}
	return res
}

func (dt *DepositTranslator) translateAmount(amount string) map[string]string {
	if amount == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}

	amountLower := strings.ToLower(amount)
	if strings.Contains(amountLower, "ko'rsatilmagan") || strings.Contains(amountLower, "ko`rsatilmagan") {
		return map[string]string{
			"uz": amount,
			"ru": "Не указано",
			"en": "Not specified",
			"oz": "Кўрсатилмаган",
		}
	}

	translations := map[string]string{
		"uz": amount,
		"ru": amount,
		"en": amount,
		"oz": amount,
	}

	// Обрабатываем валюты и убираем "dan"
	amountLower = strings.ToLower(amount)

	// "AQSH dollaridan" -> "100 000 AQSH dollaridan" -> "100 000 USD" / "100 000 долларов США"
	if strings.Contains(amountLower, "aqsh") && strings.Contains(amountLower, "dollar") {
		// Сохраняем числа перед заменой
		translations["ru"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*AQSH\s*dollaridan`).ReplaceAllString(translations["ru"], "$1 долларов США")
		translations["en"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*AQSH\s*dollaridan`).ReplaceAllString(translations["en"], "$1 USD")
		translations["oz"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*AQSH\s*dollaridan`).ReplaceAllString(translations["oz"], "$1 АҚШ доллари")
	}

	// "so'mdan" / "somdan" / "sumdan" -> убираем "dan" и переводим валюту
	translations["ru"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*so'?mdan`).ReplaceAllString(translations["ru"], "$1 сум")
	translations["en"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*so'?mdan`).ReplaceAllString(translations["en"], "$1 UZS")
	translations["oz"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*so'?mdan`).ReplaceAllString(translations["oz"], "$1 сўм")

	// Также обрабатываем варианты "somdan", "sumdan"
	translations["ru"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*(?:som|sum)dan`).ReplaceAllString(translations["ru"], "$1 сум")
	translations["en"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*(?:som|sum)dan`).ReplaceAllString(translations["en"], "$1 UZS")
	translations["oz"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*(?:som|sum)dan`).ReplaceAllString(translations["oz"], "$1 сўм")

	// Удаляем оставшийся "dan" как отдельное слово (если остался)
	danWord := regexp.MustCompile(`\b(dan|дан)\b`)
	translations["ru"] = danWord.ReplaceAllString(translations["ru"], "")
	translations["en"] = danWord.ReplaceAllString(translations["en"], "")
	translations["oz"] = danWord.ReplaceAllString(translations["oz"], "")

	// Обрабатываем "gacha" - "до"
	gachaPattern := regexp.MustCompile(`(\d+(?:\s+\d+)*)\s+gacha`)
	translations["ru"] = gachaPattern.ReplaceAllString(translations["ru"], "$1 до")
	translations["en"] = gachaPattern.ReplaceAllString(translations["en"], "$1 up to")
	translations["oz"] = gachaPattern.ReplaceAllString(translations["oz"], "$1 гача")

	// Обрабатываем "So'm" / "so'm" / "som" / "sum" в конце
	translations["ru"] = regexp.MustCompile(`\s+So'?m\s*$`).ReplaceAllString(translations["ru"], " сум")
	translations["en"] = regexp.MustCompile(`\s+So'?m\s*$`).ReplaceAllString(translations["en"], " UZS")
	translations["oz"] = regexp.MustCompile(`\s+So'?m\s*$`).ReplaceAllString(translations["oz"], " сўм")

	// Также обрабатываем варианты "som", "sum"
	translations["ru"] = regexp.MustCompile(`\s+(?:som|sum)\s*$`).ReplaceAllString(translations["ru"], " сум")
	translations["en"] = regexp.MustCompile(`\s+(?:som|sum)\s*$`).ReplaceAllString(translations["en"], " UZS")
	translations["oz"] = regexp.MustCompile(`\s+(?:som|sum)\s*$`).ReplaceAllString(translations["oz"], " сўм")

	// Убираем лишние пробелы
	translations["ru"] = strings.Join(strings.Fields(translations["ru"]), " ")
	translations["en"] = strings.Join(strings.Fields(translations["en"]), " ")
	translations["oz"] = strings.Join(strings.Fields(translations["oz"]), " ")

	return translations
}
