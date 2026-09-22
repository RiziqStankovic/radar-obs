package ingest

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitrepo.xlaxiata.id/radar/radar-gateway/internal/storage"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	metrics "go.opentelemetry.io/proto/otlp/metrics/v1"
	resource "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

func TestOTLPAcceptsProtobufMetrics(t *testing.T) {
	h := NewHandler()
	body, err := proto.Marshal(&collectormetrics.ExportMetricsServiceRequest{
		ResourceMetrics: []*metrics.ResourceMetrics{{
			Resource: &resource.Resource{Attributes: []*common.KeyValue{{Key: "service.name", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: "test"}}}}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytesReader(body))
	req.Header.Set("Content-Type", "application/x-protobuf")
	res := httptest.NewRecorder()
	h.OTLP("metrics")(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}

type recordingStore struct {
	events     []Event
	qanBuckets []storage.QANBucket
	err        error
	qanErr     error
}

func (s *recordingStore) InsertEvent(_ context.Context, event storage.Event) error {
	s.events = append(s.events, Event{Signal: event.Signal, Source: event.Source, Payload: event.Payload})
	return s.err
}

func (s *recordingStore) InsertQANBuckets(_ context.Context, buckets []storage.QANBucket) error {
	s.qanBuckets = append(s.qanBuckets, buckets...)
	return s.qanErr
}

func TestOTLPAcceptsGzipProtobufMetrics(t *testing.T) {
	h := NewHandler()
	payload, err := proto.Marshal(&collectormetrics.ExportMetricsServiceRequest{
		ResourceMetrics: []*metrics.ResourceMetrics{{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := gzip.NewWriter(&body)
	_, _ = writer.Write(payload)
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytesReader(body.Bytes()))
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "gzip")
	res := httptest.NewRecorder()
	h.OTLP("metrics")(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
}
func TestOTLPPersistsBeforeAccepting(t *testing.T) {
	store := &recordingStore{}
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytesReader([]byte(`{"resourceMetrics":[]}`)))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.OTLP("metrics")(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if len(store.events) != 1 || store.events[0].Signal != "metrics" {
		t.Fatalf("stored events = %#v, want one metrics event", store.events)
	}
}

func TestOTLPReturnsUnavailableWhenPersistenceFails(t *testing.T) {
	store := &recordingStore{err: errors.New("ClickHouse unavailable")}
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/metrics", bytesReader([]byte(`{"resourceMetrics":[]}`)))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	h.OTLP("metrics")(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
}

func TestOTLPRejectsInvalidPayload(t *testing.T) {
	h := NewHandler()
	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytesReader([]byte("not protobuf")))
	req.Header.Set("Content-Type", "application/x-protobuf")
	res := httptest.NewRecorder()
	h.OTLP("logs")(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
}

func TestQANCollectPersistsBeforeAccepting(t *testing.T) {
	store := &recordingStore{}
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/qan/collect", bytesReader([]byte(`{"buckets":[{"query_id":"q1","fingerprint":"select ?","service_name":"svc","service_type":"mysql","agent_id":"agent","agent_type":"mysql-fixture","period_start_unix_secs":1720000000,"period_length_secs":60,"num_queries":1}]}`)))
	res := httptest.NewRecorder()
	h.QANCollect(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusAccepted)
	}
	if len(store.qanBuckets) != 1 || store.qanBuckets[0].QueryID != "q1" {
		t.Fatalf("stored QAN buckets = %#v, want one q1 bucket", store.qanBuckets)
	}
	if h.accepted.Load() != 1 || h.rejected.Load() != 0 {
		t.Fatalf("counters accepted=%d rejected=%d, want 1/0", h.accepted.Load(), h.rejected.Load())
	}
}

func TestQANCollectRejectsMethod(t *testing.T) {
	h := NewHandler(&recordingStore{})
	req := httptest.NewRequest(http.MethodGet, "/v1/qan/collect", nil)
	res := httptest.NewRecorder()
	h.QANCollect(res, req)
	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusMethodNotAllowed)
	}
}

func TestQANCollectRejectsInvalidBatchWithoutPersistence(t *testing.T) {
	store := &recordingStore{}
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/qan/collect", bytesReader([]byte(`{"buckets":[{"fingerprint":"select ?"}]}`)))
	res := httptest.NewRecorder()
	h.QANCollect(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadRequest)
	}
	if len(store.qanBuckets) != 0 {
		t.Fatalf("stored QAN buckets = %#v, want none", store.qanBuckets)
	}
	if h.accepted.Load() != 0 || h.rejected.Load() != 1 {
		t.Fatalf("counters accepted=%d rejected=%d, want 0/1", h.accepted.Load(), h.rejected.Load())
	}
}

func TestQANCollectReturnsUnavailableWhenPersistenceFails(t *testing.T) {
	store := &recordingStore{qanErr: errors.New("ClickHouse unavailable")}
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/qan/collect", bytesReader([]byte(`{"buckets":[{"query_id":"q1","fingerprint":"select ?","service_name":"svc","service_type":"mysql","agent_id":"agent","agent_type":"mysql-fixture","period_start_unix_secs":1720000000,"period_length_secs":60,"num_queries":1}]}`)))
	res := httptest.NewRecorder()
	h.QANCollect(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
	if h.accepted.Load() != 0 || h.rejected.Load() != 1 {
		t.Fatalf("counters accepted=%d rejected=%d, want 0/1", h.accepted.Load(), h.rejected.Load())
	}
}

func bytesReader(body []byte) *bytes.Reader { return bytes.NewReader(body) }
