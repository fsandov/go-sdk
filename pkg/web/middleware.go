package web

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/fsandov/go-sdk/pkg/client"
	"github.com/fsandov/go-sdk/pkg/paginate"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/penglongli/gin-metrics/ginmetrics"
)

func (app *GinApp) setupMiddleware() {

	app.engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
	})

	app.engine.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	})

	if app.ginConfig.EnableRequestID {
		app.engine.Use(RequestIDMiddleware())
	}

	if app.ginConfig.EnableRecovery {
		app.engine.Use(gin.Recovery())
	}

	if app.ginConfig.EnableCORS {
		app.engine.Use(cors.Default())
	}

	if app.ginConfig.EnableCompression {
		app.engine.Use(gzip.Gzip(gzip.DefaultCompression))
	}

	if app.ginConfig.EnableMetrics {
		m := ginmetrics.GetMonitor()
		m.SetMetricPath("/metrics")
		m.SetSlowTime(10)
		m.SetDuration([]float64{0.1, 0.3, 1.2, 5, 10})
		m.Use(app.engine)
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
		if c.GetHeader("X-Auth-App-Token") != appToken {
			c.AbortWithStatusJSON(401, gin.H{"error": "Unauthorized"})
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
		c.Next()
	}
}

func generateRequestID() string {
	return uuid.New().String()
}

func SecureHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		c.Next()
	}
}

func RealIPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := clientIP(c)
		c.Set("client_ip", ip)
		c.Writer.Header().Set("X-Client-IP", ip)

		if originalIP := c.Request.Header.Get("X-Original-Client-Ip"); originalIP != "" {
			c.Set("original_client_ip", originalIP)
			c.Writer.Header().Set("X-Original-Client-Ip", originalIP)
		} else {
			c.Set("original_client_ip", ip)
			c.Writer.Header().Set("X-Original-Client-Ip", ip)
		}

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
	xOriginalClientIP := c.Request.Header.Get("X-Original-Client-Ip")
	cfIP := c.Request.Header.Get("CF-Connecting-IP")
	fwdFor := c.Request.Header.Get("X-Forwarded-For")
	realIP := c.Request.Header.Get("X-Real-Ip")
	xClientIP := c.Request.Header.Get("X-Client-IP")

	var selectedIP string

	if xOriginalClientIP != "" {
		selectedIP = xOriginalClientIP
	} else if xClientIP != "" {
		selectedIP = xClientIP
	} else if cfIP != "" {
		selectedIP = cfIP
	} else if fwdFor != "" {
		ips := strings.Split(fwdFor, ",")
		if len(ips) > 0 {
			selectedIP = strings.TrimSpace(ips[0])
		}
	} else if realIP != "" {
		selectedIP = realIP
	} else {
		addr := c.Request.RemoteAddr
		if strings.Contains(addr, ":") {
			if host, _, err := net.SplitHostPort(addr); err == nil {
				selectedIP = host
			} else {
				selectedIP = addr
			}
		} else {
			selectedIP = addr
		}
	}

	return selectedIP
}

func GetIPHeadersFromContext(c *gin.Context) map[string]string {
	headers := make(map[string]string)

	headersToExtract := []string{
		"X-Original-Client-Ip",
		"X-Client-IP",
		"CF-Connecting-IP",
		"CF-IPCountry",
		"X-Forwarded-For",
		"X-Real-IP",
		"X-Forwarded-Proto",
		"X-Forwarded-Host",
	}

	for _, header := range headersToExtract {
		if value := c.Request.Header.Get(header); value != "" {
			headers[header] = value
		}
	}

	if originalIP := c.GetString("original_client_ip"); originalIP != "" {
		headers["X-Original-Client-Ip"] = originalIP
	}

	if clientIP := c.GetString("client_ip"); clientIP != "" && headers["X-Client-IP"] == "" {
		headers["X-Client-IP"] = clientIP
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
