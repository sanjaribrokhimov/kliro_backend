package utils

import "strings"

type CurrencyNameLangData struct {
	Name string `json:"name"`
}

// CurrencyDisplayName returns localized names for common currency codes.
func CurrencyDisplayName(code string) map[string]string {
	up := strings.ToUpper(strings.TrimSpace(code))
	switch up {
	case "USD":
		return map[string]string{"uz": "AQSH dollari", "ru": "Доллар США", "en": "US Dollar", "oz": "АҚШ доллари"}
	case "EUR":
		return map[string]string{"uz": "Yevro", "ru": "Евро", "en": "Euro", "oz": "Евро"}
	case "RUB":
		return map[string]string{"uz": "Rubl", "ru": "Рубль", "en": "Ruble", "oz": "Рубль"}
	case "GBP":
		return map[string]string{"uz": "Funt sterling", "ru": "Фунт стерлингов", "en": "Pound sterling", "oz": "Фунт стерлинг"}
	case "KZT":
		return map[string]string{"uz": "Tenge", "ru": "Тенге", "en": "Tenge", "oz": "Тенге"}
	case "UZS", "SUM":
		return map[string]string{"uz": "So'm", "ru": "Сум", "en": "UZS", "oz": "Сўм"}
	default:
		return map[string]string{"uz": up, "ru": up, "en": up, "oz": up}
	}
}

