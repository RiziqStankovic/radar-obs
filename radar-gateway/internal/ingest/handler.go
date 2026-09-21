package ingest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"

	collectorlog "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Event struct {
	Signal  string         `json:"signal"`
	Source  string         `json:"source"`
	Payload map[string]any `json:"payload"`
}

type Handler struct {
	accepted atomic.Uint64
	rejected atomic.Uint64
}

func (h *Handler) OTLP(signal string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			h.reject(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<20))
		if err != nil {
			h.reject(w, http.StatusBadRequest, "request body is too large or unreadable")
			return
		}
		if err := decodeOTLP(signal, r.Header.Get("Content-Type"), body); err != nil {
			h.reject(w, http.StatusBadRequest, fmt.Sprintf("invalid OTLP %s payload: %v", signal, err))
			return
		}
		h.accepted.Add(1)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{})
	}
}

func decodeOTLP(signal, contentType string, body []byte) error {
	var message proto.Message
	switch signal {
	case "traces":
		message = &collectortrace.ExportTraceServiceRequest{}
	case "metrics":
		message = &collectormetrics.ExportMetricsServiceRequest{}
	case "logs":
		message = &collectorlog.ExportLogsServiceRequest{}
	default:
		return fmt.Errorf("unsupported signal")
	}
	if len(body) == 0 {
		return fmt.Errorf("empty payload")
	}
	if strings.Contains(strings.ToLower(contentType), "json") {
		return protojson.Unmarshal(body, message)
	}
	return proto.Unmarshal(body, message)
}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.reject(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var event Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.reject(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if event.Signal == "" || event.Source == "" {
		h.reject(w, http.StatusBadRequest, "signal and source are required")
		return
	}
	h.accepted.Add(1)
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("accepted\n"))
}

func (h *Handler) Metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = fmt.Fprintf(w, "# TYPE radar_gateway_events_accepted_total counter\nradar_gateway_events_accepted_total %d\n# TYPE radar_gateway_events_rejected_total counter\nradar_gateway_events_rejected_total %d\n", h.accepted.Load(), h.rejected.Load())
}

func (h *Handler) reject(w http.ResponseWriter, status int, message string) {
	h.rejected.Add(1)
	http.Error(w, message, status)
}
