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
		"X-Xss-Protection":          "0",
		"Permissions-Policy":        "camera=(), microphone=(), geolocation=()",
		"Cache-Control":             "no-store",
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

func TestRealIPMiddleware_XForwardedFor_NotTrusted(t *testing.T) {
	e := setupEngine(RealIPMiddleware())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")
	e.ServeHTTP(w, req)

	ip := w.Header().Get("X-Client-IP")
	// Should use RemoteAddr, not XFF (RemoteAddr in tests is typically "192.0.2.1" or empty)
	if ip == "203.0.113.50" {
		t.Error("X-Forwarded-For should NOT be trusted from non-Cloudflare sources")
	}
}

func TestRealIPMiddleware_CFConnectingIP_FromCloudflare(t *testing.T) {
	e := gin.New()
	e.Use(RealIPMiddleware())
	e.GET("/test", func(c *gin.Context) {
		c.Header("X-Client-IP", c.GetString("client_ip"))
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("CF-Connecting-IP", "198.51.100.10")
	e.ServeHTTP(w, req)

	ip := w.Header().Get("X-Client-IP")
	if ip == "198.51.100.10" {
		t.Error("CF-Connecting-IP should only be trusted from Cloudflare IPs")
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

func TestXAuthAppTokenMiddleware_EmptyEnvRejectsAll(t *testing.T) {
	t.Setenv("X_AUTH_APP_TOKEN", "")
	e := setupEngine(XAuthAppTokenMiddleware())

	// Request with no token — must be rejected when env var is empty.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	e.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("empty env token: expected 401, got %d", w.Code)
	}

	// Request with empty header value — must also be rejected.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-Auth-App-Token", "")
	e.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("empty env token + empty header: expected 401, got %d", w2.Code)
	}
}

func TestXAuthAppTokenMiddleware_ValidToken(t *testing.T) {
	t.Setenv("X_AUTH_APP_TOKEN", "supersecret")
	e := setupEngine(XAuthAppTokenMiddleware())

	// Correct token — must pass.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Auth-App-Token", "supersecret")
	e.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("valid token: expected 200, got %d", w.Code)
	}
}

func TestXAuthAppTokenMiddleware_WrongToken(t *testing.T) {
	t.Setenv("X_AUTH_APP_TOKEN", "supersecret")
	e := setupEngine(XAuthAppTokenMiddleware())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Auth-App-Token", "wrongtoken")
	e.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: expected 401, got %d", w.Code)
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
