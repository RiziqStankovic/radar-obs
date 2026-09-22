package qan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFixtureJSONMatchesSharedContractFixture(t *testing.T) {
	generated, err := FixtureJSON(true)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := os.ReadFile(filepath.Join("..", "..", "..", "qan-fixtures", "collect_request.json"))
	if err != nil {
		t.Fatal(err)
	}
	var generatedRequest CollectRequest
	if err := json.Unmarshal(generated, &generatedRequest); err != nil {
		t.Fatal(err)
	}
	var sharedRequest CollectRequest
	if err := json.Unmarshal(shared, &sharedRequest); err != nil {
		t.Fatal(err)
	}
	if generatedRequest.Buckets[0].QueryID != sharedRequest.Buckets[0].QueryID {
		t.Fatalf("QueryID = %q, want %q", generatedRequest.Buckets[0].QueryID, sharedRequest.Buckets[0].QueryID)
	}
	if generatedRequest.Buckets[0].Metrics.RowsExaminedSum != sharedRequest.Buckets[0].Metrics.RowsExaminedSum {
		t.Fatalf("RowsExaminedSum = %v, want %v", generatedRequest.Buckets[0].Metrics.RowsExaminedSum, sharedRequest.Buckets[0].Metrics.RowsExaminedSum)
	}
}

func TestFixtureJSONCanOmitExample(t *testing.T) {
	request := FixtureRequest(false)
	if request.Buckets[0].Example != "" {
		t.Fatalf("Example = %q, want empty", request.Buckets[0].Example)
	}
	if request.Buckets[0].Fingerprint == "" || request.Buckets[0].NumQueries == 0 {
		t.Fatalf("fixture lost fingerprint or aggregate metrics: %#v", request.Buckets[0])
	}
}
