package gatewayclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitrepo.xlaxiata.id/radar/radar-agent/internal/collectors"
	"gitrepo.xlaxiata.id/radar/radar-agent/internal/qan"
)

type Client struct {
	endpoint string
	http     *http.Client
}

func New(endpoint string) *Client {
	return &Client{endpoint: strings.TrimRight(endpoint, "/"), http: &http.Client{Timeout: 5 * time.Second}}
}

func (c *Client) Publish(ctx context.Context, event collectors.Event) error {
	if c.endpoint == "" {
		return nil
	}
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return c.postJSON(ctx, "/v1/radar/events", body)
}

func (c *Client) PublishQAN(ctx context.Context, request qan.CollectRequest) error {
	if c.endpoint == "" {
		return nil
	}
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal QAN collect request: %w", err)
	}
	return c.postJSON(ctx, "/v1/qan/collect", body)
}

func (c *Client) postJSON(ctx context.Context, path string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create gateway request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("publish to gateway: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("gateway returned status %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	return nil
}
