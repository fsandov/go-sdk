package web

import (
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
	if fwdFor := c.Request.Header.Get("X-Forwarded-For"); fwdFor != "" {
		ips := strings.FieldsFunc(fwdFor, func(r rune) bool {
			return r == ',' || r == ' '
		})
		if len(ips) > 0 {
			return ips[len(ips)-1]
		}
	}

	if realIP := c.Request.Header.Get("X-Real-Ip"); realIP != "" {
		return realIP
	}

	addr := c.Request.RemoteAddr
	if strings.Contains(addr, ":") {
		if host, _, err := net.SplitHostPort(addr); err == nil {
			return host
		}
	}
	return addr
}
