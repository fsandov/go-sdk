package client

import (
	"context"
	"net/http"
	"time"

	"github.com/fsandov/go-sdk/pkg/logs"
	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

type EndpointSettings struct {
	Timeout         time.Duration
	MaxRetries      int
	ShouldRetry     func(resp *http.Response, err error) bool
	BackoffStrategy func(attempt int) time.Duration
	Headers         map[string]string
	RequireAuth     bool
	RateLimiter     *rate.Limiter
	// Deprecated: use WithCircuitBreaker middleware instead. This field is not used by Do().
	Breaker *gobreaker.CircuitBreaker
	AuthTokenFn     func(*RequestInfo) (string, error)
	EnableCache     bool
	CacheTTL        time.Duration
	Fallback        func(*http.Request, error) (*http.Response, error)
	MaxResponseSize int64
}

func applyDefaults(cfg *EndpointSettings) *EndpointSettings {
	if cfg == nil {
		cfg = &EndpointSettings{}
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 2
	}
	if cfg.Headers == nil {
		cfg.Headers = map[string]string{}
	}
	return cfg
}

func ValidateEndpointConfig(settings *EndpointSettings, path string) {
	if settings == nil {
		logs.Error(context.Background(), "endpoint config not defined", "path", path)
		return
	}
	if settings.Timeout == 0 {
		logs.Info(context.Background(), "endpoint timeout not set, using default 10s", "path", path)
		settings.Timeout = 10 * time.Second
	}
}

type RequestInfo struct {
	Method string
	Path   string
}

type EndpointConfig func(method, path string) *EndpointSettings

type HooksConfig struct {
	PreRequest  func(ctx context.Context, req *RequestInfo)
	PostRequest func(ctx context.Context, req *RequestInfo, status int)
	OnError     func(ctx context.Context, req *RequestInfo, err *Error)
}
