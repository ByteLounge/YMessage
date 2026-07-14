package auth

import (
	"net/http"
	"strings"

	"ymessage/internal/crypto"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware intercepts requests to check for valid JWT access token
func AuthMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header must be Bearer token"})
			c.Abort()
			return
		}

		claims, err := crypto.ValidateAccessToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired access token"})
			c.Abort()
			return
		}

		// Save claims to context
		c.Set("userID", claims.UserID)
		c.Set("deviceID", claims.DeviceID)
		c.Set("username", claims.Username)

		c.Next()
	})
}
