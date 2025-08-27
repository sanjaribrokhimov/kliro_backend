package utils

import (
	"fmt"
	"strings"
)

// FormatUZS formats a float64 number to a string like "12 250 so'm".
// If the fractional part is zero, decimals are omitted; otherwise, up to 2 decimals are kept.
func FormatUZS(value float64) string {
	// Format with two decimals first
	s := fmt.Sprintf("%.2f", value)
	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}

	// Remove trailing .00
	if fracPart == "00" {
		fracPart = ""
	}

	// Insert spaces every 3 digits in integer part
	var b strings.Builder
	cnt := 0
	for i := len(intPart) - 1; i >= 0; i-- {
		b.WriteByte(intPart[i])
		cnt++
		if cnt%3 == 0 && i != 0 {
			b.WriteByte(' ')
		}
	}
	// Reverse the string
	runes := []rune(b.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	intWithSpaces := string(runes)

	if fracPart != "" {
		return intWithSpaces + "." + fracPart + " so'm"
	}
	return intWithSpaces + " so'm"
}
