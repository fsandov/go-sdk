# go-sdk

Shared Go SDK for microservices. Provides config, logging, HTTP client, cache, tokens, web (Gin), and observability.

## Install

```bash
go get github.com/fsandov/go-sdk
```

## Packages

### `pkg/config` — App Configuration

```go
import "github.com/fsandov/go-sdk/pkg/config"

config.Init(&config.AppConfig{
    AppName:     "my-service",
    Environment: "production",
    Port:        "8080",
})

cfg := config.Get()
// cfg.AppName == "my-service"

// Safe extras
cfg.Extras["feature_flag"] = "true"
v := cfg.ExtraString("feature_flag", "false")
```

`Get()` auto-initializes with defaults if `Init` was never called.
`MustGet()` panics if not initialized (for strict boot sequences).

### `pkg/logs` — Structured Logging

```go
import "github.com/fsandov/go-sdk/pkg/logs"

logs.NewLogger()

// zap.Field style
logs.Info(ctx, "request processed", zap.String("id", "abc"))

// key-value pairs (auto-converted to zap fields)
logs.Error(ctx, "operation failed", "error", err, "retries", 3)

// mixed
logs.Warn(ctx, "slow query", zap.Duration("elapsed", d), "table", "users")

// with notification
logs.Error(ctx, "critical failure", zap.Error(err), logs.WithNotifier())
```

Auto-init Discord notifiers from env vars:
```go
// Set DISCORD_WEBHOOK_ERROR, DISCORD_WEBHOOK_WARN, DISCORD_WEBHOOK_INFO
logs.AutoInitNotifiers()
```

### `pkg/web` — Gin Application

```go
import "github.com/fsandov/go-sdk/pkg/web"

app := web.New(web.DefaultGinConfig())

engine := app.GetEngine()
engine.GET("/api/v1/items", handler)

app.Run() // graceful shutdown built-in
```

Features enabled by config flags: CORS, gzip, pprof, metrics, request IDs, pagination, tracing, secure headers (HSTS, X-Frame-Options, Referrer-Policy, etc.).

### `pkg/client` — HTTP Client

```go
import "github.com/fsandov/go-sdk/pkg/client"

c := client.NewClient(
    client.WithBaseURL("https://api.example.com"),
    client.WithDefaultSettings(&client.EndpointSettings{
        Timeout:    10 * time.Second,
        MaxRetries: 3,
    }),
    client.WithMetrics(&client.MetricsConfig{Namespace: "my_svc"}),
    client.WithTracing(nil), // uses global OTEL provider
    client.WithMaxResponseSize(1 << 20), // 1MB limit
)
defer c.Close()

resp, err := c.Get(ctx, "/users/123", nil)
```

Per-endpoint configuration:
```go
c := client.NewClient(
    client.WithBaseURL("https://api.example.com"),
    client.WithEndpointConfig(func(method, path string) *client.EndpointSettings {
        if path == "/slow" {
            return &client.EndpointSettings{Timeout: 60 * time.Second}
        }
        return nil // uses defaults
    }),
)
```

### `pkg/cache` — Cache (Redis + Memory)

```go
import "github.com/fsandov/go-sdk/pkg/cache"

// Memory (for dev/testing)
c := cache.NewMemoryCache()

// Redis
c, err := cache.NewRedisCacheFromConfig(cache.RedisConfig{
    Enabled: true,
    Addr:    "localhost:6379",
})

_ = c.Set(ctx, "key", "value", 5*time.Minute)
val, err := c.Get(ctx, "key")

ttl, err := c.TTL(ctx, "key")
// err == cache.ErrKeyNotFound  → key doesn't exist
// ttl == 0, err == nil         → key exists, no expiry
// ttl > 0, err == nil          → key exists with TTL
```

### `pkg/tokens` — JWT Token Service

```go
import "github.com/fsandov/go-sdk/pkg/tokens"

// Short-lived (access + refresh)
svc, _ := tokens.NewService(&tokens.ShortLivedTokenConfig{
    TokenConfig: tokens.TokenConfig{
        SecretKey:      os.Getenv("TOKEN_SECRET_KEY"),
        Issuer:         "my-app",
        AccessTokenExp: 15 * time.Minute,
    },
    RefreshTokenExp: 30 * 24 * time.Hour,
}, tokens.WithCache(tokens.NewCacheManager(redisCache)))

access, refresh, refreshExp, _ := svc.GenerateTokens("user-id", "email@test.com", nil)

// Gin middleware
router.Use(tokens.AuthMiddleware(svc))
// or with cache validation
router.Use(tokens.CachedAuthMiddleware(svc, cacheMgr))
```

## Development

### Makefile

```bash
make test              # unit tests with -race
make test-integration  # integration tests (requires Docker)
make lint              # golangci-lint
make fmt               # gofmt
make build             # compile all packages
```

### Integration Tests

Redis integration tests use [testcontainers-go](https://golang.testcontainers.org/) to spin up a real Redis container. Requires Docker.

```bash
go test -tags=integration -race ./pkg/cache/...
```

### CI

GitHub Actions runs on every push/PR to `main`:
1. `go test -race ./...` (unit tests)
2. `golangci-lint run`
3. `go test -tags=integration -race ./...` (integration tests)

### Migration

See [MIGRATION.md](MIGRATION.md) for breaking changes and migration steps from v1.5.x to v1.6.0.