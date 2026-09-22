package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQANCollectRequestValidate(t *testing.T) {
	request := validQANRequest()
	if err := request.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestQANCollectRequestRejectsEmptyBuckets(t *testing.T) {
	if err := (QANCollectRequest{}).Validate(); err == nil || !strings.Contains(err.Error(), "buckets") {
		t.Fatalf("Validate() error = %v, want buckets error", err)
	}
}

func TestQANCollectRequestRejectsMissingRequiredFields(t *testing.T) {
	tests := map[string]func(*QANCollectRequest){
		"query_id":               func(r *QANCollectRequest) { r.Buckets[0].QueryID = "" },
		"fingerprint":            func(r *QANCollectRequest) { r.Buckets[0].Fingerprint = "" },
		"period_start_unix_secs": func(r *QANCollectRequest) { r.Buckets[0].PeriodStartUnixSecs = 0 },
		"period_length_secs":     func(r *QANCollectRequest) { r.Buckets[0].PeriodLengthSecs = 0 },
		"service_name":           func(r *QANCollectRequest) { r.Buckets[0].ServiceName = "" },
		"service_type":           func(r *QANCollectRequest) { r.Buckets[0].ServiceType = "" },
		"agent_id":               func(r *QANCollectRequest) { r.Buckets[0].AgentID = "" },
		"agent_type":             func(r *QANCollectRequest) { r.Buckets[0].AgentType = "" },
		"num_queries":            func(r *QANCollectRequest) { r.Buckets[0].NumQueries = 0 },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			request := validQANRequest()
			mutate(&request)
			if err := request.Validate(); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("Validate() error = %v, want field %s", err, name)
			}
		})
	}
}

func TestQANFixtureDecodesAndValidates(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "qan-fixtures", "collect_request.json"))
	if err != nil {
		t.Fatal(err)
	}
	var request QANCollectRequest
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("fixture Validate() error = %v", err)
	}
	if got := request.Buckets[0].Metrics.RowsExaminedSum; got != 1200 {
		t.Fatalf("RowsExaminedSum = %v, want 1200", got)
	}
}

func validQANRequest() QANCollectRequest {
	var request QANCollectRequest
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "qan-fixtures", "collect_request.json"))
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(body, &request); err != nil {
		panic(err)
	}
	return request
}
