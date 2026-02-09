package client

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

func TestMetricsMiddlewareIdempotentRegistration(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("MetricsMiddleware panicked on double registration: %v", r)
		}
	}()

	cfg := &MetricsConfig{Namespace: "test_idempotent", Subsystem: "sub"}
	mw1 := MetricsMiddleware(cfg)
	if mw1 == nil {
		t.Fatal("expected non-nil middleware")
	}
	mw2 := MetricsMiddleware(cfg)
	if mw2 == nil {
		t.Fatal("expected non-nil middleware on second call")
	}
}

func TestMaxResponseSizeMiddleware(t *testing.T) {
	body := []byte("hello world, this is a long response body for testing")
	transport := &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(body)),
			}, nil
		},
	}

	mw := MaxResponseSizeMiddleware(5)
	wrapped := mw(transport)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, err := wrapped.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := io.ReadAll(resp.Body)
	if len(data) > 5 {
		t.Errorf("expected at most 5 bytes, got %d", len(data))
	}
}
