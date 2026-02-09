package tokens

import (
	"context"
	"testing"
	"time"

	"github.com/fsandov/go-sdk/pkg/cache"
)

func newTestService(t *testing.T) Service {
	t.Helper()
	svc, err := NewService(&ShortLivedTokenConfig{
		TokenConfig: TokenConfig{
			SecretKey:      "test-secret-key-minimum-length",
			Issuer:         "test-issuer",
			AccessTokenExp: 1 * time.Hour,
		},
		RefreshTokenExp: 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	return svc
}

func TestGenerateAndValidateToken(t *testing.T) {
	svc := newTestService(t)

	token, _, err := svc.GenerateToken("user123", "user@test.com", nil)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := svc.ValidateTokenAndGetClaims(token)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	typ, err := GetStringClaim(claims, "typ")
	if err != nil || typ != "access" {
		t.Errorf("expected typ=access, got %q (err=%v)", typ, err)
	}

	sub, err := GetStringClaim(claims, "sub")
	if err != nil || sub != "user123" {
		t.Errorf("expected sub=user123, got %q (err=%v)", sub, err)
	}

	email, err := GetStringClaim(claims, "email")
	if err != nil || email != "user@test.com" {
		t.Errorf("expected email=user@test.com, got %q (err=%v)", email, err)
	}
}

func TestTokenExpiration(t *testing.T) {
	svc, err := NewService(&ShortLivedTokenConfig{
		TokenConfig: TokenConfig{
			SecretKey:      "test-secret-key-minimum-length",
			Issuer:         "test-issuer",
			AccessTokenExp: 1 * time.Millisecond,
		},
		RefreshTokenExp: 1 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	token, _, err := svc.GenerateToken("user123", "user@test.com", nil)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = svc.ValidateTokenAndGetClaims(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestGenerateTokensShortLived(t *testing.T) {
	svc := newTestService(t)

	accessToken, refreshToken, _, err := svc.GenerateTokens("user123", "user@test.com", nil)
	if err != nil {
		t.Fatalf("failed to generate tokens: %v", err)
	}

	if accessToken == "" || refreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}

	accessClaims, err := svc.ValidateTokenAndGetClaims(accessToken)
	if err != nil {
		t.Fatalf("failed to validate access token: %v", err)
	}
	if typ, _ := GetStringClaim(accessClaims, "typ"); typ != "access" {
		t.Errorf("expected access token type, got %q", typ)
	}

	refreshClaims, err := svc.ValidateTokenAndGetClaims(refreshToken)
	if err != nil {
		t.Fatalf("failed to validate refresh token: %v", err)
	}
	if typ, _ := GetStringClaim(refreshClaims, "typ"); typ != "refresh" {
		t.Errorf("expected refresh token type, got %q", typ)
	}
}

func TestCacheManagerOperations(t *testing.T) {
	c := cache.NewMemoryCache()
	defer c.Close()
	cm := NewCacheManager(c)
	ctx := context.Background()

	err := cm.AddToken(ctx, "token1", "user1", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("AddToken failed: %v", err)
	}

	exists, err := cm.TokenExists(ctx, "token1")
	if err != nil || !exists {
		t.Fatalf("expected token to exist, exists=%v err=%v", exists, err)
	}

	err = cm.RemoveToken(ctx, "token1")
	if err != nil {
		t.Fatalf("RemoveToken failed: %v", err)
	}

	exists, err = cm.TokenExists(ctx, "token1")
	if err != nil || exists {
		t.Fatalf("expected token to not exist after removal, exists=%v err=%v", exists, err)
	}
}

func TestInvalidateAllUserTokens(t *testing.T) {
	c := cache.NewMemoryCache()
	defer c.Close()
	cm := NewCacheManager(c)
	ctx := context.Background()

	_ = cm.AddToken(ctx, "t1", "user1", time.Now().Add(time.Hour))
	_ = cm.AddToken(ctx, "t2", "user1", time.Now().Add(time.Hour))

	err := cm.InvalidateAllUserTokens(ctx, "user1")
	if err != nil {
		t.Fatalf("InvalidateAllUserTokens failed: %v", err)
	}

	for _, tok := range []string{"t1", "t2"} {
		exists, _ := cm.TokenExists(ctx, tok)
		if exists {
			t.Errorf("expected token %s to be invalidated", tok)
		}
	}
}
