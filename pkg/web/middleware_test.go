package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupEngine(middlewares ...gin.HandlerFunc) *gin.Engine {
	e := gin.New()
	for _, mw := range middlewares {
		e.Use(mw)
	}
	e.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return e
}

func TestSecureHeadersMiddleware(t *testing.T) {
	e := setupEngine(SecureHeadersMiddleware())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	e.ServeHTTP(w, req)

	expected := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"X-Xss-Protection":          "1; mode=block",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
	}

	for header, want := range expected {
		got := w.Header().Get(header)
		if got != want {
			t.Errorf("header %s: expected %q, got %q", header, want, got)
		}
	}
}

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	e := setupEngine(RequestIDMiddleware())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	e.ServeHTTP(w, req)

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("expected X-Request-ID to be set")
	}
	if len(id) < 20 {
		t.Errorf("X-Request-ID looks too short: %q", id)
	}
}

func TestRequestIDMiddleware_PreservesExisting(t *testing.T) {
	e := setupEngine(RequestIDMiddleware())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "my-custom-id")
	e.ServeHTTP(w, req)

	id := w.Header().Get("X-Request-ID")
	if id != "my-custom-id" {
		t.Errorf("expected preserved X-Request-ID 'my-custom-id', got %q", id)
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	e := gin.New()
	e.Use(gin.Recovery())
	e.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	e.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic, got %d", w.Code)
	}
}

func TestRealIPMiddleware_XForwardedFor(t *testing.T) {
	e := setupEngine(RealIPMiddleware())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")
	e.ServeHTTP(w, req)

	ip := w.Header().Get("X-Client-IP")
	if ip != "203.0.113.50" {
		t.Errorf("expected first IP from X-Forwarded-For, got %q", ip)
	}
}

func TestRealIPMiddleware_CFConnectingIP(t *testing.T) {
	e := setupEngine(RealIPMiddleware())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("CF-Connecting-IP", "198.51.100.10")
	e.ServeHTTP(w, req)

	ip := w.Header().Get("X-Client-IP")
	if ip != "198.51.100.10" {
		t.Errorf("expected CF-Connecting-IP value, got %q", ip)
	}
}

func TestNoRouteHandler(t *testing.T) {
	e := gin.New()
	e.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	e.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHealthEndpoint(t *testing.T) {
	e := gin.New()
	e.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	e.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("expected JSON content type, got %q", ct)
	}
}
