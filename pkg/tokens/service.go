package tokens

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Config struct {
	SecretKey       string
	Issuer          string
	AccessTokenExp  time.Duration
	RefreshTokenExp time.Duration
}

func DefaultConfig() *Config {
	return &Config{
		SecretKey:       os.Getenv("TOKEN_SECRET_KEY"),
		Issuer:          os.Getenv("TOKEN_ISSUER"),
		AccessTokenExp:  15 * time.Minute,
		RefreshTokenExp: 30 * 24 * time.Hour,
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
	GenerateToken(userID, email string, customClaims map[string]interface{}) (string, error)
	ValidateTokenAndGetClaims(tokenString string) (jwt.MapClaims, error)
	IsTokenValid(tokenString string) bool
	GetClaim(claims jwt.MapClaims, key string) (interface{}, error)
}

type jwtService struct {
	cfg           *Config
	signingMethod jwt.SigningMethod
}

func NewService(cfg *Config) (Service, error) {
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
	return &jwtService{
		cfg:           cfg,
		signingMethod: jwt.SigningMethodHS256,
	}, nil
}

func (s *jwtService) GenerateTokens(userID, email string, customClaims map[string]interface{}) (string, string, time.Time, error) {
	now := time.Now().UTC()
	accessClaims := baseClaims(s.cfg.Issuer, userID, email, customClaims)
	accessClaims["exp"] = now.Add(s.cfg.AccessTokenExp).Unix()

	refreshExp := now.Add(s.cfg.RefreshTokenExp)
	refreshClaims := baseClaims(s.cfg.Issuer, userID, "", nil)
	refreshClaims["exp"] = refreshExp.Unix()

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

func (s *jwtService) GenerateToken(userID, email string, customClaims map[string]interface{}) (string, error) {
	now := time.Now().UTC()
	claims := baseClaims(s.cfg.Issuer, userID, email, customClaims)
	claims["exp"] = now.Add(s.cfg.AccessTokenExp).Unix()
	return s.signToken(claims)
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
	return token.SignedString([]byte(s.cfg.SecretKey))
}

func (s *jwtService) ValidateTokenAndGetClaims(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != s.signingMethod {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.SecretKey), nil
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
