package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type QANBucket struct {
	QueryID             string             `json:"query_id"`
	Fingerprint         string             `json:"fingerprint"`
	ServiceName         string             `json:"service_name"`
	ServiceType         string             `json:"service_type"`
	Database            string             `json:"database,omitempty"`
	Schema              string             `json:"schema,omitempty"`
	Tables              []string           `json:"tables,omitempty"`
	Username            string             `json:"username,omitempty"`
	ClientHost          string             `json:"client_host,omitempty"`
	AgentID             string             `json:"agent_id"`
	AgentType           string             `json:"agent_type"`
	PeriodStartUnixSecs uint32             `json:"period_start_unix_secs"`
	PeriodLengthSecs    uint32             `json:"period_length_secs"`
	Example             string             `json:"example,omitempty"`
	ExampleTruncated    bool               `json:"example_truncated,omitempty"`
	NumQueries          float64            `json:"num_queries"`
	Metrics             QANMetrics         `json:"metrics,omitempty"`
	Labels              map[string]string  `json:"labels,omitempty"`
	ExtraMetrics        map[string]float64 `json:"extra_metrics,omitempty"`
}

type QANMetrics struct {
	QueryTimeCnt    float64 `json:"query_time_cnt,omitempty"`
	QueryTimeSum    float64 `json:"query_time_sum,omitempty"`
	QueryTimeMin    float64 `json:"query_time_min,omitempty"`
	QueryTimeMax    float64 `json:"query_time_max,omitempty"`
	QueryTimeP99    float64 `json:"query_time_p99,omitempty"`
	LockTimeSum     float64 `json:"lock_time_sum,omitempty"`
	RowsExaminedSum float64 `json:"rows_examined_sum,omitempty"`
	RowsSentSum     float64 `json:"rows_sent_sum,omitempty"`
}

func (s *Store) EnsureQANSchema(ctx context.Context) error {
	const query = `CREATE TABLE IF NOT EXISTS qan_metrics (
		received_at DateTime64(3),
		period_start DateTime,
		period_length UInt32,
		query_id LowCardinality(String),
		fingerprint String,
		service_name LowCardinality(String),
		service_type LowCardinality(String),
		database LowCardinality(String),
		schema LowCardinality(String),
		tables Array(String),
		username LowCardinality(String),
		client_host LowCardinality(String),
		agent_id LowCardinality(String),
		agent_type LowCardinality(String),
		labels String,
		example String,
		example_truncated UInt8,
		num_queries Float64,
		query_time_cnt Float64,
		query_time_sum Float64,
		query_time_min Float64,
		query_time_max Float64,
		query_time_p99 Float64,
		lock_time_sum Float64,
		rows_examined_sum Float64,
		rows_sent_sum Float64,
		extra_metrics String
	) ENGINE = MergeTree
	PARTITION BY toYYYYMMDD(period_start)
	ORDER BY (query_id, service_name, database, schema, period_start)`
	return s.exec(ctx, query)
}

func (s *Store) VerifyQANTable(ctx context.Context) error {
	rows, err := s.Query(ctx, "SELECT name FROM system.tables WHERE database = '"+s.database+"' AND name = 'qan_metrics'")
	if err != nil {
		return fmt.Errorf("verify ClickHouse QAN table: %w", err)
	}
	if len(rows) != 1 {
		return fmt.Errorf("database %q does not have required qan_metrics table", s.database)
	}
	return nil
}

func (s *Store) InsertQANBuckets(ctx context.Context, buckets []QANBucket) error {
	if len(buckets) == 0 {
		return nil
	}
	var body strings.Builder
	for _, bucket := range buckets {
		row, err := MapQANBucket(bucket, time.Now().UTC())
		if err != nil {
			return err
		}
		encoded, err := json.Marshal(row)
		if err != nil {
			return fmt.Errorf("marshal QAN row: %w", err)
		}
		body.Write(encoded)
		body.WriteByte('\n')
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.queryURL("INSERT INTO qan_metrics FORMAT JSONEachRow"), strings.NewReader(body.String()))
	if err != nil {
		return fmt.Errorf("create ClickHouse QAN insert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	s.authorize(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("insert QAN buckets into ClickHouse: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		message, _ := readResponse(resp)
		return fmt.Errorf("ClickHouse QAN insert returned status %s: %s", resp.Status, message)
	}
	return nil
}

func MapQANBucket(bucket QANBucket, receivedAt time.Time) (map[string]any, error) {
	labels, err := json.Marshal(emptyMap(bucket.Labels))
	if err != nil {
		return nil, fmt.Errorf("marshal QAN labels: %w", err)
	}
	extraMetrics, err := json.Marshal(emptyFloatMap(bucket.ExtraMetrics))
	if err != nil {
		return nil, fmt.Errorf("marshal QAN extra metrics: %w", err)
	}
	return map[string]any{
		"received_at":       receivedAt.UTC().Format("2006-01-02 15:04:05.000"),
		"period_start":      time.Unix(int64(bucket.PeriodStartUnixSecs), 0).UTC().Format("2006-01-02 15:04:05"),
		"period_length":     bucket.PeriodLengthSecs,
		"query_id":          bucket.QueryID,
		"fingerprint":       bucket.Fingerprint,
		"service_name":      bucket.ServiceName,
		"service_type":      bucket.ServiceType,
		"database":          bucket.Database,
		"schema":            bucket.Schema,
		"tables":            bucket.Tables,
		"username":          bucket.Username,
		"client_host":       bucket.ClientHost,
		"agent_id":          bucket.AgentID,
		"agent_type":        bucket.AgentType,
		"labels":            string(labels),
		"example":           bucket.Example,
		"example_truncated": boolToUInt8(bucket.ExampleTruncated),
		"num_queries":       bucket.NumQueries,
		"query_time_cnt":    bucket.Metrics.QueryTimeCnt,
		"query_time_sum":    bucket.Metrics.QueryTimeSum,
		"query_time_min":    bucket.Metrics.QueryTimeMin,
		"query_time_max":    bucket.Metrics.QueryTimeMax,
		"query_time_p99":    bucket.Metrics.QueryTimeP99,
		"lock_time_sum":     bucket.Metrics.LockTimeSum,
		"rows_examined_sum": bucket.Metrics.RowsExaminedSum,
		"rows_sent_sum":     bucket.Metrics.RowsSentSum,
		"extra_metrics":     string(extraMetrics),
	}, nil
}

func emptyMap(input map[string]string) map[string]string {
	if input == nil {
		return map[string]string{}
	}
	return input
}

func emptyFloatMap(input map[string]float64) map[string]float64 {
	if input == nil {
		return map[string]float64{}
	}
	return input
}

func boolToUInt8(value bool) uint8 {
	if value {
		return 1
	}
	return 0
}
