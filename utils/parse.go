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
