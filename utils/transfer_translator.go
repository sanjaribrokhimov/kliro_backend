package utils

import (
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

// translateAppName - переводит название приложения через API
func (tt *TransferTranslator) translateAppName(appName string) map[string]*string {
	if appName == "" {
		empty := ""
		return map[string]*string{"uz": &empty, "ru": &empty, "en": &empty, "oz": &empty}
	}

	result := map[string]*string{
		"uz": &appName,
		"ru": nil,
		"en": nil,
		"oz": nil,
	}

	// Переводим через API для ru и en
	if tt.translationService != nil {
		ruTrans, err := tt.translationService.Translate(appName, "ru")
		if err == nil && ruTrans != "" {
			result["ru"] = &ruTrans
		} else {
			result["ru"] = &appName // Fallback
		}

		enTrans, err := tt.translationService.Translate(appName, "en")
		if err == nil && enTrans != "" {
			result["en"] = &enTrans
		} else {
			result["en"] = &appName // Fallback
		}
	} else {
		// Если сервис не инициализирован, используем оригинал
		result["ru"] = &appName
		result["en"] = &appName
	}

	// Для oz используем транслитерацию
	ozTrans := TransliterateUzToOz(appName)
	result["oz"] = &ozTrans

	return result
}

// translateCommission - переводит комиссию через API
func (tt *TransferTranslator) translateCommission(commission string) map[string]string {
	if commission == "" {
		return map[string]string{"uz": "", "ru": "", "en": "", "oz": ""}
	}

	result := map[string]string{
		"uz": commission,
		"ru": commission,
		"en": commission,
		"oz": commission,
	}

	// Переводим через API для ru и en
	if tt.translationService != nil {
		ruTrans, err := tt.translationService.Translate(commission, "ru")
		if err == nil && ruTrans != "" {
			result["ru"] = ruTrans
		}

		enTrans, err := tt.translationService.Translate(commission, "en")
		if err == nil && enTrans != "" {
			result["en"] = enTrans
		}
	}

	// Для oz используем транслитерацию
	result["oz"] = TransliterateUzToOz(commission)

	return result
}

// translateLimit - переводит лимит через API
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

	// Переводим через API для ru и en
	if tt.translationService != nil {
		ruTrans, err := tt.translationService.Translate(limit, "ru")
		if err == nil && ruTrans != "" {
			result["ru"] = &ruTrans
		} else {
			// Fallback - используем оригинал
			result["ru"] = limitUZ
		}

		enTrans, err := tt.translationService.Translate(limit, "en")
		if err == nil && enTrans != "" {
			result["en"] = &enTrans
		} else {
			// Fallback - используем оригинал
			result["en"] = limitUZ
		}
	} else {
		// Если сервис не инициализирован, используем оригинал
		result["ru"] = limitUZ
		result["en"] = limitUZ
	}

	// Для oz используем транслитерацию
	ozTrans := TransliterateUzToOz(limit)
	result["oz"] = &ozTrans

	return result
}
