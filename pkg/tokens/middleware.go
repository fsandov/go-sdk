package tokens

import (
	"net/http"
	"strings"

	"github.com/fsandov/go-sdk/pkg/logs"
	"github.com/gin-gonic/gin"
)

const (
	KeyUserID = "user_id"
	KeyClaims = "claims"
)

func AuthMiddleware(tokenSvc Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		const bearer = "Bearer "

		if len(authHeader) <= len(bearer) || !strings.HasPrefix(authHeader, bearer) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed Authorization header"})
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(authHeader[len(bearer):])
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token is empty"})
			c.Abort()
			return
		}

		claims, err := tokenSvc.ValidateTokenAndGetClaims(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		typ, _ := GetStringClaim(claims, "typ")
		if typ != "access" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token type"})
			c.Abort()
			return
		}

		userID, _ := GetStringClaim(claims, "sub")
		if userID == "" {
			logs.Warn(c.Request.Context(), "[AuthMiddleware] missing 'sub' in claims")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token: no subject"})
			c.Abort()
			return
		}

		c.Set("Authorization", authHeader)
		c.Set(KeyUserID, userID)
		c.Set(KeyClaims, claims)
		c.Next()
	}
}
