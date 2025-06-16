package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fsandov/go-sdk/pkg/cache"
	"github.com/fsandov/go-sdk/pkg/client"
	"github.com/fsandov/go-sdk/pkg/logs"

	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

func main() {
	logs.NewLogger()

	apiLimiter := rate.NewLimiter(10, 20)
	apiBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "default-api-breaker",
		MaxRequests: 5,
		Interval:    30 * time.Second,
		Timeout:     5 * time.Second,
	})

	memCache := cache.NewMemoryCache()

	c := client.NewClient(
		client.WithBaseURL("https://jsonplaceholder.typicode.com"),
		client.WithDefaultSettings(&client.EndpointSettings{
			Timeout:    15 * time.Second,
			MaxRetries: 2,
			Headers: map[string]string{
				"test": "Test",
			},
			RequireAuth: true,
		}),
		client.WithEndpointConfig(func(method, path string) *client.EndpointSettings {
			switch {
			case strings.HasPrefix(path, "/public"):
				return &client.EndpointSettings{
					Timeout:     5 * time.Second,
					MaxRetries:  1,
					RequireAuth: false, // Public
				}
			case strings.HasPrefix(path, "/private"), strings.HasPrefix(path, "/admin"):
				return &client.EndpointSettings{
					Timeout:     5 * time.Second,
					MaxRetries:  1,
					RequireAuth: true, // Private
				}
			case method == http.MethodGet && path == "/posts/1":
				return &client.EndpointSettings{
					Timeout:     1 * time.Millisecond, // Timeout
					MaxRetries:  0,
					RateLimiter: apiLimiter,
					Breaker:     apiBreaker,
					Headers:     map[string]string{"X-Custom-Posts": "timeout"},
					RequireAuth: true,
				}
			case method == http.MethodGet && path == "/posts/2":
				return &client.EndpointSettings{
					Timeout:     10 * time.Second,
					MaxRetries:  3,
					RateLimiter: apiLimiter,
					Breaker:     apiBreaker,
					Headers:     map[string]string{"X-Custom-Posts": "yes"},
					RequireAuth: true,
					EnableCache: true,
					CacheTTL:    3 * time.Second,
				}
			}
			return nil

		}),
		client.WithMiddleware(client.RequestIDMiddleware()),
		client.WithMiddleware(client.IPPropagationMiddleware()),
		client.WithMiddleware(client.AuthMiddleware()),
		client.WithMiddleware(client.RateLimitMiddleware(&client.RateLimitConfig{
			LimiterFor: func(method, path string) *rate.Limiter {
				if strings.HasPrefix(path, "/posts") {
					return apiLimiter
				}
				return nil
			},
		})),
		client.WithMiddleware(client.CircuitBreakerMiddleware(&client.CircuitBreakerConfig{
			BreakerFor: func(method, path string) *gobreaker.CircuitBreaker {
				if strings.HasPrefix(path, "/posts") {
					return apiBreaker
				}
				return nil
			},
		})),
		client.WithMiddleware(client.TracingMiddleware(nil)),
		client.WithMiddleware(client.MetricsMiddleware(&client.MetricsConfig{
			Namespace: "fsandov",
			Subsystem: "client",
		})),
		client.WithMiddleware(client.CacheMiddleware(&client.CacheConfig{
			Cache:       memCache,
			DefaultTTL:  1 * time.Minute,
			Methods:     []string{"GET"},
			StatusCodes: []int{200, 201},
		})),
		client.WithHooks(&client.HooksConfig{
			PreRequest: func(ctx context.Context, req *client.RequestInfo) {
				logs.Info(ctx, fmt.Sprintf(">> %s %s", req.Method, req.Path))
			},
			PostRequest: func(ctx context.Context, req *client.RequestInfo, status int) {
				logs.Info(ctx, fmt.Sprintf("<< %s %s [%d]", req.Method, req.Path, status))
			},
			OnError: func(ctx context.Context, req *client.RequestInfo, err *client.Error) {
				logs.Error(ctx, fmt.Sprintf("!! %s %s [%v]", req.Method, req.Path, err))
			},
		}),
	)

	ctx := context.WithValue(context.Background(), "Authorization", "test-1234")

	resp, err := c.Get(ctx, "/posts/1", nil)
	if err != nil {
		log.Printf("[TIMEOUT] error GET /posts/1: %v", err)
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Println("GET /posts/1 body:", string(body))
	}

	resp, err = c.Get(ctx, "/posts/2", nil)
	if err != nil {
		log.Fatalf("error GET /posts/2: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("GET /posts/2 body:", string(body)[:120], "...")

	resp, err = c.Get(ctx, "/posts/2", nil)
	if err != nil {
		log.Fatalf("error GET /posts/2: %v", err)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	fmt.Println("GET /posts/2 body:", string(body)[:120], "...")

	time.Sleep(6 * time.Second)

	resp, err = c.Get(ctx, "/posts/2", nil)
	if err != nil {
		log.Fatalf("error GET /posts/2: %v", err)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	fmt.Println("GET /posts/2 body:", string(body)[:120], "...")
}
