package storage

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMapQANBucket(t *testing.T) {
	row, err := MapQANBucket(QANBucket{
		QueryID:             "q1",
		Fingerprint:         "select ?",
		ServiceName:         "svc",
		ServiceType:         "mysql",
		Database:            "app",
		Schema:              "public",
		Tables:              []string{"users"},
		Username:            "app_user",
		ClientHost:          "127.0.0.1",
		AgentID:             "agent",
		AgentType:           "mysql-fixture",
		PeriodStartUnixSecs: 1720000000,
		PeriodLengthSecs:    60,
		ExampleTruncated:    true,
		NumQueries:          2,
		Metrics: QANMetrics{
			QueryTimeCnt:    2,
			QueryTimeSum:    1.5,
			QueryTimeMin:    0.2,
			QueryTimeMax:    1.3,
			QueryTimeP99:    1.3,
			LockTimeSum:     0.1,
			RowsExaminedSum: 20,
			RowsSentSum:     4,
		},
		Labels:       map[string]string{"environment": "test"},
		ExtraMetrics: map[string]float64{"rows_affected_sum": 1},
	}, time.Date(2026, 1, 2, 3, 4, 5, 6000000, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, row["received_at"], "2026-01-02 03:04:05.006")
	assertEqual(t, row["period_start"], "2024-07-03 09:46:40")
	assertEqual(t, row["query_id"], "q1")
	assertEqual(t, row["example_truncated"], uint8(1))
	assertEqual(t, row["query_time_sum"], 1.5)
	if got := row["tables"].([]string); len(got) != 1 || got[0] != "users" {
		t.Fatalf("tables = %#v, want users", got)
	}
	var labels map[string]string
	if err := json.Unmarshal([]byte(row["labels"].(string)), &labels); err != nil {
		t.Fatal(err)
	}
	if labels["environment"] != "test" {
		t.Fatalf("labels = %#v, want environment=test", labels)
	}
	var extra map[string]float64
	if err := json.Unmarshal([]byte(row["extra_metrics"].(string)), &extra); err != nil {
		t.Fatal(err)
	}
	if extra["rows_affected_sum"] != 1 {
		t.Fatalf("extra = %#v, want rows_affected_sum=1", extra)
	}
}

func TestMapQANBucketOmittedExampleAndNilMaps(t *testing.T) {
	row, err := MapQANBucket(QANBucket{PeriodStartUnixSecs: 1720000000}, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, row["example"], "")
	assertEqual(t, row["labels"], "{}")
	assertEqual(t, row["extra_metrics"], "{}")
}

func TestEnsureQANSchemaCreatesTable(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	store, err := NewClickHouse(server.URL, "default", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureQANSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "CREATE TABLE IF NOT EXISTS qan_metrics") {
		t.Fatalf("query = %q, want qan_metrics schema", gotQuery)
	}
}

func TestInsertQANBucketsPostsJSONEachRow(t *testing.T) {
	var gotQuery string
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	store, err := NewClickHouse(server.URL, "default", "", "")
	if err != nil {
		t.Fatal(err)
	}
	err = store.InsertQANBuckets(context.Background(), []QANBucket{{
		QueryID: "q1", Fingerprint: "select ?", ServiceName: "svc", ServiceType: "mysql", AgentID: "agent", AgentType: "fixture", PeriodStartUnixSecs: 1720000000, PeriodLengthSecs: 60, NumQueries: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery != "INSERT INTO qan_metrics FORMAT JSONEachRow" {
		t.Fatalf("query = %q", gotQuery)
	}
	if !strings.Contains(gotBody, `"query_id":"q1"`) {
		t.Fatalf("body = %s, want query_id", gotBody)
	}
}

func TestInsertQANBucketsReturnsClickHouseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer server.Close()
	store, err := NewClickHouse(server.URL, "default", "", "")
	if err != nil {
		t.Fatal(err)
	}
	err = store.InsertQANBuckets(context.Background(), []QANBucket{{
		QueryID: "q1", Fingerprint: "select ?", ServiceName: "svc", ServiceType: "mysql", AgentID: "agent", AgentType: "fixture", PeriodStartUnixSecs: 1720000000, PeriodLengthSecs: 60, NumQueries: 1,
	}})
	if err == nil || !strings.Contains(err.Error(), "ClickHouse QAN insert returned status") {
		t.Fatalf("error = %v, want ClickHouse status error", err)
	}
}

func assertEqual(t *testing.T, got, want any) {
	t.Helper()
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
