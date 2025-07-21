package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func ParserJWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if !strings.HasPrefix(token, "Bearer ") || strings.TrimPrefix(token, "Bearer ") != "my-secret-jwt-token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}
