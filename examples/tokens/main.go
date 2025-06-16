package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/fsandov/go-sdk/pkg/tokens"
	"github.com/golang-jwt/jwt/v5"
)

func main() {
	cfg := tokens.Config{
		SecretKey:       "123",
		Issuer:          "123",
		AccessTokenExp:  time.Hour * 1,
		RefreshTokenExp: time.Hour * 24 * 30,
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

func printJSON(v interface{}) {
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling to JSON: %v", err)
	}
	fmt.Println(string(jsonBytes))
}
