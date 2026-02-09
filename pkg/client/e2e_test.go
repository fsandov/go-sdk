package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

func TestRateLimitMiddleware_E2E(t *testing.T) {
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	limiter := rate.NewLimiter(rate.Every(time.Second), 2)
	c := NewClient(
		WithBaseURL(srv.URL),
		WithDefaultSettings(&EndpointSettings{
			Timeout:    5 * time.Second,
			MaxRetries: 0,
			Headers:    map[string]string{},
		}),
		WithRateLimit(&RateLimitConfig{
			LimiterFor: func(method, path string) *rate.Limiter {
				return limiter
			},
		}),
	)

	ctx := context.Background()

	for i := 0; i < 2; i++ {
		_, err := c.Get(ctx, "/ok", nil)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}
	}

	ctxShort, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	_, err := c.Get(ctxShort, "/ok", nil)
	if err == nil {
		t.Error("expected rate limit to cause context deadline error on 3rd request in quick succession")
	}
}

func TestCircuitBreakerMiddleware_E2E(t *testing.T) {
	var requestCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requestCount, 1)
		if n <= 5 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "test-breaker",
		MaxRequests: 1,
		Interval:    10 * time.Second,
		Timeout:     1 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	})

	c := NewClient(
		WithBaseURL(srv.URL),
		WithDefaultSettings(&EndpointSettings{
			Timeout:    5 * time.Second,
			MaxRetries: 0,
			Headers:    map[string]string{},
		}),
		WithCircuitBreaker(&CircuitBreakerConfig{
			BreakerFor: func(method, path string) *gobreaker.CircuitBreaker {
				return breaker
			},
		}),
	)

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, _ = c.Get(ctx, "/test", nil)
	}

	if breaker.State() != gobreaker.StateOpen {
		t.Errorf("expected breaker to be Open after 3 failures, got %v", breaker.State())
	}

	_, err := c.Get(ctx, "/test", nil)
	if err == nil {
		t.Error("expected error when breaker is open")
	}

	time.Sleep(1500 * time.Millisecond)

	if breaker.State() != gobreaker.StateHalfOpen {
		t.Errorf("expected breaker to be HalfOpen after timeout, got %v", breaker.State())
	}

	atomic.StoreInt32(&requestCount, 10)
	resp, err := c.Get(ctx, "/test", nil)
	if err != nil {
		t.Fatalf("expected successful request in half-open, got error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	if breaker.State() != gobreaker.StateClosed {
		t.Errorf("expected breaker to be Closed after success, got %v", breaker.State())
	}
}

func TestCircuitBreakerMiddleware_NilConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(
		WithBaseURL(srv.URL),
		WithDefaultSettings(&EndpointSettings{
			Timeout:    5 * time.Second,
			MaxRetries: 0,
			Headers:    map[string]string{},
		}),
		WithCircuitBreaker(&CircuitBreakerConfig{
			BreakerFor: func(method, path string) *gobreaker.CircuitBreaker {
				return nil
			},
		}),
	)

	resp, err := c.Get(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
