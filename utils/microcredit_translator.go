package utils

import (
	"regexp"
	"strings"
)

// MicrocreditTranslator - утилита для перевода полей микрокредитов
type MicrocreditTranslator struct {
	translationService *TranslationService
}

var globalTranslator *MicrocreditTranslator

// GetMicrocreditTranslator - возвращает экземпляр переводчика
func GetMicrocreditTranslator() *MicrocreditTranslator {
	if globalTranslator == nil {
		globalTranslator = &MicrocreditTranslator{
			translationService: nil, // Будет инициализирован при первом использовании
		}
	}
	return globalTranslator
}

// SetTranslationService устанавливает сервис переводов
func (mt *MicrocreditTranslator) SetTranslationService(service *TranslationService) {
	mt.translationService = service
}

// MicrocreditLangData - данные микрокредита для одного языка
type MicrocreditLangData struct {
	Description string `json:"description"`
	Rate        string `json:"rate"`
	Term        string `json:"term"`
	Amount      string `json:"amount"`
	Channel     string `json:"channel"`
}

// TranslatedMicrocredit - структура микрокредита с переводами (каждый язык отдельным объектом)
type TranslatedMicrocredit struct {
	ID        uint                `json:"id"`
	BankName  string              `json:"bank_name"`
	Uz        MicrocreditLangData `json:"uz"`
	Ru        MicrocreditLangData `json:"ru"`
	En        MicrocreditLangData `json:"en"`
	Oz        MicrocreditLangData `json:"oz"`
	URL       string              `json:"url"`
	CreatedAt string              `json:"created_at"`
}

// TranslatedAutocredit - структура автокредита с переводами (каждый язык отдельным объектом)
type TranslatedAutocredit struct {
	ID        uint                `json:"id"`
	BankName  string              `json:"bank_name"`
	Uz        MicrocreditLangData `json:"uz"`
	Ru        MicrocreditLangData `json:"ru"`
	En        MicrocreditLangData `json:"en"`
	Oz        MicrocreditLangData `json:"oz"`
	CreatedAt string              `json:"created_at"`
}

// TranslateMicrocredit - переводит микрокредит на 4 языка (каждый язык отдельным объектом)
func (mt *MicrocreditTranslator) TranslateMicrocredit(bankName, description, rate, term, amount, channel string) TranslatedMicrocredit {
	descTrans := mt.translateDescription(description)
	rateTrans := mt.translateRate(rate)
	termTrans := mt.translateTerm(term)
	amountTrans := mt.translateAmount(amount)
	channelTrans := mt.translateChannel(channel)

	return TranslatedMicrocredit{
		BankName: bankName,
		Uz: MicrocreditLangData{
			Description: descTrans["uz"],
			Rate:        rateTrans["uz"],
			Term:        termTrans["uz"],
			Amount:      amountTrans["uz"],
			Channel:     channelTrans["uz"],
		},
		Ru: MicrocreditLangData{
			Description: descTrans["ru"],
			Rate:        rateTrans["ru"],
			Term:        termTrans["ru"],
			Amount:      amountTrans["ru"],
			Channel:     channelTrans["ru"],
		},
		En: MicrocreditLangData{
			Description: descTrans["en"],
			Rate:        rateTrans["en"],
			Term:        termTrans["en"],
			Amount:      amountTrans["en"],
			Channel:     channelTrans["en"],
		},
		Oz: MicrocreditLangData{
			Description: descTrans["oz"],
			Rate:        rateTrans["oz"],
			Term:        termTrans["oz"],
			Amount:      amountTrans["oz"],
			Channel:     channelTrans["oz"],
		},
	}
}

// TranslateAutocredit - переводит автокредит на 4 языка (каждый язык отдельным объектом)
func (mt *MicrocreditTranslator) TranslateAutocredit(bankName, description, rate, term, amount, channel string) TranslatedAutocredit {
	descTrans := mt.translateDescription(description)
	rateTrans := mt.translateRate(rate)
	termTrans := mt.translateTerm(term)
	amountTrans := mt.translateAmount(amount)
	channelTrans := mt.translateChannel(channel)

	return TranslatedAutocredit{
		BankName: bankName,
		Uz: MicrocreditLangData{
			Description: descTrans["uz"],
			Rate:        rateTrans["uz"],
			Term:        termTrans["uz"],
			Amount:      amountTrans["uz"],
			Channel:     channelTrans["uz"],
		},
		Ru: MicrocreditLangData{
			Description: descTrans["ru"],
			Rate:        rateTrans["ru"],
			Term:        termTrans["ru"],
			Amount:      amountTrans["ru"],
			Channel:     channelTrans["ru"],
		},
		En: MicrocreditLangData{
			Description: descTrans["en"],
			Rate:        rateTrans["en"],
			Term:        termTrans["en"],
			Amount:      amountTrans["en"],
			Channel:     channelTrans["en"],
		},
		Oz: MicrocreditLangData{
			Description: descTrans["oz"],
			Rate:        rateTrans["oz"],
			Term:        termTrans["oz"],
			Amount:      amountTrans["oz"],
			Channel:     channelTrans["oz"],
		},
	}
}

// translateBankName - название банка остается как есть (не переводим)
func (mt *MicrocreditTranslator) translateBankName(bankName string) map[string]string {
	return map[string]string{
		"uz": bankName,
		"ru": bankName,
		"en": bankName,
		"oz": bankName,
	}
}

// translateDescription - автоматический перевод описания
// НЕ ПЕРЕВОДИТ - оставляет оригинальное название для всех языков
func (mt *MicrocreditTranslator) translateDescription(desc string) map[string]string {
	if desc == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}

	// Description не переводим - оставляем оригинальное название для всех языков
	return map[string]string{
		"uz": desc,
		"ru": desc,
		"en": desc,
		"oz": desc,
	}
}

// translateRate - автоматический перевод процентной ставки
func (mt *MicrocreditTranslator) translateRate(rate string) map[string]string {
	if rate == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}

	// Специальная обработка для "Ko'rsatilmagan"
	rateLower := strings.ToLower(rate)
	if strings.Contains(rateLower, "ko'rsatilmagan") || strings.Contains(rateLower, "ko`rsatilmagan") {
		return map[string]string{
			"uz": rate,
			"ru": "Не указано",
			"en": "Not specified",
			"oz": "Кўрсатилмаган",
		}
	}

	translations := map[string]string{
		"uz": rate,
		"ru": rate,
		"en": rate,
		"oz": rate,
	}

	// Заменяем "dan" - обрабатываем случаи: "27 % dan", "31dan", "23dan"
	// Сначала обрабатываем "dan" с процентом
	danPatternWithPercent := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%\s*dan`)
	translations["ru"] = danPatternWithPercent.ReplaceAllString(translations["ru"], "$1 % от")
	translations["en"] = danPatternWithPercent.ReplaceAllString(translations["en"], "$1 % from")
	translations["oz"] = danPatternWithPercent.ReplaceAllString(translations["oz"], "$1 % дан")

	// Затем обрабатываем "dan" без процента (только если не было замены выше)
	danPattern := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*dan`)
	if !danPatternWithPercent.MatchString(rate) {
		translations["ru"] = danPattern.ReplaceAllString(translations["ru"], "$1 от")
		translations["en"] = danPattern.ReplaceAllString(translations["en"], "$1 from")
		translations["oz"] = danPattern.ReplaceAllString(translations["oz"], "$1 дан")
	}

	// Заменяем "gacha" - обрабатываем случаи: "34 % gacha", "25 % gacha"
	// Сначала обрабатываем "gacha" с процентом
	gachaPatternWithPercent := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%\s*gacha`)
	translations["ru"] = gachaPatternWithPercent.ReplaceAllString(translations["ru"], "$1 % до")
	translations["en"] = gachaPatternWithPercent.ReplaceAllString(translations["en"], "$1 % to")
	translations["oz"] = gachaPatternWithPercent.ReplaceAllString(translations["oz"], "$1 % гача")

	// Затем обрабатываем "gacha" без процента (только если не было замены выше)
	gachaPattern := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*gacha`)
	if !gachaPatternWithPercent.MatchString(rate) {
		translations["ru"] = gachaPattern.ReplaceAllString(translations["ru"], "$1 до")
		translations["en"] = gachaPattern.ReplaceAllString(translations["en"], "$1 to")
		translations["oz"] = gachaPattern.ReplaceAllString(translations["oz"], "$1 гача")
	}

	// Для oz используем транслитерацию, если есть русский перевод
	if translations["ru"] != rate && translations["ru"] != "" {
		// Если есть русский перевод, используем его для oz (кириллица совместима)
		translations["oz"] = translations["ru"]
	} else if translations["oz"] == rate {
		// Если перевода нет, транслитерируем оригинал
		translations["oz"] = TransliterateUzToOz(rate)
	}

	return translations
}

// translateTerm - автоматический перевод срока
func (mt *MicrocreditTranslator) translateTerm(term string) map[string]string {
	if term == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}

	translations := map[string]string{
		"uz": term,
		"ru": term,
		"en": term,
		"oz": term,
	}

	// Заменяем "oy" на переводы
	if strings.Contains(strings.ToLower(term), "oy") {
		translations["ru"] = strings.ReplaceAll(strings.ReplaceAll(term, "oy", "мес."), "oy", "мес.")
		translations["en"] = strings.ReplaceAll(term, "oy", "months")
		translations["oz"] = strings.ReplaceAll(term, "oy", "ой")
	}

	// Заменяем "yil" на переводы
	if strings.Contains(strings.ToLower(term), "yil") {
		translations["ru"] = strings.ReplaceAll(translations["ru"], "yil", "лет")
		translations["en"] = strings.ReplaceAll(translations["en"], "yil", "years")
		translations["oz"] = strings.ReplaceAll(translations["oz"], "yil", "йил")
	}

	// Заменяем "месяц" на переводы (если уже есть русский текст)
	if strings.Contains(strings.ToLower(term), "месяц") {
		translations["en"] = strings.ReplaceAll(term, "месяц", "months")
		translations["oz"] = strings.ReplaceAll(term, "месяц", "ой")
	}

	// Заменяем "год" на переводы
	if strings.Contains(strings.ToLower(term), "год") {
		translations["en"] = strings.ReplaceAll(translations["en"], "год", "years")
		translations["oz"] = strings.ReplaceAll(translations["oz"], "год", "йил")
	}

	// Для oz используем транслитерацию, если есть русский перевод
	if translations["ru"] != term && translations["ru"] != "" {
		// Если есть русский перевод, используем его для oz (кириллица совместима)
		translations["oz"] = translations["ru"]
	} else if translations["oz"] == term {
		// Если перевода нет, транслитерируем оригинал
		translations["oz"] = TransliterateUzToOz(term)
	}

	return translations
}

// translateAmount - автоматический перевод суммы
func (mt *MicrocreditTranslator) translateAmount(amount string) map[string]string {
	if amount == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}

	// Специальная обработка для "Ko'rsatilmagan"
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

	// Заменяем "so'm" и варианты (регистронезависимо)
	// amountLower уже объявлен выше

	// Сначала заменяем "so'mgacha" (более длинный паттерн)
	if strings.Contains(amountLower, "so'mgacha") {
		soomgachaPattern := regexp.MustCompile(`(?i)so'?mgacha`)
		translations["ru"] = soomgachaPattern.ReplaceAllString(translations["ru"], "сум")
		translations["en"] = soomgachaPattern.ReplaceAllString(translations["en"], "UZS")
		translations["oz"] = soomgachaPattern.ReplaceAllString(translations["oz"], "сўмгача")
	} else if strings.Contains(amountLower, "so'm") || strings.Contains(amountLower, "so`m") {
		// Затем заменяем "so'm" или "so`m"
		soomPattern := regexp.MustCompile(`(?i)so'?m`)
		translations["ru"] = soomPattern.ReplaceAllString(translations["ru"], "сум")
		translations["en"] = soomPattern.ReplaceAllString(translations["en"], "UZS")
		translations["oz"] = soomPattern.ReplaceAllString(translations["oz"], "сўм")
	}

	// Заменяем "dan" с пробелом перед ним (если есть число перед ним)
	danPattern := regexp.MustCompile(`(\d+)\s*dan`)
	translations["ru"] = danPattern.ReplaceAllString(translations["ru"], "$1 от")
	translations["en"] = danPattern.ReplaceAllString(translations["en"], "$1 from")
	translations["oz"] = danPattern.ReplaceAllString(translations["oz"], "$1 дан")

	// Заменяем "gacha" с пробелом перед ним
	gachaPattern := regexp.MustCompile(`(\d+)\s*gacha`)
	translations["ru"] = gachaPattern.ReplaceAllString(translations["ru"], "$1 до")
	translations["en"] = gachaPattern.ReplaceAllString(translations["en"], "$1 to")
	translations["oz"] = gachaPattern.ReplaceAllString(translations["oz"], "$1 гача")

	// Для oz используем транслитерацию, если есть русский перевод
	if translations["ru"] != amount && translations["ru"] != "" {
		// Если есть русский перевод, используем его для oz (кириллица совместима)
		translations["oz"] = translations["ru"]
	} else if translations["oz"] == amount {
		// Если перевода нет, транслитерируем оригинал
		translations["oz"] = TransliterateUzToOz(amount)
	}

	return translations
}

// translateChannel - автоматический перевод канала
func (mt *MicrocreditTranslator) translateChannel(channel string) map[string]string {
	if channel == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}

	channelLower := strings.ToLower(channel)

	translations := map[string]string{
		"uz": channel,
		"ru": channel,
		"en": channel,
		"oz": channel,
	}

	// Заменяем "onlayn" / "online"
	if strings.Contains(channelLower, "onlayn") || strings.Contains(channelLower, "online") {
		translations["ru"] = strings.ReplaceAll(strings.ReplaceAll(channel, "Onlayn", "Онлайн"), "onlayn", "онлайн")
		translations["en"] = strings.ReplaceAll(strings.ReplaceAll(channel, "Onlayn", "Online"), "onlayn", "online")
		translations["oz"] = strings.ReplaceAll(strings.ReplaceAll(channel, "Onlayn", "Онлайн"), "onlayn", "онлайн")
	}

	// Заменяем "bank" (только если это отдельное слово или часть "BankOnlayn")
	if strings.Contains(channelLower, "bank") {
		// Проверяем, не является ли это частью названия банка
		if channel == "Bank" || channel == "Onlayn" || channel == "BankOnlayn" {
			translations["ru"] = strings.ReplaceAll(translations["ru"], "Bank", "Банк")
			translations["oz"] = strings.ReplaceAll(translations["oz"], "Bank", "Банк")
		}
	}

	// Специальный случай для "BankOnlayn"
	if channel == "BankOnlayn" {
		translations["ru"] = "БанкОнлайн"
		translations["en"] = "BankOnline"
		translations["oz"] = "БанкОнлайн"
	}

	// Для oz используем транслитерацию, если есть русский перевод
	if translations["ru"] != channel && translations["ru"] != "" {
		// Если есть русский перевод, используем его для oz (кириллица совместима)
		translations["oz"] = translations["ru"]
	} else if translations["oz"] == channel {
		// Если перевода нет, транслитерируем оригинал
		translations["oz"] = TransliterateUzToOz(channel)
	}

	return translations
}

// translateByWords - переводит текст по словам автоматически
func (mt *MicrocreditTranslator) translateByWords(text string, wordMap map[string]map[string]string) map[string]string {
	if text == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}

	// Сначала проверяем полные фразы (приоритет)
	fullPhrases := map[string]map[string]string{
		"onlayn kredit": {
			"uz": "Onlayn kredit",
			"ru": "Онлайн кредит",
			"en": "Online credit",
			"oz": "Онлайн кредит",
		},
		"online mikroqarz": {
			"uz": "Online mikroqarz",
			"ru": "Онлайн микрозайм",
			"en": "Online microloan",
			"oz": "Онлайн микрозайм",
		},
		"onlayn mikroqarz": {
			"uz": "Onlayn mikroqarz",
			"ru": "Онлайн микрозайм",
			"en": "Online microloan",
			"oz": "Онлайн микрозайм",
		},
		"biznesga birinchi qadam": {
			"uz": "Biznesga birinchi qadam",
			"ru": "Первый шаг к бизнесу",
			"en": "First step to business",
			"oz": "Бизнесга биринчи қадам",
		},
		"biznesga madad": {
			"uz": "Biznesga Madad",
			"ru": "Помощь бизнесу",
			"en": "Business Support",
			"oz": "Бизнесга Мадад",
		},
		"biznesga madad mikroqarzi": {
			"uz": "Biznesga Madad mikroqarzi",
			"ru": "Микрозайм - помощь бизнесу",
			"en": "Microloan - Business Support",
			"oz": "Микрозайм - Бизнесга Мадад",
		},
		"biznesga birinchi qadam mikroqarzi": {
			"uz": "Biznesga birinchi qadam mikroqarzi",
			"ru": "Микрозайм - первый шаг к бизнесу",
			"en": "Microloan - first step to business",
			"oz": "Микрозайм - Бизнесга биринчи қадам",
		},
		"mahalla loyihasi": {
			"uz": "Mahalla loyihasi",
			"ru": "Проект Махалля",
			"en": "Mahalla Project",
			"oz": "Маҳалла лойиҳаси",
		},
		"mikroqarz": {
			"uz": "Mikroqarz",
			"ru": "Микрозайм",
			"en": "Microloan",
			"oz": "Микрозайм",
		},
		"avans plyus": {
			"uz": "Avans plyus",
			"ru": "Аванс плюс",
			"en": "Advance plus",
			"oz": "Аванс плюс",
		},
		"onlayn to'lov": {
			"uz": "Onlayn to'lov",
			"ru": "Онлайн платеж",
			"en": "Online payment",
			"oz": "Онлайн тўлов",
		},
		"mikroqarz — birinchi 30 kunda foizlarsiz qaytaring": {
			"uz": "Mikroqarz — birinchi 30 kunda foizlarsiz qaytaring",
			"ru": "Микрозайм — верните в первые 30 дней без процентов",
			"en": "Microloan — return in the first 30 days without interest",
			"oz": "Микрозайм — биринчи 30 кунда фойизларсиз қайтаринг",
		},
		"oflayn kredit": {
			"uz": "Oflayn kredit",
			"ru": "Офлайн кредит",
			"en": "Offline credit",
			"oz": "Офлайн кредит",
		},
		"arzon mikroqarzi": {
			"uz": "\"Arzon\" mikroqarzi",
			"ru": "Микрозайм \"Дешевый\"",
			"en": "Microloan \"Cheap\"",
			"oz": "\"Арзон\" микрозайми",
		},
		"avtokredit chevrolet": {
			"uz": "Avtokredit Chevrolet",
			"ru": "Автокредит Chevrolet",
			"en": "Auto credit Chevrolet",
			"oz": "Автокредит Chevrolet",
		},
		"avtokredit (birlamchi bozor)": {
			"uz": "Avtokredit (Birlamchi bozor)",
			"ru": "Автокредит (первичный рынок)",
			"en": "Auto credit (primary market)",
			"oz": "Автокредит (биринчи бозор)",
		},
		"oson ipoteka": {
			"uz": "Oson ipoteka",
			"ru": "Легкая ипотека",
			"en": "Easy mortgage",
			"oz": "Осон ипотека",
		},
		"ma'qul ipoteka krediti": {
			"uz": "Ma'qul ipoteka krediti",
			"ru": "Доступный ипотечный кредит",
			"en": "Affordable mortgage loan",
			"oz": "Маъқул ипотека кредити",
		},
		"maqul ipoteka krediti": {
			"uz": "Maqul ipoteka krediti",
			"ru": "Доступный ипотечный кредит",
			"en": "Affordable mortgage loan",
			"oz": "Маъқул ипотека кредити",
		},
		"avtokredit - birlamchi bozor uchun": {
			"uz": "Avtokredit - birlamchi bozor uchun",
			"ru": "Автокредит - для первичного рынка",
			"en": "Auto credit - for primary market",
			"oz": "Автокредит - биринчи бозор учун",
		},
		"avtokredit - ikkilamchi bozor uchun": {
			"uz": "Avtokredit - ikkilamchi bozor uchun",
			"ru": "Автокредит - для вторичного рынка",
			"en": "Auto credit - for secondary market",
			"oz": "Автокредит - иккиламчи бозор учун",
		},
		"avto premium": {
			"uz": "Avto Premium",
			"ru": "Авто Премиум",
			"en": "Auto Premium",
			"oz": "Авто Премиум",
		},
		"avtokredit – uzauto motors": {
			"uz": "Avtokredit – UzAuto Motors",
			"ru": "Автокредит – UzAuto Motors",
			"en": "Auto credit – UzAuto Motors",
			"oz": "Автокредит – UzAuto Motors",
		},
		"avtokredit - «mikroavtobus » va «minigruzovik»": {
			"uz": "Avtokredit - «mikroavtobus» va «minigruzovik»",
			"ru": "Автокредит - «микроавтобус» и «микрогрузовик»",
			"en": "Auto credit - «minibus» and «mini truck»",
			"oz": "Автокредит - «микроавтобус» ва «микрогрузовик»",
		},
		"birlamchi avtokredit": {
			"uz": "BIRLAMCHI AVTOKREDIT",
			"ru": "ПЕРВИЧНЫЙ АВТОКРЕДИТ",
			"en": "PRIMARY AUTO CREDIT",
			"oz": "БИРИНЧИ АВТОКРЕДИТ",
		},
		"apex jetour": {
			"uz": "APEX JETOUR",
			"ru": "APEX JETOUR",
			"en": "APEX JETOUR",
			"oz": "APEX JETOUR",
		},
	}

	textLower := strings.ToLower(text)
	// Убираем лишние пробелы и нормализуем
	textLower = strings.TrimSpace(textLower)
	textLower = regexp.MustCompile(`\s+`).ReplaceAllString(textLower, " ")

	for phrase, trans := range fullPhrases {
		phraseLower := strings.ToLower(phrase)
		if strings.Contains(textLower, phraseLower) {
			// Если текст полностью совпадает с фразой
			if textLower == phraseLower {
				return trans
			}
			// Если фраза является началом или концом текста
			if strings.HasPrefix(textLower, phraseLower) || strings.HasSuffix(textLower, phraseLower) {
				return trans
			}
			// Если фраза является основной частью (более 70% текста)
			if len(phraseLower) > 0 && float64(len(phraseLower))/float64(len(textLower)) > 0.7 {
				return trans
			}
		}
	}

	// Разбиваем на слова для пошагового перевода
	words := regexp.MustCompile(`\s+`).Split(text, -1)

	uzWords := []string{}
	ruWords := []string{}
	enWords := []string{}
	ozWords := []string{}

	for _, word := range words {
		if word == "" {
			continue
		}

		// Убираем знаки препинания для поиска (сохраняем дефисы и тире)
		cleanWord := strings.ToLower(strings.Trim(word, ".,!?;:\"'()[]{}«»"))
		originalWord := word

		// Обрабатываем дефисы и тире отдельно
		if strings.Contains(cleanWord, "—") || strings.Contains(cleanWord, "-") {
			// Разбиваем по дефису/тире и переводим каждую часть
			parts := regexp.MustCompile(`[—\-]`).Split(cleanWord, -1)
			translatedParts := []string{"", "", "", ""}
			for _, part := range parts {
				if part == "" {
					continue
				}
				if trans, ok := wordMap[part]; ok {
					translatedParts[0] += trans["uz"] + " — "
					translatedParts[1] += trans["ru"] + " — "
					translatedParts[2] += trans["en"] + " — "
					translatedParts[3] += trans["oz"] + " — "
				} else {
					translatedParts[0] += part + " — "
					translatedParts[1] += part + " — "
					translatedParts[2] += part + " — "
					translatedParts[3] += part + " — "
				}
			}
			// Убираем последний " — "
			for i := range translatedParts {
				translatedParts[i] = strings.TrimSuffix(translatedParts[i], " — ")
			}
			uzWords = append(uzWords, translatedParts[0])
			ruWords = append(ruWords, translatedParts[1])
			enWords = append(enWords, translatedParts[2])
			ozWords = append(ozWords, translatedParts[3])
			continue
		}

		// Проверяем, есть ли перевод
		found := false
		if trans, ok := wordMap[cleanWord]; ok {
			// Определяем, нужно ли делать первую букву заглавной
			needsCapitalize := false
			if len(originalWord) > 0 {
				firstRune := []rune(originalWord)[0]
				// Проверяем, является ли первая буква заглавной (для латиницы и кириллицы)
				firstStr := string(firstRune)
				needsCapitalize = firstStr == strings.ToUpper(firstStr) && firstStr != strings.ToLower(firstStr)
			}

			if needsCapitalize {
				uzWords = append(uzWords, capitalizeFirst(trans["uz"]))
				ruWords = append(ruWords, capitalizeFirst(trans["ru"]))
				enWords = append(enWords, capitalizeFirst(trans["en"]))
				ozWords = append(ozWords, capitalizeFirst(trans["oz"]))
			} else {
				uzWords = append(uzWords, trans["uz"])
				ruWords = append(ruWords, trans["ru"])
				enWords = append(enWords, trans["en"])
				ozWords = append(ozWords, trans["oz"])
			}
			found = true
		}

		if !found {
			// Если слова нет в словаре, оставляем как есть
			uzWords = append(uzWords, originalWord)
			ruWords = append(ruWords, originalWord)
			enWords = append(enWords, originalWord)
			ozWords = append(ozWords, originalWord)
		}
	}

	result := map[string]string{
		"uz": strings.Join(uzWords, " "),
		"ru": strings.Join(ruWords, " "),
		"en": strings.Join(enWords, " "),
		"oz": strings.Join(ozWords, " "),
	}

	// Если перевод не изменился (все слова не найдены), используем API переводчик
	// Но только если хотя бы одно слово было переведено, иначе используем API
	hasTranslation := false
	for _, word := range words {
		if word != "" {
			cleanWord := strings.ToLower(strings.Trim(word, ".,!?;:\"'()[]{}«»"))
			if _, ok := wordMap[cleanWord]; ok {
				hasTranslation = true
				break
			}
		}
	}

	if !hasTranslation && result["ru"] == text && result["en"] == text && mt.translationService != nil {
		// Переводим через API на русский
		if ruTrans, err := mt.translationService.Translate(text, "ru"); err == nil && ruTrans != "" && ruTrans != text {
			result["ru"] = ruTrans
		}

		// Переводим через API на английский
		if enTrans, err := mt.translationService.Translate(text, "en"); err == nil && enTrans != "" && enTrans != text {
			result["en"] = enTrans
		}
	}

	// Для oz всегда используем транслитерацию с русского перевода (если есть) или с оригинала
	if result["ru"] != text && result["ru"] != "" {
		// Если есть русский перевод, транслитерируем его
		result["oz"] = TransliterateRuToOz(result["ru"])
	} else if result["oz"] == text {
		// Если перевода нет, транслитерируем оригинал
		result["oz"] = TransliterateUzToOz(text)
	}

	return result
}

// capitalizeFirst - делает первую букву заглавной (работает с кириллицей)
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	// Для кириллицы используем правильную обработку
	firstRune := []rune(s)[0]
	rest := string([]rune(s)[1:])
	// Преобразуем первую руну в заглавную
	firstUpper := strings.ToUpper(string(firstRune))
	return firstUpper + rest
}

// replaceCaseInsensitive - заменяет подстроку без учета регистра
func replaceCaseInsensitive(text, old, new string) string {
	// Простая замена без учета регистра
	result := text
	textLower := strings.ToLower(text)
	oldLower := strings.ToLower(old)

	idx := strings.Index(textLower, oldLower)
	if idx != -1 {
		// Находим оригинальную подстроку с учетом регистра
		originalOld := text[idx : idx+len(old)]
		result = strings.ReplaceAll(text, originalOld, new)
	}

	return result
}
