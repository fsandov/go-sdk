package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

type mockTransport struct {
	roundTripFunc func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestRetryClosesBodyOnFailedAttempts(t *testing.T) {
	var closeCalls int32

	transport := &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 500,
				Body: &trackingReadCloser{
					Reader:  bytes.NewReader([]byte("error")),
					onClose: func() { atomic.AddInt32(&closeCalls, 1) },
				},
			}, nil
		},
	}

	c := &Client{
		httpClient: &http.Client{Transport: transport},
		options: &options{
			defaultSettings: &EndpointSettings{
				Timeout:    5 * time.Second,
				MaxRetries: 2,
				Headers:    map[string]string{},
			},
			hooks: &HooksConfig{},
		},
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/test", nil)
	_, _ = c.Do(context.Background(), req)

	closed := atomic.LoadInt32(&closeCalls)
	if closed < 3 {
		t.Errorf("expected at least 3 body closes (2 retries + final read), got %d", closed)
	}
}

func TestFallbackDoesNotPanic(t *testing.T) {
	transport := &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	t.Run("fallback returns plain error", func(t *testing.T) {
		c := &Client{
			httpClient: &http.Client{Transport: transport},
			options: &options{
				defaultSettings: &EndpointSettings{
					Timeout:    5 * time.Second,
					MaxRetries: 0,
					Headers:    map[string]string{},
					Fallback: func(req *http.Request, err error) (*http.Response, error) {
						return nil, fmt.Errorf("fallback error: %w", err)
					},
				},
				hooks: &HooksConfig{},
			},
		}

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/test", nil)

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("fallback caused panic: %v", r)
			}
		}()

		_, cErr := c.Do(context.Background(), req)
		if cErr == nil {
			t.Fatal("expected error from fallback")
		}
	})

	t.Run("fallback returns nil error", func(t *testing.T) {
		c := &Client{
			httpClient: &http.Client{Transport: transport},
			options: &options{
				defaultSettings: &EndpointSettings{
					Timeout:    5 * time.Second,
					MaxRetries: 0,
					Headers:    map[string]string{},
					Fallback: func(req *http.Request, err error) (*http.Response, error) {
						return &http.Response{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewReader([]byte("ok"))),
						}, nil
					},
				},
				hooks: &HooksConfig{},
			},
		}

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/test", nil)
		resp, cErr := c.Do(context.Background(), req)
		if cErr != nil {
			t.Fatalf("expected nil error, got %v", cErr)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestTimeoutRespectsContextDeadline(t *testing.T) {
	transport := &mockTransport{
		roundTripFunc: func(req *http.Request) (*http.Response, error) {
			deadline, ok := req.Context().Deadline()
			if !ok {
				t.Error("expected context to have deadline")
			}
			remaining := time.Until(deadline)
			if remaining > 3*time.Second {
				t.Errorf("deadline too far: %v", remaining)
			}
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		},
	}

	c := &Client{
		httpClient: &http.Client{Transport: transport},
		options: &options{
			defaultSettings: &EndpointSettings{
				Timeout:    2 * time.Second,
				MaxRetries: 0,
				Headers:    map[string]string{},
			},
			hooks: &HooksConfig{},
		},
	}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/test", nil)
	_, _ = c.Do(context.Background(), req)
}

type trackingReadCloser struct {
	io.Reader
	onClose func()
}

func (r *trackingReadCloser) Close() error {
	if r.onClose != nil {
		r.onClose()
	}
	return nil
}

func (r *trackingReadCloser) Read(p []byte) (int, error) {
	return r.Reader.Read(p)
}
