package tokens

import (
	"log"
	"net/http"
	"strings"

	"github.com/fsandov/go-sdk/pkg/logs"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	KeyUserID = "user_id"
	KeyClaims = "claims"
)

// CachedAuthMiddleware creates a new Gin middleware that validates tokens using a cache.
// It checks if the token exists in the cache before performing any validation.
// The cache should be populated by another process (e.g., during token creation/refresh).
//
// Parameters:
//   - tokenSvc: The token service used to validate tokens
//   - cacheMgr: The cache manager used to check token existence
//
// Returns:
// CachedAuthMiddleware is a middleware that checks if the token is valid and exists in cache
func CachedAuthMiddleware(svc Service, cacheMgr CacheManager) gin.HandlerFunc {
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

		claims, err := svc.ValidateTokenAndGetClaims(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		exists, err := cacheMgr.TokenExists(c.Request.Context(), tokenString)
		if err != nil {
			log.Printf("Warning: error checking token in cache: %v", err)
		} else if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked or expired"})
			c.Abort()
			return
		}

		setUserContext(c, claims, authHeader)
	}
}

// AuthMiddleware creates a new Gin middleware that validates JWT tokens without caching.
// This is the original implementation that validates the token on every request.
// For better performance, consider using CachedAuthMiddleware instead.
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

		setUserContext(c, claims, authHeader)
	}
}

// setUserContext sets the user-related values in the Gin context.
// It extracts the user ID from the claims and sets the Authorization header, user ID, and claims in the context.
func setUserContext(c *gin.Context, claims jwt.MapClaims, authHeader string) {
	userID, _ := GetStringClaim(claims, "sub")
	if userID == "" {
		logs.Warn(c.Request.Context(), "[AuthMiddleware] missing 'sub' in claims")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token: no subject"})
		c.Abort()
		return
	}

	if typ, ok := claims["typ"].(string); ok {
		c.Set("token_type", typ)
	}

	c.Set("Authorization", authHeader)
	c.Set(KeyUserID, userID)
	c.Set(KeyClaims, claims)
	c.Next()
}
