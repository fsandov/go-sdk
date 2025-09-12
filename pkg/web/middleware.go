package web

import (
	"context"
	"log"
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
			log.Printf("[IP PROPAGATION] Preserving X-Original-Client-Ip: %s", originalIP)
		} else {
			c.Set("original_client_ip", ip)
			c.Writer.Header().Set("X-Original-Client-Ip", ip)
			log.Printf("[IP PROPAGATION] Setting X-Original-Client-Ip to detected IP: %s", ip)
		}

		log.Printf("[IP PROPAGATION] Middleware processed - client_ip: %s, original_client_ip: %s", ip, c.GetString("original_client_ip"))
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
	xForwardedProto := c.Request.Header.Get("X-Forwarded-Proto")
	xForwardedHost := c.Request.Header.Get("X-Forwarded-Host")
	remoteAddr := c.Request.RemoteAddr

	log.Printf("[IP PROPAGATION] Incoming request %s %s", c.Request.Method, c.Request.URL.Path)
	log.Printf("[IP PROPAGATION] X-Original-Client-Ip: '%s'", xOriginalClientIP)
	log.Printf("[IP PROPAGATION] CF-Connecting-IP: '%s'", cfIP)
	log.Printf("[IP PROPAGATION] X-Forwarded-For: '%s'", fwdFor)
	log.Printf("[IP PROPAGATION] X-Real-Ip: '%s'", realIP)
	log.Printf("[IP PROPAGATION] X-Client-IP: '%s'", xClientIP)
	log.Printf("[IP PROPAGATION] X-Forwarded-Proto: '%s'", xForwardedProto)
	log.Printf("[IP PROPAGATION] X-Forwarded-Host: '%s'", xForwardedHost)
	log.Printf("[IP PROPAGATION] RemoteAddr: '%s'", remoteAddr)

	for name, values := range c.Request.Header {
		if strings.Contains(strings.ToLower(name), "ip") ||
			strings.Contains(strings.ToLower(name), "forward") ||
			strings.Contains(strings.ToLower(name), "client") ||
			strings.Contains(strings.ToLower(name), "real") ||
			strings.Contains(strings.ToLower(name), "original") {
			log.Printf("[IP PROPAGATION] Header %s: %v", name, values)
		}
	}

	var selectedIP string
	var source string

	if xOriginalClientIP != "" {
		selectedIP = xOriginalClientIP
		source = "X-Original-Client-Ip"
	} else if xClientIP != "" {
		selectedIP = xClientIP
		source = "X-Client-IP"
	} else if cfIP != "" {
		selectedIP = cfIP
		source = "CF-Connecting-IP"
	} else if fwdFor != "" {
		ips := strings.Split(fwdFor, ",")
		log.Printf("[IP PROPAGATION] X-Forwarded-For contains %d IPs: %v", len(ips), ips)
		if len(ips) > 0 {
			selectedIP = strings.TrimSpace(ips[0])
			source = "X-Forwarded-For[0]"
		}
	} else if realIP != "" {
		selectedIP = realIP
		source = "X-Real-Ip"
	} else {
		addr := c.Request.RemoteAddr
		if strings.Contains(addr, ":") {
			if host, port, err := net.SplitHostPort(addr); err == nil {
				log.Printf("[IP PROPAGATION] Extracted host '%s' from '%s' (port: %s)", host, addr, port)
				selectedIP = host
				source = "RemoteAddr"
			} else {
				log.Printf("[IP PROPAGATION] Failed to split host:port from '%s': %v", addr, err)
				selectedIP = addr
				source = "RemoteAddr"
			}
		} else {
			selectedIP = addr
			source = "RemoteAddr"
		}
	}

	log.Printf("[IP PROPAGATION] Selected IP: %s (source: %s)", selectedIP, source)
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

	log.Printf("[IP PROPAGATION] Extracted %d IP headers from Gin context", len(headers))
	return headers
}

func IPContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := GetIPHeadersFromContext(c)

		enrichedCtx := context.WithValue(c.Request.Context(), client.IPHeadersContextKey, headers)
		c.Request = c.Request.WithContext(enrichedCtx)

		log.Printf("[IP PROPAGATION] Middleware enriched context with %d IP headers for %s %s",
			len(headers), c.Request.Method, c.Request.URL.Path)

		c.Next()
	}
}
