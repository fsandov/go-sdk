package tokens

import (
	"context"
	"net/http"
	"strings"

	"github.com/fsandov/go-sdk/pkg/logs"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	KeyUserID = "user_id"
	KeyClaims = "claims"
	KeyEmail  = "email"

	bearerPrefix    = "Bearer "
	accessTokenType = "access"
)

// tokenValidationResult holds the result of token validation
type tokenValidationResult struct {
	tokenString string
	claims      jwt.MapClaims
	authHeader  string
}

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
		result, ok := validateTokenFromHeader(c, svc)
		if !ok {
			return
		}

		if !validateTokenType(c, result.claims) {
			return
		}

		exists, err := cacheMgr.TokenExists(c.Request.Context(), result.tokenString)
		if err != nil {
			logs.Warn(c.Request.Context(), "[CachedAuthMiddleware] error checking token in cache", "error", err)
			// Continue execution even if cache check fails (graceful degradation)
		} else if !exists {
			logs.Info(c.Request.Context(), "[CachedAuthMiddleware] token not found in cache or revoked")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked or expired"})
			c.Abort()
			return
		}

		setUserContext(c, result.claims, result.authHeader)
	}
}

// AuthMiddleware creates a new Gin middleware that validates JWT tokens without caching.
// This is the original implementation that validates the token on every request.
// For better performance, consider using CachedAuthMiddleware instead.
func AuthMiddleware(tokenSvc Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, ok := validateTokenFromHeader(c, tokenSvc)
		if !ok {
			return
		}

		if !validateTokenType(c, result.claims) {
			return
		}

		setUserContext(c, result.claims, result.authHeader)
	}
}

// validateTokenFromHeader extracts and validates the JWT token from the Authorization header
func validateTokenFromHeader(c *gin.Context, svc Service) (*tokenValidationResult, bool) {
	authHeader := c.GetHeader("Authorization")

	if len(authHeader) <= len(bearerPrefix) || !strings.HasPrefix(authHeader, bearerPrefix) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed Authorization header"})
		c.Abort()
		return nil, false
	}

	tokenString := strings.TrimSpace(authHeader[len(bearerPrefix):])
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token is empty"})
		c.Abort()
		return nil, false
	}

	claims, err := svc.ValidateTokenAndGetClaims(tokenString)
	if err != nil {
		logs.Info(c.Request.Context(), "[TokenValidation] token validation failed", "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		c.Abort()
		return nil, false
	}

	return &tokenValidationResult{
		tokenString: tokenString,
		claims:      claims,
		authHeader:  authHeader,
	}, true
}

func validateTokenType(c *gin.Context, claims jwt.MapClaims) bool {
	typ, _ := GetStringClaim(claims, "typ")
	if typ != accessTokenType {
		logs.Info(c.Request.Context(), "[TokenValidation] invalid token type", "type", typ)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token type"})
		c.Abort()
		return false
	}
	return true
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

	if email, _ := GetStringClaim(claims, "email"); email != "" {
		c.Set(KeyEmail, email)
	}

	ctx := context.WithValue(c.Request.Context(), "Authorization", authHeader)
	c.Request = c.Request.WithContext(ctx)

	c.Next()
}
