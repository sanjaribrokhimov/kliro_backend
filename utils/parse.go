package utils

import (
	"regexp"
	"strconv"
	"strings"
)

var reNum = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)`)

func ParseFloatSafe(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func ParseIntSafe(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.Atoi(s)
	return v
}

func ParseInt64Safe(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func ExtractFirstFloat(s string) float64 {
	m := reNum.FindStringSubmatch(strings.ReplaceAll(s, ",", "."))
	if len(m) > 1 {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v
	}
	return 0
}

func ExtractMinMonths(s string) int {
	lower := strings.ToLower(s)
	nums := reNum.FindAllString(lower, -1)
	if len(nums) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(nums[0], 64)
	months := int(v)
	if strings.Contains(lower, "yil") {
		months = int(v*12.0 + 0.5)
	}
	return months
}

func ExtractMaxAmount(s string) int64 {
	clean := strings.ReplaceAll(s, "\u00a0", " ")
	clean = strings.ReplaceAll(clean, " ", "")
	nums := reNum.FindAllString(clean, -1)
	var max int64
	for _, t := range nums {
		t = strings.SplitN(t, ".", 2)[0]
		v, err := strconv.ParseInt(t, 10, 64)
		if err == nil && v > max {
			max = v
		}
	}
	return max
}

// ExtractMinAmount возвращает минимальную сумму из строки (первая найденная цифра)
// Пример: "100 000dan - 5 000 000 000 so'mgacha" -> 100000
// Пример: "500 000 so'mdan" -> 500000
func ExtractMinAmount(s string) int64 {
	clean := strings.ReplaceAll(s, "\u00a0", " ")
	clean = strings.ReplaceAll(clean, " ", "")
	nums := reNum.FindAllString(clean, -1)
	if len(nums) == 0 {
		return 0
	}
	first := strings.SplitN(nums[0], ".", 2)[0]
	v, err := strconv.ParseInt(first, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// DetectCurrencyFromAmount определяет валюту по тексту суммы: uzs/usd/eur/rub/unknown
func DetectCurrencyFromAmount(s string) string {
	lower := strings.ToLower(s)
	if strings.Contains(lower, "aqsh") || strings.Contains(lower, "usd") || strings.Contains(lower, "dollar") {
		return "usd"
	}
	if strings.Contains(lower, "yevro") || strings.Contains(lower, "eur") || strings.Contains(lower, "euro") {
		return "eur"
	}
	if strings.Contains(lower, "rubl") || strings.Contains(lower, "rub") {
		return "rub"
	}
	if strings.Contains(lower, "so'm") || strings.Contains(lower, "som") || strings.Contains(lower, "sum") || strings.Contains(lower, "uzs") {
		return "uzs"
	}
	return "unknown"
}
