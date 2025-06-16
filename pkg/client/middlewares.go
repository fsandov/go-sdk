package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fsandov/go-sdk/pkg/cache"
	"github.com/fsandov/go-sdk/pkg/logs"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"
)

type Middleware func(next http.RoundTripper) http.RoundTripper

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func RequestIDMiddleware() Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("X-Request-ID") == "" {
				req.Header.Set("X-Request-ID", uuid.New().String())
			}
			return next.RoundTrip(req)
		})
	}
}

func IPPropagationMiddleware() Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			remoteAddr := req.RemoteAddr
			if idx := strings.LastIndex(remoteAddr, ":"); idx != -1 {
				remoteAddr = remoteAddr[:idx]
			}
			fwdFor := req.Header.Get("X-Forwarded-For")
			if fwdFor != "" && !strings.Contains(fwdFor, remoteAddr) {
				req.Header.Set("X-Forwarded-For", fwdFor+", "+remoteAddr)
			} else if fwdFor == "" {
				req.Header.Set("X-Forwarded-For", remoteAddr)
			}
			return next.RoundTrip(req)
		})
	}
}

func AppTokenMiddleware() Middleware {
	appToken := os.Getenv("X_AUTH_APP_TOKEN")
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if appToken != "" {
				req.Header.Set("X-Auth-App-Token", appToken)
			}
			return next.RoundTrip(req)
		})
	}
}

const CtxKeyIncomingAuth = "Authorization"

func AuthMiddleware() Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			cfgAny := req.Context().Value(EndpointConfigKey{})
			cfg, _ := cfgAny.(*EndpointSettings)
			if cfg != nil && cfg.RequireAuth {
				if v := req.Context().Value(CtxKeyIncomingAuth); v != nil {
					if token, ok := v.(string); ok && token != "" {
						req.Header.Set("Authorization", token)
					}
				}
			}
			return next.RoundTrip(req)
		})
	}
}

type RateLimitConfig struct {
	LimiterFor func(method, path string) *rate.Limiter
}

func RateLimitMiddleware(cfg *RateLimitConfig) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if cfg != nil && cfg.LimiterFor != nil {
				limiter := cfg.LimiterFor(req.Method, req.URL.Path)
				if limiter != nil {
					if err := limiter.Wait(req.Context()); err != nil {
						return nil, err
					}
				}
			}
			return next.RoundTrip(req)
		})
	}
}

type CircuitBreakerConfig struct {
	BreakerFor func(method, path string) *gobreaker.CircuitBreaker
}

func CircuitBreakerMiddleware(cfg *CircuitBreakerConfig) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if cfg != nil && cfg.BreakerFor != nil {
				breaker := cfg.BreakerFor(req.Method, req.URL.Path)
				if breaker != nil {
					var resp *http.Response
					_, err := breaker.Execute(func() (interface{}, error) {
						var err error
						resp, err = next.RoundTrip(req)
						if err != nil || (resp != nil && resp.StatusCode >= 500) {
							return nil, err
						}
						return resp, nil
					})
					return resp, err
				}
			}
			return next.RoundTrip(req)
		})
	}
}

type TracingConfig struct {
	TracerProvider    trace.TracerProvider
	Propagators       propagation.TextMapPropagator
	SpanNameFormatter func(r *http.Request) string
}

func DefaultTracingConfig() *TracingConfig {
	return &TracingConfig{
		TracerProvider: otel.GetTracerProvider(),
		Propagators:    otel.GetTextMapPropagator(),
		SpanNameFormatter: func(r *http.Request) string {
			return fmt.Sprintf("HTTP %s", r.Method)
		},
	}
}

func TracingMiddleware(config *TracingConfig) Middleware {
	if config == nil {
		config = DefaultTracingConfig()
	}
	if config.TracerProvider == nil {
		config.TracerProvider = otel.GetTracerProvider()
	}
	if config.Propagators == nil {
		config.Propagators = otel.GetTextMapPropagator()
	}
	if config.SpanNameFormatter == nil {
		config.SpanNameFormatter = DefaultTracingConfig().SpanNameFormatter
	}
	return func(next http.RoundTripper) http.RoundTripper {
		return &tracingTransport{
			next:   next,
			config: config,
			tracer: config.TracerProvider.Tracer("github.com/fsandov/go-sdk/pkg/client"),
		}
	}
}

type tracingTransport struct {
	next   http.RoundTripper
	config *TracingConfig
	tracer trace.Tracer
}

func (t *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	spanName := t.config.SpanNameFormatter(req)
	ctx, span := t.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	t.config.Propagators.Inject(ctx, propagation.HeaderCarrier(req.Header))
	span.SetAttributes(
		attribute.String("http.method", req.Method),
		attribute.String("http.url", req.URL.String()),
		attribute.String("http.target", req.URL.Path),
		attribute.String("http.scheme", req.URL.Scheme),
		attribute.String("http.host", req.Host),
	)
	if req.ContentLength > 0 {
		span.SetAttributes(attribute.Int("http.request_content_length", int(req.ContentLength)))
	}
	resp, err := t.next.RoundTrip(req.WithContext(ctx))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
	if resp.ContentLength > 0 {
		span.SetAttributes(attribute.Int("http.response_content_length", int(resp.ContentLength)))
	}
	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, http.StatusText(resp.StatusCode))
	}
	return resp, nil
}

type MetricsConfig struct {
	Namespace string
	Subsystem string
}

func MetricsMiddleware(config *MetricsConfig) Middleware {
	if config == nil {
		return nil
	}
	if config.Namespace == "" {
		config.Namespace = "http_client"
	}
	var (
		requestDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "request_duration_seconds",
				Help:      "Time spent processing HTTP requests",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "host", "path", "status"},
		)
		requestsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "host", "path", "status"},
		)
		requestErrors = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: config.Namespace,
				Subsystem: config.Subsystem,
				Name:      "request_errors_total",
				Help:      "Total number of HTTP request errors",
			},
			[]string{"method", "host", "path", "error"},
		)
	)
	prometheus.MustRegister(requestDuration, requestsTotal, requestErrors)
	return func(next http.RoundTripper) http.RoundTripper {
		return &metricsTransport{
			next:            next,
			requestDuration: requestDuration,
			requestsTotal:   requestsTotal,
			requestErrors:   requestErrors,
		}
	}
}

type metricsTransport struct {
	next            http.RoundTripper
	requestDuration *prometheus.HistogramVec
	requestsTotal   *prometheus.CounterVec
	requestErrors   *prometheus.CounterVec
}

func (t *metricsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	method := req.Method
	host := req.URL.Host
	path := req.URL.Path
	resp, err := t.next.RoundTrip(req)
	duration := time.Since(start).Seconds()
	if err != nil {
		t.requestErrors.WithLabelValues(method, host, path, err.Error()).Inc()
		return nil, err
	}
	status := fmt.Sprintf("%d", resp.StatusCode)
	t.requestDuration.WithLabelValues(method, host, path, status).Observe(duration)
	t.requestsTotal.WithLabelValues(method, host, path, status).Inc()
	return resp, nil
}

type CacheConfig struct {
	Cache           cache.Cache
	DefaultTTL      time.Duration
	Methods         []string
	StatusCodes     []int
	KeyFunc         func(r *http.Request) string
	SkipCacheHeader string
}

func CacheMiddleware(config *CacheConfig) Middleware {
	if config == nil || config.Cache == nil {
		return nil
	}
	if config.KeyFunc == nil {
		config.KeyFunc = defaultCacheKey
	}
	return func(next http.RoundTripper) http.RoundTripper {
		return &cacheTransport{next: next, config: config}
	}
}

type cacheEntry struct {
	Status     string      `json:"status"`
	StatusCode int         `json:"status_code"`
	Header     http.Header `json:"header"`
	Body       string      `json:"body"`
}
type cacheTransport struct {
	next   http.RoundTripper
	config *CacheConfig
}

func (t *cacheTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cfgAny := req.Context().Value(EndpointConfigKey{})
	cfg, _ := cfgAny.(*EndpointSettings)
	enableCache := cfg != nil && cfg.EnableCache
	ttl := t.config.DefaultTTL
	if cfg != nil && cfg.CacheTTL > 0 {
		ttl = cfg.CacheTTL
	}
	if enableCache && t.config.Cache == nil {
		logs.Warn(req.Context(), fmt.Sprintf("¡Warning! EnableCache is true but no cache backend is configured %s %s", req.Method, req.URL.Path))
		return t.next.RoundTrip(req)
	}

	if !enableCache {
		return t.next.RoundTrip(req)
	}
	if req.Header.Get(t.config.SkipCacheHeader) == "true" {
		req.Header.Del(t.config.SkipCacheHeader)
		return t.next.RoundTrip(req)
	}
	methodCacheable := false
	for _, m := range t.config.Methods {
		if req.Method == m {
			methodCacheable = true
			break
		}
	}
	if !methodCacheable {
		return t.next.RoundTrip(req)
	}
	key := t.config.KeyFunc(req)
	if cached, err := t.config.Cache.Get(req.Context(), key); err == nil {
		var entry cacheEntry
		if err := json.Unmarshal([]byte(cached), &entry); err == nil {
			resp := &http.Response{
				Status:        entry.Status,
				StatusCode:    entry.StatusCode,
				Header:        entry.Header.Clone(),
				Body:          io.NopCloser(strings.NewReader(entry.Body)),
				ContentLength: int64(len(entry.Body)),
			}
			return resp, nil
		}
	}
	resp, err := t.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	body, err := ReadAndRestoreBody(resp)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	statusCacheable := false
	for _, code := range t.config.StatusCodes {
		if resp.StatusCode == code {
			statusCacheable = true
			break
		}
	}
	if statusCacheable {
		headers := resp.Header.Clone()
		headers.Del("X-Request-Id")
		entry := cacheEntry{
			Status:     resp.Status,
			StatusCode: resp.StatusCode,
			Header:     headers,
			Body:       string(body),
		}
		if cachedData, err := json.Marshal(entry); err == nil {
			_ = t.config.Cache.Set(req.Context(), key, string(cachedData), ttl)
		}
	}
	return resp, nil
}

func defaultCacheKey(r *http.Request) string {
	return r.Method + ":" + r.URL.String()
}

type HooksMiddlewareConfig struct {
	PreRequest  func(req *http.Request)
	PostRequest func(req *http.Request, resp *http.Response)
	OnError     func(req *http.Request, err error)
}

func HooksMiddleware(cfg *HooksMiddlewareConfig) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if cfg != nil && cfg.PreRequest != nil {
				cfg.PreRequest(req)
			}
			resp, err := next.RoundTrip(req)
			if err != nil && cfg != nil && cfg.OnError != nil {
				cfg.OnError(req, err)
			}
			if resp != nil && cfg != nil && cfg.PostRequest != nil {
				cfg.PostRequest(req, resp)
			}
			return resp, err
		})
	}
}

func ReadAndRestoreBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func MaxResponseSizeMiddleware(maxSize int64) Middleware {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			resp, err := next.RoundTrip(req)
			if err != nil || maxSize <= 0 {
				return resp, err
			}
			resp.Body = http.MaxBytesReader(nil, resp.Body, maxSize)
			return resp, err
		})
	}
}
