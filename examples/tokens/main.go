package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fsandov/go-sdk/pkg/cache"
	"github.com/fsandov/go-sdk/pkg/tokens"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func main() {
	showBasicExample()
	showCachedExample()
}

func showBasicExample() {
	fmt.Println("=== Basic Token Example ===")

	cfg := tokens.ShortLivedTokenConfig{
		TokenConfig: tokens.TokenConfig{
			SecretKey:      "123",
			Issuer:         "test-issuer",
			AccessTokenExp: time.Hour * 1,
		},
		RefreshTokenExp: time.Hour * 24,
	}

	tokensService, err := tokens.NewService(&cfg)
	if err != nil {
		log.Fatalf("failed to create token service: %v", err)
	}

	userID := "123e4567-e89b-12d3-a456-426614174000"
	email := "user@example.com"
	customClaims := map[string]interface{}{
		"account_type": "premium",
		"team_id":      "team-123",
	}

	accessToken, refreshToken, refreshExp, err := tokensService.GenerateTokens(userID, email, customClaims)
	if err != nil {
		log.Fatalf("Failed to generate tokens: %v", err)
	}

	fmt.Printf("Access Token: %s\n\n", accessToken)
	fmt.Printf("Refresh Token: %s\n\n", refreshToken)
	fmt.Printf("Refresh Token Expires: %s\n\n", refreshExp.Format(time.RFC3339))

	validateAndPrintToken(tokensService, accessToken, "Access Token")
	validateAndPrintToken(tokensService, refreshToken, "Refresh Token")

	claims, err := tokensService.ValidateTokenAndGetClaims(accessToken)
	if err != nil {
		log.Fatalf("Failed to validate token: %v", err)
	}

	printJSON(claims)
	printClaim(claims, "sub", "User ID")
	printClaim(claims, "email", "Email")
	printClaim(claims, "roles", "Roles")
	printClaim(claims, "perms", "Permissions")
	printClaim(claims, "account_type", "Account Type")

	fmt.Printf("Is access token valid? %v\n", tokensService.IsTokenValid(accessToken))
	fmt.Printf("Is invalid token valid? %v\n", tokensService.IsTokenValid("invalid.token.here"))
}

func validateAndPrintToken(svc tokens.Service, token, tokenType string) {
	fmt.Printf("Validating %s... ", tokenType)
	if svc.IsTokenValid(token) {
		fmt.Println("Valid")
	} else {
		fmt.Println("Invalid")
	}
}

func printClaim(claims jwt.MapClaims, key, label string) {
	val, exists := claims[key]
	if !exists {
		fmt.Printf("%s: claim not found\n", label)
		return
	}
	fmt.Printf("%s: %+v\n", label, val)
}

func showCachedExample() {
	fmt.Println("\n=== Cached Token Example ===")

	cacheService := cache.NewMemoryCache()
	cacheMgr := tokens.NewCacheManager(cacheService)
	defer cacheService.Close()

	cfg := tokens.ShortLivedTokenConfig{
		TokenConfig: tokens.TokenConfig{
			SecretKey:      "cached-secret",
			Issuer:         "cached-issuer",
			AccessTokenExp: time.Minute * 5,
		},
		RefreshTokenExp: time.Hour * 1,
	}

	tokenSvc, err := tokens.NewService(&cfg, tokens.WithCache(cacheMgr))
	if err != nil {
		log.Fatalf("failed to create cached token service: %v", err)
	}

	userID := "cached-user-123"
	email := "cached@example.com"
	customClaims := map[string]interface{}{
		"role": "admin",
		"plan": "premium",
	}

	fmt.Println("\n🔑 Generating multiple tokens for the same user...")
	var tokensList []string
	for i := 0; i < 3; i++ {
		accessToken, expiresAt, err := tokenSvc.GenerateToken(
			userID,
			fmt.Sprintf("%s-%d@example.com", strings.TrimSuffix(email, "@example.com"), i+1),
			customClaims,
		)
		if err != nil {
			log.Fatalf("Failed to generate token %d: %v", i+1, err)
		}
		tokensList = append(tokensList, accessToken)
		_ = tokenSvc.AddTokenToCache(context.Background(), accessToken, userID, expiresAt)
		fmt.Printf("  Token %d: %s... (cached)\n", i+1, accessToken[:30])
	}

	var tokensList2 []string
	for i := 4; i < 7; i++ {
		accessToken, _, expiresAt, err := tokenSvc.GenerateTokens(
			userID,
			fmt.Sprintf("%s-%d@example.com", strings.TrimSuffix(email, "@example.com"), i+1),
			customClaims,
		)
		if err != nil {
			log.Fatalf("Failed to generate token %d: %v", i+1, err)
		}
		tokensList2 = append(tokensList2, accessToken)
		_ = tokenSvc.AddTokenToCache(context.Background(), accessToken, userID, expiresAt)
		fmt.Printf("  Token %d: %s... (cached)\n", i+1, accessToken[:30])
	}

	fmt.Println("\n🔍 Validating all tokens in cache:")
	for i, token := range tokensList {
		fmt.Printf("\nToken %d:", i+1)
		validateAndPrintCached(tokenSvc, token, fmt.Sprintf("Access Token %d", i+1))
	}

	fmt.Printf("\n🚫 Invalidating ALL tokens for user %s\n", userID)
	if err := tokenSvc.InvalidateAllUserTokens(context.Background(), userID); err != nil {
		log.Printf("Failed to invalidate tokens: %v", err)
	}

	fmt.Println("\n🔍 Verifying token invalidation:")
	for i, token := range tokensList {
		fmt.Printf("\nToken %d after invalidation:", i+1)
		validateAndPrintCached(tokenSvc, token, fmt.Sprintf("Access Token %d", i+1))
	}

	fmt.Println("\n🔄 Generating a new token after invalidation:")
	newToken, _, expiresAt, err := tokenSvc.GenerateTokens(userID, email, customClaims)
	if err != nil {
		log.Fatalf("Failed to generate new token: %v", err)
	}
	_ = tokenSvc.AddTokenToCache(context.Background(), newToken, userID, expiresAt)
	validateAndPrintCached(tokenSvc, newToken, "New Access Token")

	demoHTTPServer(tokenSvc, cacheMgr)
}

func validateAndPrintCached(svc tokens.Service, token, tokenType string) {
	fmt.Printf("\n🔍 Validating %s Token\n", tokenType)
	fmt.Println(strings.Repeat("-", 40))

	cached, err := svc.TokenExistsInCache(context.Background(), token)
	if err != nil {
		fmt.Printf("❌ Cache Check Failed: %v\n", err)
	} else {
		status := "✅ Present"
		if !cached {
			status = "❌ Not Found"
		}
		fmt.Printf("🔑 Cache Status: %s\n", status)
	}

	valid := svc.IsTokenValid(token)
	validationStatus := "✅ Valid"
	if !valid {
		validationStatus = "❌ Invalid"
	}

	claims, err := svc.ValidateTokenAndGetClaims(token)
	if err == nil && claims != nil {
		if exp, ok := claims["exp"]; ok {
			expTime := time.Unix(int64(exp.(float64)), 0)
			remaining := time.Until(expTime).Round(time.Second)
			fmt.Printf("⏱️  Expires In: %s (%s)\n", remaining, expTime.Format(time.RFC1123))
		}
		if userID, ok := claims["sub"]; ok {
			fmt.Printf("👤 User ID: %s\n", userID)
		}
	}

	fmt.Printf("\n🔍 Validation Result: %s\n", validationStatus)
	fmt.Println(strings.Repeat("=", 60))
}

func demoHTTPServer(svc tokens.Service, cacheMgr tokens.CacheManager) {
	fmt.Println("\n=== Starting HTTP Server on :8080 ===")
	fmt.Println("Try these endpoints:")
	fmt.Println("  GET  /auth/me")
	fmt.Println("  POST /auth/logout")

	r := gin.Default()

	r.GET("/auth/me", tokens.CachedAuthMiddleware(svc, cacheMgr), func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		email, _ := c.Get("email")

		c.JSON(http.StatusOK, gin.H{
			"user_id": userID,
			"email":   email,
			"message": "You are authenticated!"})
	})

	r.POST("/auth/logout", func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing Authorization header"})
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if tokenString == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token format"})
			return
		}

		_ = svc.RemoveTokenFromCache(c.Request.Context(), tokenString)
		c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
	})

	go func() {
		if err := r.Run(":8080"); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	testHTTPRequest(svc)
}

func testHTTPRequest(svc tokens.Service) {
	userID := "test-user"
	email := "test@example.com"
	customClaims := map[string]interface{}{
		"role": "tester",
	}

	token, _, expiresAt, err := svc.GenerateTokens(userID, email, customClaims)
	if err != nil {
		log.Printf("Failed to generate test token: %v", err)
		return
	}

	if err := svc.AddTokenToCache(context.Background(), token, userID, expiresAt); err != nil {
		log.Printf("Failed to cache test token: %v", err)
		return
	}

	fmt.Printf("\n🔐 Generated and cached test token for user %s\n", userID)

	req, _ := http.NewRequest("GET", "http://localhost:8080/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("Failed to decode response: %v", err)
		return
	}

	fmt.Printf("\n✅ Test Request Response (Status: %d):\n", resp.StatusCode)
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonBytes))

	fmt.Println("\n🔍 Verifying token after request:")
	validateAndPrintCached(svc, token, "Test Access Token")
}

func printJSON(v interface{}) {
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling to JSON: %v", err)
	}
	fmt.Println(string(jsonBytes))
}
