package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	options    *options
}

type options struct {
	baseURL         string
	endpointConfig  EndpointConfig
	defaultSettings *EndpointSettings
	middlewares     []Middleware
	hooks           *HooksConfig
}

func WithBaseURL(url string) func(*options) {
	return func(o *options) { o.baseURL = strings.TrimRight(url, "/") }
}
func WithEndpointConfig(ec EndpointConfig) func(*options) {
	return func(o *options) { o.endpointConfig = ec }
}
func WithDefaultSettings(s *EndpointSettings) func(*options) {
	return func(o *options) { o.defaultSettings = s }
}
func WithMiddleware(mw Middleware) func(*options) {
	return func(o *options) { o.middlewares = append(o.middlewares, mw) }
}
func WithHooks(hooks *HooksConfig) func(*options) { return func(o *options) { o.hooks = hooks } }

func NewClient(opts ...func(*options)) *Client {
	o := &options{
		defaultSettings: &EndpointSettings{
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Headers:    map[string]string{},
		},
		hooks: &HooksConfig{},
	}
	for _, opt := range opts {
		opt(o)
	}
	transport := http.DefaultTransport
	for i := len(o.middlewares) - 1; i >= 0; i-- {
		transport = o.middlewares[i](transport)
	}
	return &Client{
		httpClient: &http.Client{Transport: transport, Timeout: o.defaultSettings.Timeout},
		options:    o,
	}
}

type EndpointConfigKey struct{}

func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, *Error) {
	var cfg *EndpointSettings
	if c.options.endpointConfig != nil {
		cfg = c.options.endpointConfig(req.Method, req.URL.Path)
	}
	if cfg == nil {
		cfg = c.options.defaultSettings
	}
	cfg = applyDefaults(cfg)
	ValidateEndpointConfig(cfg, req.URL.Path)
	ctx = context.WithValue(ctx, EndpointConfigKey{}, cfg)

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	req = req.WithContext(ctx)

	for k, v := range c.options.defaultSettings.Headers {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	if cfg.AuthTokenFn != nil {
		if token, err := cfg.AuthTokenFn(&RequestInfo{Method: req.Method, Path: req.URL.Path}); err == nil && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	var (
		resp      *http.Response
		err       error
		retry     int
		body      []byte
		clientErr *Error
	)
	shouldRetry := cfg.ShouldRetry
	if shouldRetry == nil {
		shouldRetry = func(resp *http.Response, err error) bool {
			return err != nil || (resp != nil && resp.StatusCode >= 500)
		}
	}
	backoffStrategy := cfg.BackoffStrategy
	if backoffStrategy == nil {
		backoffStrategy = func(attempt int) time.Duration { return 200 * time.Millisecond }
	}

	for retry = 0; retry <= cfg.MaxRetries; retry++ {
		resp, err = c.httpClient.Do(req)
		if !shouldRetry(resp, err) {
			break
		}
		if retry < cfg.MaxRetries {
			time.Sleep(backoffStrategy(retry))
		}
	}
	if resp != nil && resp.Body != nil {
		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}
	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		clientErr = &Error{
			StatusCode:   0,
			Err:          err,
			Retries:      retry,
			Method:       req.Method,
			URL:          req.URL.String(),
			LastResponse: resp,
		}
		if resp != nil {
			clientErr.StatusCode = resp.StatusCode
			clientErr.Body = body
		}
		if cfg.Fallback != nil {
			resp, err := cfg.Fallback(req, err)
			return resp, err.(*Error)
		}
		return resp, clientErr
	}
	return resp, nil
}

func (c *Client) Get(ctx context.Context, path string, headers map[string]string) (*http.Response, *Error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.options.baseURL+path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.Do(ctx, req)
}
func (c *Client) Post(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, *Error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.options.baseURL+path, bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.Do(ctx, req)
}
func (c *Client) Put(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, *Error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, c.options.baseURL+path, bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.Do(ctx, req)
}
func (c *Client) Patch(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, *Error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPatch, c.options.baseURL+path, bytes.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.Do(ctx, req)
}
func (c *Client) Delete(ctx context.Context, path string, headers map[string]string) (*http.Response, *Error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, c.options.baseURL+path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.Do(ctx, req)
}
func (c *Client) Head(ctx context.Context, path string, headers map[string]string) (*http.Response, *Error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodHead, c.options.baseURL+path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.Do(ctx, req)
}

func (c *Client) Close() {
	if c.httpClient != nil {
		if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
}
