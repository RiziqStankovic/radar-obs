package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type MetricStore interface {
	InsertMetricRows(context.Context, string, []map[string]any) error
}

func (s *Store) VerifyMetricTables(ctx context.Context) error {
	rows, err := s.Query(ctx, "SELECT name FROM system.tables WHERE database = '"+s.database+"' AND name IN ('otel_metrics_gauge', 'otel_metrics_sum', 'otel_metrics_histogram', 'otel_metrics_summary', 'otel_metrics_exp_histogram') ORDER BY name")
	if err != nil {
		return fmt.Errorf("verify ClickHouse metric tables: %w", err)
	}
	if len(rows) < 4 {
		return fmt.Errorf("database %q has %d metric tables; expected at least 4 existing otel_metrics_* tables", s.database, len(rows))
	}
	return nil
}

func (s *Store) InsertMetricRows(ctx context.Context, table string, rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	if !allowedMetricTable(table) {
		return fmt.Errorf("unsupported metrics table %q", table)
	}
	var body strings.Builder
	for _, row := range rows {
		encoded, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("marshal metric row: %w", err)
		}
		body.Write(encoded)
		body.WriteByte('\n')
	}
	query := "INSERT INTO " + table + " FORMAT JSONEachRow"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.queryURL(query), strings.NewReader(body.String()))
	if err != nil {
		return fmt.Errorf("create ClickHouse metric insert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	s.authorize(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("insert metrics into ClickHouse: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		message, _ := readResponse(resp)
		return fmt.Errorf("ClickHouse metric insert returned status %s: %s", resp.Status, message)
	}
	return nil
}

func allowedMetricTable(table string) bool {
	switch table {
	case "otel_metrics_gauge", "otel_metrics_sum", "otel_metrics_histogram", "otel_metrics_summary", "otel_metrics_exp_histogram":
		return true
	default:
		return false
	}
}
