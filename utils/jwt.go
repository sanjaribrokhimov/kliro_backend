package utils

import (
	"time"

	"github.com/dgrijalva/jwt-go"
)

func GenerateJWT(userID uint, role, secret string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"role":    role,
		"exp":     time.Now().Add(time.Hour * 72).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseJWT(tokenStr, secret string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if token != nil && token.Valid {
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			return claims, nil
		}
	}
	return nil, err
}

func GenerateRefreshToken(userID uint, secret string) (string, int64, error) {
	expiry := time.Now().Add(30 * 24 * time.Hour).Unix() // 30 дней
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     expiry,
		"type":    "refresh",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	return tokenStr, expiry, err
}
