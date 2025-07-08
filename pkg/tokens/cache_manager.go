package tokens

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/fsandov/go-sdk/pkg/cache"
)

type CacheManager interface {
	AddToken(ctx context.Context, token, userID string, expiresAt time.Time) error
	RemoveToken(ctx context.Context, token string) error
	TokenExists(ctx context.Context, token string) (bool, error)
	InvalidateAllUserTokens(ctx context.Context, userID string) error
}

type cacheManager struct {
	cache cache.Cache
}

type tokenData struct {
	UserID string `json:"user_id"`
}

func (t *tokenData) UnmarshalJSON(data []byte) error {
	type Alias tokenData
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	return nil
}

func (t tokenData) MarshalJSON() ([]byte, error) {
	type Alias tokenData
	return json.Marshal(&struct {
		Alias
	}{
		Alias: (Alias)(t),
	})
}

func NewCacheManager(cache cache.Cache) CacheManager {
	return &cacheManager{
		cache: cache,
	}
}

func (cm *cacheManager) AddToken(ctx context.Context, token, userID string, expiresAt time.Time) error {
	if token == "" || userID == "" {
		return fmt.Errorf("token and userID cannot be empty")
	}

	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return fmt.Errorf("token already expired")
	}

	tokenKey := fmt.Sprintf("token:%s", token)
	tokenData := tokenData{
		UserID: userID,
	}

	tokenDataJSON, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("failed to marshal token data: %w", err)
	}

	err = cm.cache.Set(ctx, tokenKey, string(tokenDataJSON), ttl)
	if err != nil {
		return fmt.Errorf("failed to cache token: %w", err)
	}

	userTokensKey := fmt.Sprintf("user_tokens:%s", userID)
	err = cm.cache.ZAdd(ctx, userTokensKey, float64(expiresAt.Unix()), token)
	if err != nil {
		_ = cm.cache.Delete(ctx, tokenKey)
		return fmt.Errorf("failed to add token to user set: %w", err)
	}

	_, _ = cm.cache.Expire(ctx, userTokensKey, ttl+time.Hour*24)

	return nil
}

func (cm *cacheManager) RemoveToken(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}

	tokenKey := fmt.Sprintf("token:%s", token)

	dataStr, err := cm.cache.Get(ctx, tokenKey)
	if err != nil {
		if errors.Is(err, cache.ErrKeyNotFound) {
			return nil
		}
		return fmt.Errorf("failed to get token data: %w", err)
	}

	var data tokenData
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	userTokensKey := fmt.Sprintf("user_tokens:%s", data.UserID)
	if err := cm.cache.ZRem(ctx, userTokensKey, token); err != nil {
		log.Printf("warning: failed to remove token from user set: %v", err)
	}

	if err := cm.cache.Delete(ctx, tokenKey); err != nil && !errors.Is(err, cache.ErrKeyNotFound) {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	return nil
}

func (cm *cacheManager) TokenExists(ctx context.Context, token string) (bool, error) {
	if token == "" {
		return false, nil
	}

	tokenKey := fmt.Sprintf("token:%s", token)

	dataStr, err := cm.cache.Get(ctx, tokenKey)
	if err != nil {
		if errors.Is(err, cache.ErrKeyNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get token data: %w", err)
	}

	var data tokenData
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return false, fmt.Errorf("failed to unmarshal token data: %w", err)
	}

	return true, nil
}

func (cm *cacheManager) InvalidateAllUserTokens(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	userTokensKey := fmt.Sprintf("user_tokens:%s", userID)

	tokens, err := cm.cache.ZRange(ctx, userTokensKey, 0, -1)
	if err != nil {
		if errors.Is(err, cache.ErrKeyNotFound) {
			return nil
		}
		return fmt.Errorf("failed to get user tokens: %w", err)
	}

	for _, token := range tokens {
		tokenKey := fmt.Sprintf("token:%s", token)
		_ = cm.cache.Delete(ctx, tokenKey)
		_ = cm.cache.ZRem(ctx, userTokensKey, token)
	}

	return nil
}
