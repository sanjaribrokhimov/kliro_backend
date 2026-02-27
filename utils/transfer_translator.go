package utils

import (
	"html"
	"strings"
)

// TransferLangData - данные перевода для одного языка
type TransferLangData struct {
	AppName    string  `json:"app_name"`
	Commission string  `json:"commission"`
	Limit      *string `json:"limit"`
}

// TranslatedTransfer - структура перевода с переводами (каждый язык отдельным объектом)
type TranslatedTransfer struct {
	ID        uint              `json:"id"`
	Uz        TransferLangData  `json:"uz"`
	Ru        TransferLangData  `json:"ru"`
	En        TransferLangData  `json:"en"`
	Oz        TransferLangData  `json:"oz"`
	CreatedAt string            `json:"created_at"`
}

// TransferTranslator - утилита для перевода полей переводов
type TransferTranslator struct {
	translationService *TranslationService
}

var globalTransferTranslator *TransferTranslator

// GetTransferTranslator - возвращает экземпляр переводчика
func GetTransferTranslator() *TransferTranslator {
	if globalTransferTranslator == nil {
		globalTransferTranslator = &TransferTranslator{
			translationService: nil, // Будет инициализирован при первом использовании
		}
	}
	return globalTransferTranslator
}

// SetTranslationService устанавливает сервис переводов
func (tt *TransferTranslator) SetTranslationService(service *TranslationService) {
	tt.translationService = service
}

// TranslateTransfer - переводит перевод на 4 языка (каждый язык отдельным объектом)
func (tt *TransferTranslator) TranslateTransfer(appName, commission string, limitUZ *string) TranslatedTransfer {
	// Переводим каждое поле
	appNameTrans := tt.translateAppName(appName)
	commissionTrans := tt.translateCommission(commission)
	limitTrans := tt.translateLimit(limitUZ)

	// Получаем значения из указателей
	var appNameUz, appNameRu, appNameEn, appNameOz string
	if appNameTrans["uz"] != nil {
		appNameUz = *appNameTrans["uz"]
	}
	if appNameTrans["ru"] != nil {
		appNameRu = *appNameTrans["ru"]
	}
	if appNameTrans["en"] != nil {
		appNameEn = *appNameTrans["en"]
	}
	if appNameTrans["oz"] != nil {
		appNameOz = *appNameTrans["oz"]
	}

	return TranslatedTransfer{
		Uz: TransferLangData{
			AppName:    appNameUz,
			Commission: commissionTrans["uz"],
			Limit:      limitTrans["uz"],
		},
		Ru: TransferLangData{
			AppName:    appNameRu,
			Commission: commissionTrans["ru"],
			Limit:      limitTrans["ru"],
		},
		En: TransferLangData{
			AppName:    appNameEn,
			Commission: commissionTrans["en"],
			Limit:      limitTrans["en"],
		},
		Oz: TransferLangData{
			AppName:    appNameOz,
			Commission: commissionTrans["oz"],
			Limit:      limitTrans["oz"],
		},
	}
}

// translateAppName - названия приложений не переводим через API (даёт ошибки: A Pay -> "Кусок" и т.д.), оставляем оригинал для uz/ru/en, для oz — транслит.
func (tt *TransferTranslator) translateAppName(appName string) map[string]*string {
	if appName == "" {
		empty := ""
		return map[string]*string{"uz": &empty, "ru": &empty, "en": &empty, "oz": &empty}
	}
	uz := appName
	ru := appName
	en := appName
	oz := TransliterateUzToOz(appName)
	return map[string]*string{"uz": &uz, "ru": &ru, "en": &en, "oz": &oz}
}

// translateCommission - комиссия это число/процент (0.5%, 1%), не переводим — на всех языках одно значение.
func (tt *TransferTranslator) translateCommission(commission string) map[string]string {
	if commission == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}
	c := strings.TrimSpace(commission)
	return map[string]string{"uz": c, "ru": c, "en": c, "oz": c}
}

// isTranslationError - признак ответа API об ошибке/лимите (не подставлять как перевод).
func isTranslationError(s string) bool {
	return strings.Contains(s, "QUERY LENGTH LIMIT") ||
		strings.Contains(s, "500 CHARS") ||
		strings.Contains(s, "MAX ALLOWED QUERY")
}

// decodeHTMLEntities - заменяет &#39; на ', &#10; на \n и т.д.
func decodeHTMLEntities(s string) string {
	return html.UnescapeString(s)
}

// translateLimit - переводит лимит через API; при ошибке или лимите API — оригинал; в ответе декодируем HTML-сущности.
func (tt *TransferTranslator) translateLimit(limitUZ *string) map[string]*string {
	if limitUZ == nil || *limitUZ == "" {
		return map[string]*string{"uz": nil, "ru": nil, "en": nil, "oz": nil}
	}

	limit := strings.TrimSpace(*limitUZ)
	if limit == "" || limit == "\n" {
		return map[string]*string{"uz": nil, "ru": nil, "en": nil, "oz": nil}
	}

	result := map[string]*string{
		"uz": limitUZ,
		"ru": nil,
		"en": nil,
		"oz": nil,
	}

	if tt.translationService != nil {
		ruTrans, err := tt.translationService.Translate(limit, "ru")
		if err == nil && ruTrans != "" && !isTranslationError(ruTrans) {
			ruDec := decodeHTMLEntities(ruTrans)
			result["ru"] = &ruDec
		} else {
			result["ru"] = limitUZ
		}

		enTrans, err := tt.translationService.Translate(limit, "en")
		if err == nil && enTrans != "" && !isTranslationError(enTrans) {
			enDec := decodeHTMLEntities(enTrans)
			result["en"] = &enDec
		} else {
			result["en"] = limitUZ
		}
	} else {
		result["ru"] = limitUZ
		result["en"] = limitUZ
	}

	ozTrans := TransliterateUzToOz(limit)
	result["oz"] = &ozTrans

	return result
}
