package web

import (
	"log"
	"net"
	"net/http"
	"os"
	"strings"

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
	// Log all relevant headers for debugging
	cfIP := c.Request.Header.Get("CF-Connecting-IP")
	fwdFor := c.Request.Header.Get("X-Forwarded-For")
	realIP := c.Request.Header.Get("X-Real-Ip")
	xClientIP := c.Request.Header.Get("X-Client-IP")
	xForwardedProto := c.Request.Header.Get("X-Forwarded-Proto")
	xForwardedHost := c.Request.Header.Get("X-Forwarded-Host")
	remoteAddr := c.Request.RemoteAddr

	log.Printf("[IP DEBUG] Request from %s %s", c.Request.Method, c.Request.URL.Path)
	log.Printf("[IP DEBUG] CF-Connecting-IP: '%s'", cfIP)
	log.Printf("[IP DEBUG] X-Forwarded-For: '%s'", fwdFor)
	log.Printf("[IP DEBUG] X-Real-Ip: '%s'", realIP)
	log.Printf("[IP DEBUG] X-Client-IP: '%s'", xClientIP)
	log.Printf("[IP DEBUG] X-Forwarded-Proto: '%s'", xForwardedProto)
	log.Printf("[IP DEBUG] X-Forwarded-Host: '%s'", xForwardedHost)
	log.Printf("[IP DEBUG] RemoteAddr: '%s'", remoteAddr)

	// Log all headers that might contain IP information
	for name, values := range c.Request.Header {
		if strings.Contains(strings.ToLower(name), "ip") ||
			strings.Contains(strings.ToLower(name), "forward") ||
			strings.Contains(strings.ToLower(name), "client") ||
			strings.Contains(strings.ToLower(name), "real") {
			log.Printf("[IP DEBUG] Header %s: %v", name, values)
		}
	}

	// Check CF-Connecting-IP first (Cloudflare)
	if cfIP != "" {
		log.Printf("[IP DEBUG] Using CF-Connecting-IP: %s", cfIP)
		return cfIP
	}

	// Check X-Client-IP (inter-service communication)
	if xClientIP != "" {
		log.Printf("[IP DEBUG] Using X-Client-IP: %s", xClientIP)
		return xClientIP
	}

	// Check X-Forwarded-For (standard proxy header)
	if fwdFor != "" {
		ips := strings.Split(fwdFor, ",")
		log.Printf("[IP DEBUG] X-Forwarded-For contains %d IPs: %v", len(ips), ips)
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			log.Printf("[IP DEBUG] Using first IP from X-Forwarded-For: %s", clientIP)
			return clientIP
		}
	}

	// Check X-Real-Ip
	if realIP != "" {
		log.Printf("[IP DEBUG] Using X-Real-Ip: %s", realIP)
		return realIP
	}

	// Fallback to RemoteAddr
	addr := c.Request.RemoteAddr
	log.Printf("[IP DEBUG] Falling back to RemoteAddr: %s", addr)
	if strings.Contains(addr, ":") {
		if host, port, err := net.SplitHostPort(addr); err == nil {
			log.Printf("[IP DEBUG] Extracted host '%s' from '%s' (port: %s)", host, addr, port)
			return host
		} else {
			log.Printf("[IP DEBUG] Failed to split host:port from '%s': %v", addr, err)
		}
	}

	log.Printf("[IP DEBUG] Final IP result: %s", addr)
	return addr
}
