# Changelog

## [1.6.0] - 2026-02-08

### Breaking Changes
- **`pkg/logs`**: Context keys for notifications are now typed (`logs.CtxKeyUserID`, `logs.CtxKeyRequestID`, `logs.CtxKeyTraceID`) instead of bare strings (`"user_id"`, `"request_id"`, `"trace_id"`)
- **`pkg/config`**: `Get()` no longer panics — it auto-initializes with defaults. Use `MustGet()` for strict panic behavior
- **`pkg/web`**: Default OTEL endpoint changed from `otel-collector:4317` (gRPC) to `otel-collector:4318` (HTTP) to match the `otlptracehttp` exporter
- **`pkg/client`**: `http.Client.Timeout` removed; timeout is now per-request via context deadline
- **`pkg/client`**: Circuit breaker middleware now returns error for 5xx responses (previously counted as success)

### Bug Fixes
- **`pkg/web/telemetry`**: Removed goroutines that auto-shutdown tracing/metrics providers after 5 seconds
- **`pkg/client`**: Fixed fallback panic caused by unsafe `error.(*Error)` type assertion
- **`pkg/client`**: Fixed response body leak on failed retry attempts (`resp.Body.Close()` now called)
- **`pkg/client/middlewares`**: `MetricsMiddleware` no longer panics on duplicate registration (uses `registerOrReuse`)
- **`pkg/client/middlewares`**: `MaxResponseSizeMiddleware` uses `io.LimitReader` instead of server-only `http.MaxBytesReader`
- **`pkg/cache/redis`**: Fixed TTL semantics — `-1` now returns `(0, nil)` (key without expiry) instead of `ErrKeyNotFound`
- **`pkg/client/config`**: Fixed log calls using `fmt`-style format strings without `Sprintf`

### Features
- **`pkg/logs`**: Logger now supports key-value pairs — `logs.Info(ctx, "msg", "key", value)` auto-converts to `zap.Any`
- **`pkg/config`**: Added `MustGet()` for callers that need strict panic-on-uninitialized behavior
- **`pkg/client`**: Added convenience options: `WithRateLimit`, `WithCircuitBreaker`, `WithCache`, `WithMaxResponseSize`, `WithMetrics`, `WithTracing`
- **`pkg/web/middleware`**: Enhanced `SecureHeadersMiddleware` with HSTS, X-Frame-Options, Referrer-Policy, X-XSS-Protection

### Improvements
- Upgraded `otlptracehttp` from v1.19.0 to v1.36.0 (aligned with `otel` v1.36.0)
- Removed sensitive webhook URL from Discord notifier log output
- Replaced `context.TODO()` with `context.Background()` in runtime code

### Testing
- Added unit tests for `pkg/client`, `pkg/cache`, `pkg/logs`, `pkg/web`, `pkg/tokens`, `pkg/config`, `pkg/paginate`, `pkg/database`, `pkg/jobscheduler`
- Added E2E tests for rate limiter and circuit breaker middlewares with `httptest.Server`
- Added Redis integration tests using `testcontainers-go` (run with `-tags=integration`)
- Added middleware tests with `httptest` for security headers, recovery, request ID, real IP

### Infrastructure
- Added GitHub Actions CI (`.github/workflows/ci.yml`) with unit tests, integration tests, and golangci-lint
- Added `Makefile` with `test`, `test-integration`, `lint`, `fmt`, `build` targets
- Added `.golangci.yml` configuration

## [1.5.7] - 24-09-2025
- Enhance logger context handling for notifications

## [1.5.6] - 20-09-2025
- Fix environment variable key formatting for Discord webhook initialization

## [1.5.5] - 20-09-2025
- Add context logging for notifier initialization process

## [1.5.4] - 20-09-2025
- Refactor logging in application and logger modules to enhance clarity and reduce verbosity

## [1.5.3] - 12-09-2025
- Refactor IP propagation middleware to remove excessive debug logging

## [1.5.2] - 12-09-2025
- Refactor IP propagation middleware to comment out unused header fields

## [1.5.1] - 12-09-2025
- Enhance IP propagation middleware with additional debug logging for request and response headers

## [1.5.0] - 12-09-2025
- Enhance IP propagation middleware with detailed logging and context enrichment

## [1.4.3] - 10-09-2025
- Enhance client IP retrieval with detailed logging for better debugging

## [1.4.2] - 08-09-2025
- Enhance MySQL DSN construction to support dynamic query parameters for improved configuration

## [1.4.1] - 07-09-2025
- Enhance Redis client initialization to support URL parsing and improve configuration handling

## [1.4.0] - 26-07-2025
- Refactor authentication middleware to improve token validation, error handling and email ctx

## [1.3.0] - 20-07-2025
- Add context to request with authorization header in middleware

## [1.2.1] - 09-07-2025
- Refactor token generation to return expiration time and update service interface

## [1.2.0] - 07-07-2025
- Implement token caching with cache manager and enhance middleware for cached token validation

## [1.1.0] - 07-07-2025
- Add sorted set operations to memory and Redis cache implementations

## [1.0.3] - 05-07-2025
- Enhance client IP retrieval in middleware to support CF-Connecting-IP header

## [1.0.2] - 19-06-2025
- Add refresh token type validation in authentication middleware

## [1.0.1] - 19-06-2025
- Reorder application initialization to set up middleware before routes

## [1.0.0] - 18-06-2025
- Initial release with basic user management features.



