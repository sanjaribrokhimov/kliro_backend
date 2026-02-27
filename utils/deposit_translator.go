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

	// Rate: только цифры и % — убираем dan/gacha/от/до/from/to на ВСЕХ языках (включая uz)
	// dan (с %)
	danWithPercent := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%\s*dan`)
	translations["uz"] = danWithPercent.ReplaceAllString(translations["uz"], "$1 %")
	translations["ru"] = danWithPercent.ReplaceAllString(translations["ru"], "$1 %")
	translations["en"] = danWithPercent.ReplaceAllString(translations["en"], "$1 %")
	translations["oz"] = danWithPercent.ReplaceAllString(translations["oz"], "$1 %")

	// dan (без %)
	danNoPercent := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*dan`)
	translations["uz"] = danNoPercent.ReplaceAllString(translations["uz"], "$1")
	translations["ru"] = danNoPercent.ReplaceAllString(translations["ru"], "$1")
	translations["en"] = danNoPercent.ReplaceAllString(translations["en"], "$1")
	translations["oz"] = danNoPercent.ReplaceAllString(translations["oz"], "$1")

	// gacha с процентом
	gachaPatternWithPercent := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%\s*gacha`)
	translations["uz"] = gachaPatternWithPercent.ReplaceAllString(translations["uz"], "$1 %")
	translations["ru"] = gachaPatternWithPercent.ReplaceAllString(translations["ru"], "$1 %")
	translations["en"] = gachaPatternWithPercent.ReplaceAllString(translations["en"], "$1 %")
	translations["oz"] = gachaPatternWithPercent.ReplaceAllString(translations["oz"], "$1 %")

	// gacha как слово
	gachaWord := regexp.MustCompile(`\bgacha\b`)
	translations["uz"] = gachaWord.ReplaceAllString(translations["uz"], "")
	translations["ru"] = gachaWord.ReplaceAllString(translations["ru"], "")
	translations["en"] = gachaWord.ReplaceAllString(translations["en"], "")
	translations["oz"] = gachaWord.ReplaceAllString(translations["oz"], "")

	// Удаляем оставшиеся dan/gacha (uz), от/до/from/to/дан/гача
	translations["uz"] = regexp.MustCompile(`\s*dan\s*`).ReplaceAllString(translations["uz"], " ")
	translations["uz"] = regexp.MustCompile(`\s*gacha\s*`).ReplaceAllString(translations["uz"], " ")
	translations["ru"] = regexp.MustCompile(`\s*от\s*`).ReplaceAllString(translations["ru"], " ")
	translations["ru"] = regexp.MustCompile(`\s*до\s*`).ReplaceAllString(translations["ru"], " ")
	translations["en"] = regexp.MustCompile(`\s*from\s*`).ReplaceAllString(translations["en"], " ")
	translations["en"] = regexp.MustCompile(`\s*(?:to|up\s+to)\s*`).ReplaceAllString(translations["en"], " ")
	translations["oz"] = regexp.MustCompile(`\s*дан\s*`).ReplaceAllString(translations["oz"], " ")
	translations["oz"] = regexp.MustCompile(`\s*гача\s*`).ReplaceAllString(translations["oz"], " ")
	translations["uz"] = strings.Join(strings.Fields(translations["uz"]), " ")
	translations["ru"] = strings.Join(strings.Fields(translations["ru"]), " ")
	translations["en"] = strings.Join(strings.Fields(translations["en"]), " ")
	translations["oz"] = strings.Join(strings.Fields(translations["oz"]), " ")

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

	// Amount: только цифры — убираем валюту и dan/gacha
	amountLower = strings.ToLower(amount)

	// uz: убираем so'm, so'mgacha, dan, gacha
	translations["uz"] = regexp.MustCompile(`(?i)(\d+(?:\s+\d+)*)\s*so'?mgacha`).ReplaceAllString(translations["uz"], "$1")
	translations["uz"] = regexp.MustCompile(`(?i)(\d+(?:\s+\d+)*)\s*so'?mdan`).ReplaceAllString(translations["uz"], "$1")
	translations["uz"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*dan`).ReplaceAllString(translations["uz"], "$1")
	translations["uz"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*gacha`).ReplaceAllString(translations["uz"], "$1")
	translations["uz"] = regexp.MustCompile(`(?i)\s*so'?m\s*$`).ReplaceAllString(translations["uz"], "")
	translations["uz"] = regexp.MustCompile(`(?i)\s*(?:som|sum)\s*$`).ReplaceAllString(translations["uz"], "")

	// Убираем "AQSH dollaridan" и т.п. — оставляем только числа
	translations["ru"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*AQSH\s*dollaridan`).ReplaceAllString(translations["ru"], "$1")
	translations["en"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*AQSH\s*dollaridan`).ReplaceAllString(translations["en"], "$1")
	translations["oz"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*AQSH\s*dollaridan`).ReplaceAllString(translations["oz"], "$1")

	// "so'mdan" / "somdan" / "sumdan" — только числа
	translations["ru"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*so'?mdan`).ReplaceAllString(translations["ru"], "$1")
	translations["en"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*so'?mdan`).ReplaceAllString(translations["en"], "$1")
	translations["oz"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*so'?mdan`).ReplaceAllString(translations["oz"], "$1")
	translations["ru"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*(?:som|sum)dan`).ReplaceAllString(translations["ru"], "$1")
	translations["en"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*(?:som|sum)dan`).ReplaceAllString(translations["en"], "$1")
	translations["oz"] = regexp.MustCompile(`(\d+(?:\s+\d+)*)\s*(?:som|sum)dan`).ReplaceAllString(translations["oz"], "$1")

	// Удаляем оставшийся "dan" как отдельное слово (если остался)
	danWord := regexp.MustCompile(`\b(dan|дан)\b`)
	translations["ru"] = danWord.ReplaceAllString(translations["ru"], "")
	translations["en"] = danWord.ReplaceAllString(translations["en"], "")
	translations["oz"] = danWord.ReplaceAllString(translations["oz"], "")
	// Также удаляем "от" и "до" если они остались
	otWord := regexp.MustCompile(`\bот\b`)
	translations["ru"] = otWord.ReplaceAllString(translations["ru"], "")
	doWord := regexp.MustCompile(`\bдо\b`)
	translations["ru"] = doWord.ReplaceAllString(translations["ru"], "")
	fromWord := regexp.MustCompile(`\bfrom\b`)
	translations["en"] = fromWord.ReplaceAllString(translations["en"], "")
	toWord := regexp.MustCompile(`\b(?:to|up\s+to)\b`)
	translations["en"] = toWord.ReplaceAllString(translations["en"], "")

	// Обрабатываем "gacha" - убираем слово "gacha" без добавления "до"
	gachaPattern := regexp.MustCompile(`(\d+(?:\s+\d+)*)\s+gacha`)
	translations["ru"] = gachaPattern.ReplaceAllString(translations["ru"], "$1")
	translations["en"] = gachaPattern.ReplaceAllString(translations["en"], "$1")
	translations["oz"] = gachaPattern.ReplaceAllString(translations["oz"], "$1")

	// Удаляем существующие "от" и "до" из всех языков
	// Удаляем "от" (русский) - может быть между числами или отдельно
	otPattern := regexp.MustCompile(`\s*от\s*`)
	translations["ru"] = otPattern.ReplaceAllString(translations["ru"], " ")
	// Удаляем "до" (русский) - может быть между числами или отдельно
	doPattern := regexp.MustCompile(`\s*до\s*`)
	translations["ru"] = doPattern.ReplaceAllString(translations["ru"], " ")
	// Удаляем "from" (английский)
	fromPattern := regexp.MustCompile(`\s*from\s*`)
	translations["en"] = fromPattern.ReplaceAllString(translations["en"], " ")
	// Удаляем "to" и "up to" (английский)
	toPattern := regexp.MustCompile(`\s*(?:to|up\s+to)\s*`)
	translations["en"] = toPattern.ReplaceAllString(translations["en"], " ")
	// Удаляем "дан" и "гача" (узбекский кириллица)
	danOzPattern := regexp.MustCompile(`\s*дан\s*`)
	translations["oz"] = danOzPattern.ReplaceAllString(translations["oz"], " ")
	gachaOzPattern := regexp.MustCompile(`\s*гача\s*`)
	translations["oz"] = gachaOzPattern.ReplaceAllString(translations["oz"], " ")

	// Убираем валюту в конце — amount только цифры
	translations["ru"] = regexp.MustCompile(`\s+So'?m\s*$`).ReplaceAllString(translations["ru"], "")
	translations["en"] = regexp.MustCompile(`\s+So'?m\s*$`).ReplaceAllString(translations["en"], "")
	translations["oz"] = regexp.MustCompile(`\s+So'?m\s*$`).ReplaceAllString(translations["oz"], "")
	translations["ru"] = regexp.MustCompile(`\s+(?:som|sum)\s*$`).ReplaceAllString(translations["ru"], "")
	translations["en"] = regexp.MustCompile(`\s+(?:som|sum)\s*$`).ReplaceAllString(translations["en"], "")
	translations["oz"] = regexp.MustCompile(`\s+(?:som|sum)\s*$`).ReplaceAllString(translations["oz"], "")
	// Убираем оставшиеся слова валют (сум, UZS, сўм, долларов США)
	translations["ru"] = regexp.MustCompile(`\s*сум\s*`).ReplaceAllString(translations["ru"], " ")
	translations["en"] = regexp.MustCompile(`\s*UZS\s*`).ReplaceAllString(translations["en"], " ")
	translations["oz"] = regexp.MustCompile(`\s*сўм\s*`).ReplaceAllString(translations["oz"], " ")
	translations["ru"] = regexp.MustCompile(`\s*долларов США\s*`).ReplaceAllString(translations["ru"], " ")
	translations["en"] = regexp.MustCompile(`\s*USD\s*`).ReplaceAllString(translations["en"], " ")
	translations["oz"] = regexp.MustCompile(`\s*АҚШ доллари\s*`).ReplaceAllString(translations["oz"], " ")

	// Финальная очистка "от" и "до" перед нормализацией пробелов
	// Удаляем "от" и "до" (русский)
	otFinalPattern := regexp.MustCompile(`\s*от\s*`)
	translations["ru"] = otFinalPattern.ReplaceAllString(translations["ru"], " ")
	doFinalPattern := regexp.MustCompile(`\s*до\s*`)
	translations["ru"] = doFinalPattern.ReplaceAllString(translations["ru"], " ")
	// Удаляем "from", "to", "up to" (английский)
	fromFinalPattern := regexp.MustCompile(`\s*from\s*`)
	translations["en"] = fromFinalPattern.ReplaceAllString(translations["en"], " ")
	toFinalPattern := regexp.MustCompile(`\s*(?:to|up\s+to)\s*`)
	translations["en"] = toFinalPattern.ReplaceAllString(translations["en"], " ")
	// Удаляем "дан" и "гача" (узбекский кириллица)
	danFinalPattern := regexp.MustCompile(`\s*дан\s*`)
	translations["oz"] = danFinalPattern.ReplaceAllString(translations["oz"], " ")
	gachaFinalPattern := regexp.MustCompile(`\s*гача\s*`)
	translations["oz"] = gachaFinalPattern.ReplaceAllString(translations["oz"], " ")

	// Убираем лишние пробелы
	translations["uz"] = strings.Join(strings.Fields(translations["uz"]), " ")
	translations["ru"] = strings.Join(strings.Fields(translations["ru"]), " ")
	translations["en"] = strings.Join(strings.Fields(translations["en"]), " ")
	translations["oz"] = strings.Join(strings.Fields(translations["oz"]), " ")

	return translations
}
