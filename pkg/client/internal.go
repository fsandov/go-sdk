package client

import (
	"os"
	"time"
)

// NewInternalClient creates a standard HTTP client for inter-service communication
// within the backend. Uses BACKEND_URL env var with smart defaults:
// develop URL in non-production, production URL otherwise.
func NewInternalClient(appName string) *Client {
	baseURL := os.Getenv("BACKEND_URL")

	defaultSettings := &EndpointSettings{
		Timeout:    10 * time.Second,
		MaxRetries: 3,
		BackoffStrategy: func(attempt int) time.Duration {
			d := 200 * time.Millisecond * time.Duration(1<<uint(attempt))
			if d > 2*time.Second {
				d = 2 * time.Second
			}
			return d
		},
		RequireAuth: true,
	}

	return NewClient(
		WithBaseURL(baseURL),
		WithDefaultSettings(defaultSettings),
		WithMiddleware(RequestIDMiddleware()),
		WithMiddleware(IPPropagationMiddleware()),
		WithMiddleware(UserContextMiddleware()),
		WithMiddleware(AuthMiddleware()),
		WithTracing(DefaultTracingConfig()),
		WithMetrics(&MetricsConfig{
			Namespace: os.Getenv("METRICS_NAMESPACE"),
			Subsystem: appName,
		}),
		WithMaxResponseSize(2*1024*1024),
		WithMiddleware(AppTokenMiddleware()),
	)
}
