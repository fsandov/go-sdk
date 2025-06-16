package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultTimeout = 2 * time.Second

type Client struct {
	baseURL    string
	timeout    time.Duration
	httpClient *http.Client
}

type Option func(*Client)

func WithURL(url string) Option {
	return func(c *Client) {
		c.baseURL = url
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

func NewClient(opts ...Option) (*Client, error) {
	client := &Client{
		baseURL:    "",
		timeout:    defaultTimeout,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		opt(client)
	}
	client.httpClient.Timeout = client.timeout

	if client.baseURL == "" {
		return nil, fmt.Errorf("discord client: baseURL is required")
	}
	return client, nil
}

type WebhookPayload struct {
	Username  string `json:"username,omitempty"`
	Content   string `json:"content"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Embeds    []any  `json:"embeds,omitempty"`
}

func (c *Client) SendWebhook(ctx context.Context, payload WebhookPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshalling payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("discord webhook failed, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
}
