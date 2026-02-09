# Migration Guide — v1.5.x → v1.6.0

## Version Decision

**Version: v1.6.0** (minor bump)
---

## Breaking Changes & Migration Steps

### 1. `pkg/logs` — Typed Context Keys

**Before:**
```go
ctx = context.WithValue(ctx, "user_id", userID)
ctx = context.WithValue(ctx, "request_id", reqID)
ctx = context.WithValue(ctx, "trace_id", traceID)
```

**After:**
```go
import "github.com/fsandov/go-sdk/pkg/logs"

ctx = context.WithValue(ctx, logs.CtxKeyUserID, userID)
ctx = context.WithValue(ctx, logs.CtxKeyRequestID, reqID)
ctx = context.WithValue(ctx, logs.CtxKeyTraceID, traceID)
```

**Impact**: Only affects code that sets these context values for log notifications. If you don't use `logs.WithNotifier()`, no change needed.

---

### 2. `pkg/config` — `Get()` No Longer Panics

**Before:**
```go
cfg := config.Get() // panics if Init() not called
```

**After:**
```go
cfg := config.Get()     // auto-initializes with defaults if Init() not called
cfg := config.MustGet() // panics if Init() not called (old behavior)
```

**Migration**: If your boot sequence depends on the panic to catch missing `Init()`, switch to `MustGet()`. Otherwise, no change needed — `Get()` is now safe by default.

---

### 3. `pkg/web` — OTEL Endpoint Default

**Before:** `otel-collector:4317` (gRPC port)
**After:** `otel-collector:4318` (HTTP port)

**Migration**: If you're passing `OTELEndpoint` explicitly in `GinConfig`, no change needed. If relying on the default, ensure your OTEL collector listens on port 4318 for HTTP, or set the endpoint explicitly:

```go
config := web.DefaultGinConfig()
config.OTELEndpoint = "otel-collector:4317" // keep gRPC if needed
```

---

### 4. `pkg/client` — Timeout Strategy

**Before:** `http.Client.Timeout` was set globally on the client.
**After:** Timeout is per-request via `context.WithTimeout`, derived from `EndpointSettings.Timeout`.

**Migration**: No change needed for callers — the `Do()` method already applied context timeouts. The global `http.Client.Timeout` was redundant and has been removed.

---

### 5. `pkg/client` — Circuit Breaker 5xx Handling

**Before:** 5xx responses were treated as success by gobreaker (no error returned from Execute callback).
**After:** 5xx responses return `fmt.Errorf("server error: %d", statusCode)` so gobreaker counts them as failures.

**Migration**: If you relied on the circuit breaker *not* tripping on 5xx errors, this is a behavior change. The new behavior is the correct one — 5xx should trip the breaker.

---

### 6. `pkg/client` — New Convenience Options

No migration needed. These are additive:

```go
client.WithRateLimit(&client.RateLimitConfig{...})
client.WithCircuitBreaker(&client.CircuitBreakerConfig{...})
client.WithCache(&client.CacheConfig{...})
client.WithMaxResponseSize(1 << 20)
client.WithMetrics(&client.MetricsConfig{...})
client.WithTracing(nil)
```

---

### 7. `pkg/web/middleware` — Enhanced Security Headers

The following headers are now set by default on all responses:

| Header | Value |
|--------|-------|
| `X-Content-Type-Options` | `nosniff` |
| `X-Frame-Options` | `DENY` |
| `Referrer-Policy` | `strict-origin-when-cross-origin` |
| `X-XSS-Protection` | `1; mode=block` |
| `Strict-Transport-Security` | `max-age=63072000; includeSubDomains` |

**Migration**: If your application needs different values (e.g., `X-Frame-Options: SAMEORIGIN` for embedding), override them in a custom middleware after `SecureHeadersMiddleware`.

---

## New Features (no migration needed)

- **Logger key-value pairs**: `logs.Info(ctx, "msg", "key", value, "key2", value2)`
- **Redis TTL contract**: `-2` → `ErrKeyNotFound`, `-1` → `(0, nil)` (no expiry), `>=0` → duration
- **Integration tests**: `go test -tags=integration ./...` (requires Docker for Redis)
- **CI**: GitHub Actions runs unit tests + integration tests + golangci-lint
- **Makefile**: `make test`, `make test-integration`, `make lint`, `make fmt`
