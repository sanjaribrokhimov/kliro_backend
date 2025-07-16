package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateOTP() string {
	var digits = []rune("0123456789")
	otp := make([]rune, 6)
	for i := range otp {
		b := make([]byte, 1)
		rand.Read(b)
		otp[i] = digits[int(b[0])%10]
	}
	return string(otp)
}

func GenerateSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
