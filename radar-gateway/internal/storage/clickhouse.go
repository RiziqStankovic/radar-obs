package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Event struct {
	Signal  string
	Source  string
	Payload map[string]any
}

type Store struct {
	endpoint string
	database string
	user     string
	password string
	client   *http.Client
}

func NewClickHouse(endpoint, database, user, password string) (*Store, error) {
	endpoint = strings.TrimRight(endpoint, "/")
	if endpoint == "" {
		return nil, fmt.Errorf("ClickHouse endpoint is required")
	}
	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return nil, fmt.Errorf("invalid ClickHouse endpoint: %w", err)
	}
	if database == "" {
		database = "default"
	}
	if strings.ContainsAny(database, ",;") {
		return nil, fmt.Errorf("ClickHouse database must be exactly one database name, got %q; use RADAR_CLICKHOUSE_DATABASE=radar_metrics, not a comma/semicolon-separated list", database)
	}
	return &Store{endpoint: endpoint, database: database, user: user, password: password, client: &http.Client{Timeout: 10 * time.Second}}, nil
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	const query = `CREATE TABLE IF NOT EXISTS radar_events (
		received_at DateTime64(3),
		signal LowCardinality(String),
		source LowCardinality(String),
		payload String
	) ENGINE = MergeTree ORDER BY received_at`
	return s.exec(ctx, query)
}

func (s *Store) InsertEvent(ctx context.Context, event Event) error {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	body := strings.NewReader(fmt.Sprintf("%s\t%s\t%s\t%s\n", time.Now().UTC().Format("2006-01-02 15:04:05.000"), event.Signal, event.Source, payload))
	query := "INSERT INTO radar_events (received_at, signal, source, payload) FORMAT TabSeparated"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.queryURL(query), body)
	if err != nil {
		return fmt.Errorf("create ClickHouse insert request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	s.authorize(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("insert event into ClickHouse: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ClickHouse insert returned status %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	return nil
}

func (s *Store) exec(ctx context.Context, query string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.queryURL(query), nil)
	if err != nil {
		return fmt.Errorf("create ClickHouse schema request: %w", err)
	}
	s.authorize(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("create ClickHouse schema: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("ClickHouse schema returned status %s: %s", resp.Status, strings.TrimSpace(string(message)))
	}
	return nil
}

func (s *Store) queryURL(query string) string {
	values := url.Values{}
	values.Set("database", s.database)
	values.Set("query", query)
	return s.endpoint + "/?" + values.Encode()
}

func (s *Store) authorize(req *http.Request) {
	if s.user != "" {
		req.Header.Set("X-ClickHouse-User", s.user)
	}
	if s.password != "" {
		req.Header.Set("X-ClickHouse-Key", s.password)
	}
}

func readResponse(resp *http.Response) (string, error) {
	message, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return strings.TrimSpace(string(message)), err
}

func (s *Store) Query(ctx context.Context, query string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.queryURL(query+" FORMAT TabSeparated"), nil)
	if err != nil {
		return nil, err
	}
	s.authorize(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("ClickHouse query returned status %s", resp.Status)
	}
	var rows []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		rows = append(rows, scanner.Text())
	}
	return rows, scanner.Err()
}
