package middleware

import (
	"net/http"
	"os"
	"strings"

	"kliro/utils"

	"github.com/gin-gonic/gin"
)

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid Authorization header"})
			c.Abort()
			return
		}
		token := strings.TrimPrefix(header, "Bearer ")
		claims, err := utils.ParseJWT(token, os.Getenv("JWT_SECRET"))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}
		userID, ok := claims["user_id"].(float64)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token payload"})
			c.Abort()
			return
		}
		c.Set("user_id", int(userID))
		c.Next()
	}
}
