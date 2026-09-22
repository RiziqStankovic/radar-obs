package gatewayclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitrepo.xlaxiata.id/radar/radar-agent/internal/qan"
)

func TestPublishQANPostsCollectRequest(t *testing.T) {
	var gotPath string
	var gotRequest qan.CollectRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("content type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	client := New(server.URL)
	if err := client.PublishQAN(context.Background(), qan.FixtureRequest(false)); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/qan/collect" {
		t.Fatalf("path = %q, want /v1/qan/collect", gotPath)
	}
	if gotRequest.Buckets[0].QueryID != "7d1f1c4b8d" {
		t.Fatalf("QueryID = %q", gotRequest.Buckets[0].QueryID)
	}
}

func TestPublishQANReturnsNon2xxError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer server.Close()
	client := New(server.URL)
	if err := client.PublishQAN(context.Background(), qan.FixtureRequest(false)); err == nil {
		t.Fatal("PublishQAN() error = nil, want error")
	}
}
