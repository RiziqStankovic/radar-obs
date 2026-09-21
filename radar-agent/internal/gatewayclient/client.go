package gatewayclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitrepo.xlaxiata.id/radar/radar-agent/internal/collectors"
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/v1/radar/events", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create gateway request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("publish event: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("gateway returned status %d", resp.StatusCode)
	}
	return nil
}
