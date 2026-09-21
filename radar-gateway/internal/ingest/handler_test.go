package ingest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

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

func bytesReader(body []byte) *bytes.Reader { return bytes.NewReader(body) }
