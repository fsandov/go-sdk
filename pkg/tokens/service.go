package tokens

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenConfig is the base configuration for token generation
type TokenConfig struct {
	SecretKey      string
	Issuer         string
	AccessTokenExp time.Duration
}

// ShortLivedTokenConfig contains configuration for short-lived access tokens with refresh tokens
type ShortLivedTokenConfig struct {
	TokenConfig
	RefreshTokenExp time.Duration
}

// LongLivedTokenConfig contains configuration for long-lived access tokens without refresh tokens
type LongLivedTokenConfig struct {
	TokenConfig
}

// DefaultShortLivedConfig returns the default configuration for short-lived tokens
func DefaultShortLivedConfig() *ShortLivedTokenConfig {
	return &ShortLivedTokenConfig{
		TokenConfig: TokenConfig{
			SecretKey:      os.Getenv("TOKEN_SECRET_KEY"),
			Issuer:         os.Getenv("TOKEN_ISSUER"),
			AccessTokenExp: 15 * time.Minute,
		},
		RefreshTokenExp: 30 * 24 * time.Hour,
	}
}

// DefaultLongLivedConfig returns the default configuration for long-lived tokens
func DefaultLongLivedConfig() *LongLivedTokenConfig {
	return &LongLivedTokenConfig{
		TokenConfig: TokenConfig{
			SecretKey:      os.Getenv("TOKEN_SECRET_KEY"),
			Issuer:         os.Getenv("TOKEN_ISSUER"),
			AccessTokenExp: 30 * 24 * time.Hour,
		},
	}
}

var (
	ErrInvalidClaims = errors.New("invalid claims")
	ErrInvalidToken  = errors.New("invalid token")
	ErrNoSecret      = errors.New("secret key is required")
	ErrNoIssuer      = errors.New("issuer is required")
)

type Service interface {
	GenerateTokens(userID, email string, customClaims map[string]interface{}) (accessToken, refreshToken string, refreshTokenExpire time.Time, err error)
	GenerateToken(userID, email string, customClaims map[string]interface{}) (string, time.Time, error)
	ValidateTokenAndGetClaims(tokenString string) (jwt.MapClaims, error)
	IsTokenValid(tokenString string) bool
	GetClaim(claims jwt.MapClaims, key string) (interface{}, error)

	AddTokenToCache(ctx context.Context, token, userID string, expiresAt time.Time) error
	RemoveTokenFromCache(ctx context.Context, token string) error
	InvalidateAllUserTokens(ctx context.Context, userID string) error
	TokenExistsInCache(ctx context.Context, token string) (bool, error)
}

type jwtService struct {
	tokenCfg      interface{} // Can be either ShortLivedTokenConfig or LongLivedTokenConfig
	cacheMgr      CacheManager
	signingMethod jwt.SigningMethod
}

func (s *jwtService) getTokenConfig() TokenConfig {
	switch cfg := s.tokenCfg.(type) {
	case ShortLivedTokenConfig:
		return cfg.TokenConfig
	case LongLivedTokenConfig:
		return cfg.TokenConfig
	default:
		panic("invalid token configuration type")
	}
}

func (s *jwtService) isShortLived() bool {
	_, ok := s.tokenCfg.(ShortLivedTokenConfig)
	return ok
}

func (s *jwtService) getShortLivedConfig() ShortLivedTokenConfig {
	cfg, ok := s.tokenCfg.(ShortLivedTokenConfig)
	if !ok {
		panic("attempted to use short-lived token methods with long-lived configuration")
	}
	return cfg
}

// NewService creates a new token service with short-lived tokens configuration
func NewService(cfg *ShortLivedTokenConfig, opts ...ServiceOption) (Service, error) {
	if cfg == nil {
		return nil, errors.New("tokens: config is nil")
	}
	if cfg.SecretKey == "" {
		return nil, ErrNoSecret
	}
	if cfg.Issuer == "" {
		return nil, ErrNoIssuer
	}
	if cfg.AccessTokenExp == 0 {
		cfg.AccessTokenExp = 4 * time.Hour
	}
	if cfg.RefreshTokenExp == 0 {
		cfg.RefreshTokenExp = 24 * time.Hour
	}

	svc := &jwtService{
		tokenCfg:      *cfg,
		signingMethod: jwt.SigningMethodHS256,
	}

	for _, opt := range opts {
		opt(svc)
	}

	return svc, nil
}

type ServiceOption func(*jwtService)

// WithCache enables token caching using the provided cache manager
func WithCache(cacheMgr CacheManager) ServiceOption {
	return func(s *jwtService) {
		s.cacheMgr = cacheMgr
	}
}

// NewLongLivedService creates a new token service with long-lived tokens configuration
func NewLongLivedService(cfg *LongLivedTokenConfig, opts ...ServiceOption) (Service, error) {
	if cfg == nil {
		return nil, errors.New("tokens: config is nil")
	}
	if cfg.SecretKey == "" {
		return nil, ErrNoSecret
	}
	if cfg.Issuer == "" {
		return nil, ErrNoIssuer
	}
	if cfg.AccessTokenExp == 0 {
		cfg.AccessTokenExp = 30 * 24 * time.Hour
	}

	svc := &jwtService{
		tokenCfg:      *cfg,
		signingMethod: jwt.SigningMethodHS256,
	}

	for _, opt := range opts {
		opt(svc)
	}

	return svc, nil
}

func (s *jwtService) GenerateTokens(userID, email string, customClaims map[string]interface{}) (string, string, time.Time, error) {
	if !s.isShortLived() {
		return "", "", time.Time{}, errors.New("GenerateTokens can only be used with short-lived token configuration")
	}

	cfg := s.getShortLivedConfig()
	tokenCfg := s.getTokenConfig()
	now := time.Now().UTC()

	accessClaims := baseClaims(tokenCfg.Issuer, userID, email, customClaims)
	accessClaims["exp"] = now.Add(tokenCfg.AccessTokenExp).Unix()
	accessClaims["typ"] = "access"

	refreshExp := now.Add(cfg.RefreshTokenExp)
	refreshClaims := baseClaims(tokenCfg.Issuer, userID, "", nil)
	refreshClaims["exp"] = refreshExp.Unix()
	refreshClaims["typ"] = "refresh"

	accessToken, err := s.signToken(accessClaims)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}

	refreshToken, err := s.signToken(refreshClaims)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("sign refresh token: %w", err)
	}

	return accessToken, refreshToken, refreshExp, nil
}

func (s *jwtService) GenerateToken(userID, email string, customClaims map[string]interface{}) (string, time.Time, error) {
	tokenCfg := s.getTokenConfig()
	now := time.Now().UTC()

	tokenExp := now.Add(tokenCfg.AccessTokenExp)
	claims := baseClaims(tokenCfg.Issuer, userID, email, customClaims)
	claims["exp"] = tokenExp.Unix()
	claims["typ"] = "access"

	accessToken, err := s.signToken(claims)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}

	return accessToken, tokenExp, nil
}

func baseClaims(issuer, userID, email string, customClaims map[string]interface{}) jwt.MapClaims {
	now := time.Now().UTC().Unix()
	claims := jwt.MapClaims{
		"sub": userID,
		"iss": issuer,
		"iat": now,
		"nbf": now,
	}
	if email != "" {
		claims["email"] = email
	}
	for k, v := range customClaims {
		claims[k] = v
	}
	return claims
}

func (s *jwtService) signToken(claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(s.signingMethod, claims)
	tokenCfg := s.getTokenConfig()
	return token.SignedString([]byte(tokenCfg.SecretKey))
}

// AddTokenToCache adds a token to the cache and associates it with the user
func (s *jwtService) AddTokenToCache(ctx context.Context, token, userID string, expiresAt time.Time) error {
	if s.cacheMgr == nil {
		return nil
	}
	if expiresAt.Before(time.Now()) {
		return fmt.Errorf("token has already expired")
	}
	if userID == "" {
		return errors.New("user ID is required")
	}
	if expiresAt.IsZero() {
		return errors.New("expiration time is required")
	}
	return s.cacheMgr.AddToken(ctx, token, userID, expiresAt)
}

// RemoveTokenFromCache removes a specific token from the cache
// This is typically called during logout or when a token needs to be revoked
func (s *jwtService) RemoveTokenFromCache(ctx context.Context, token string) error {
	if s.cacheMgr == nil {
		return nil
	}
	return s.cacheMgr.RemoveToken(ctx, token)
}

func (s *jwtService) InvalidateAllUserTokens(ctx context.Context, userID string) error {
	if s.cacheMgr == nil {
		return errors.New("cache manager not configured")
	}
	if userID == "" {
		return errors.New("user ID is required")
	}
	return s.cacheMgr.InvalidateAllUserTokens(ctx, userID)
}

// TokenExistsInCache checks if a token exists and is valid in the cache
// Returns true only if the token exists and is not expired
func (s *jwtService) TokenExistsInCache(ctx context.Context, token string) (bool, error) {
	if s.cacheMgr == nil {
		return false, nil
	}
	return s.cacheMgr.TokenExists(ctx, token)
}

func (s *jwtService) ValidateTokenAndGetClaims(tokenString string) (jwt.MapClaims, error) {
	tokenCfg := s.getTokenConfig()
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != s.signingMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(tokenCfg.SecretKey), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidClaims
	}
	return claims, nil
}

func (s *jwtService) IsTokenValid(tokenString string) bool {
	_, err := s.ValidateTokenAndGetClaims(tokenString)
	return err == nil
}

func (s *jwtService) GetClaim(claims jwt.MapClaims, key string) (interface{}, error) {
	val, ok := claims[key]
	if !ok {
		return nil, ErrInvalidClaims
	}
	return val, nil
}

func GetStringClaim(claims jwt.MapClaims, key string) (string, error) {
	val, ok := claims[key]
	if !ok {
		return "", ErrInvalidClaims
	}
	str, ok := val.(string)
	if !ok {
		return "", ErrInvalidClaims
	}
	return str, nil
}

func GetStringSliceClaim(claims jwt.MapClaims, key string) ([]string, error) {
	val, ok := claims[key]
	if !ok {
		return nil, ErrInvalidClaims
	}
	switch v := val.(type) {
	case []string:
		return v, nil
	case []interface{}:
		result := make([]string, len(v))
		for i, x := range v {
			s, ok := x.(string)
			if !ok {
				return nil, ErrInvalidClaims
			}
			result[i] = s
		}
		return result, nil
	default:
		return nil, ErrInvalidClaims
	}
}
