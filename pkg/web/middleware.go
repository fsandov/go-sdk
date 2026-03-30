package web

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/fsandov/go-sdk/pkg/client"
	"github.com/fsandov/go-sdk/pkg/config"
	"github.com/fsandov/go-sdk/pkg/env"
	"github.com/fsandov/go-sdk/pkg/logs"
	"github.com/fsandov/go-sdk/pkg/paginate"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func (app *GinApp) setupMiddleware() {

	app.engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
	})

	app.engine.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	})

	if app.ginConfig.EnableTracing {
		app.engine.Use(otelgin.Middleware(config.Get().AppName))
	}

	if app.ginConfig.EnableRequestID {
		app.engine.Use(RequestIDMiddleware())
	}

	if app.ginConfig.EnableRecovery {
		app.engine.Use(gin.Recovery())
	}

	if app.ginConfig.EnableCORS {
		corsConfig := cors.DefaultConfig()
		if len(app.ginConfig.CORSOrigins) > 0 {
			corsConfig.AllowOrigins = app.ginConfig.CORSOrigins
		} else if env.IsProduction() {
			logs.Error(context.Background(), "CORS: no origins configured in production — allowing all origins is a security risk. Set CORSOrigins explicitly.", logs.WithNotifier())
			corsConfig.AllowAllOrigins = true
		} else if env.IsRemote() {
			app.logger.Warn(context.Background(), "CORS: no origins configured, allowing all origins in remote environment")
			corsConfig.AllowAllOrigins = true
		} else {
			corsConfig.AllowAllOrigins = true
		}
		corsConfig.AllowCredentials = !corsConfig.AllowAllOrigins
		corsConfig.AllowHeaders = append(corsConfig.AllowHeaders, "Authorization", "X-Request-ID")
		app.engine.Use(cors.New(corsConfig))
	}

	if app.ginConfig.EnableMetrics {
		app.engine.Use(httpServerMetricsMiddleware())
		app.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	if app.ginConfig.EnableCompression {
		app.engine.Use(gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/metrics"})))
	}

	if app.ginConfig.EnableGinPagination {
		app.engine.Use(paginate.GinPagination())
	}

	if app.ginConfig.EnableXAuthAppToken {
		app.engine.Use(XAuthAppTokenMiddleware())
	}

	app.engine.Use(SecureHeadersMiddleware())
	app.engine.Use(RealIPMiddleware())
	app.engine.Use(IPContextMiddleware())

}

func XAuthAppTokenMiddleware() gin.HandlerFunc {
	appToken := os.Getenv("X_AUTH_APP_TOKEN")
	return func(c *gin.Context) {
		if appToken == "" || subtle.ConstantTimeCompare([]byte(c.GetHeader("X-Auth-App-Token")), []byte(appToken)) != 1 {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		c.Set("request_id", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)

		ctx := context.WithValue(c.Request.Context(), client.RequestIDContextKey{}, requestID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func generateRequestID() string {
	return uuid.New().String()
}

var (
	httpServerRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_server_requests_total",
			Help: "Total number of HTTP requests handled by the server",
		},
		[]string{"method", "path", "status"},
	)
	httpServerRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path", "status"},
	)
	httpServerRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_server_requests_in_flight",
			Help: "Number of HTTP requests currently being processed",
		},
	)
	httpServerResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_response_size_bytes",
			Help:    "HTTP response size in bytes",
			Buckets: []float64{100, 1000, 5000, 10000, 50000, 100000, 500000, 1000000},
		},
		[]string{"method", "path", "status"},
	)
	httpServerPanicsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "http_server_panics_total",
			Help: "Total number of panics recovered by the server",
		},
	)
)

func init() {
	prometheus.MustRegister(
		httpServerRequestsTotal,
		httpServerRequestDuration,
		httpServerRequestsInFlight,
		httpServerResponseSize,
		httpServerPanicsTotal,
	)
}

func httpServerMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		httpServerRequestsInFlight.Inc()

		defer func() {
			httpServerRequestsInFlight.Dec()

			if r := recover(); r != nil {
				httpServerPanicsTotal.Inc()
				panic(r) // re-panic for gin.Recovery() to handle
			}
		}()

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}
		method := c.Request.Method
		duration := time.Since(start).Seconds()
		responseSize := float64(c.Writer.Size())

		httpServerRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpServerRequestDuration.WithLabelValues(method, path, status).Observe(duration)
		httpServerResponseSize.WithLabelValues(method, path, status).Observe(responseSize)
	}
}

func SecureHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Writer.Header().Set("X-XSS-Protection", "0")
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

func RealIPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := clientIP(c)
		c.Set("client_ip", ip)
		c.Next()
	}
}

func GetIPFromContext(c *gin.Context) string {
	if c == nil {
		return ""
	}

	if ip, exists := c.Get("client_ip"); exists {
		if ipStr, ok := ip.(string); ok {
			return ipStr
		}
	}

	return clientIP(c)
}

func clientIP(c *gin.Context) string {
	remoteAddr := c.Request.RemoteAddr

	if IsFromCloudflare(remoteAddr) {
		if cfIP := c.Request.Header.Get("CF-Connecting-IP"); cfIP != "" {
			return cfIP
		}
		if trueClientIP := c.Request.Header.Get("True-Client-IP"); trueClientIP != "" {
			return trueClientIP
		}
	}

	if xClientIP := c.Request.Header.Get("X-Client-IP"); xClientIP != "" {
		return xClientIP
	}

	if origClientIP := c.Request.Header.Get("X-Original-Client-IP"); origClientIP != "" {
		return origClientIP
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func GetIPHeadersFromContext(c *gin.Context) map[string]string {
	headers := make(map[string]string)

	headersToExtract := []string{
		"CF-Connecting-IP",
		"CF-IPCountry",
		"True-Client-IP",
		"X-Forwarded-For",
		"X-Forwarded-Proto",
		"X-Forwarded-Host",
	}

	for _, header := range headersToExtract {
		if value := c.Request.Header.Get(header); value != "" {
			headers[header] = value
		}
	}

	if clientIP := c.GetString("client_ip"); clientIP != "" {
		headers["X-Client-IP"] = clientIP
	}

	if ua := c.Request.Header.Get("X-Original-User-Agent"); ua != "" {
		headers["X-Original-User-Agent"] = ua
	} else if ua := c.Request.Header.Get("User-Agent"); ua != "" {
		headers["X-Original-User-Agent"] = ua
	}

	return headers
}

func IPContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := GetIPHeadersFromContext(c)

		enrichedCtx := context.WithValue(c.Request.Context(), client.IPHeadersContextKey, headers)
		c.Request = c.Request.WithContext(enrichedCtx)

		c.Next()
	}
}
