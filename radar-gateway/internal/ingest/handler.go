package ingest

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	collectorlog "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"gitrepo.xlaxiata.id/radar/radar-gateway/internal/storage"
)

type EventStore interface {
	InsertEvent(context.Context, storage.Event) error
}

type QANStore interface {
	InsertQANBuckets(context.Context, []storage.QANBucket) error
}

type MetricStore interface {
	InsertMetricRows(context.Context, string, []map[string]any) error
}

type Event struct {
	Signal  string         `json:"signal"`
	Source  string         `json:"source"`
	Payload map[string]any `json:"payload"`
}

type Handler struct {
	accepted    atomic.Uint64
	rejected    atomic.Uint64
	store       EventStore
	metricStore MetricStore
	qanStore    QANStore
}

func (h *Handler) OTLP(signal string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			h.reject(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		reader := io.Reader(r.Body)
		var err error
		var gzipReader *gzip.Reader
		if strings.EqualFold(r.Header.Get("Content-Encoding"), "gzip") {
			gzipReader, err = gzip.NewReader(r.Body)
			if err != nil {
				h.reject(w, http.StatusBadRequest, "invalid gzip payload")
				return
			}
			defer gzipReader.Close()
			reader = gzipReader
		}
		body, err := io.ReadAll(io.LimitReader(reader, 32<<20))
		if err != nil {
			h.reject(w, http.StatusBadRequest, "request body is too large or unreadable")
			return
		}
		message, err := decodeOTLP(signal, r.Header.Get("Content-Type"), body)
		if err != nil {
			h.reject(w, http.StatusBadRequest, fmt.Sprintf("invalid OTLP %s payload: %v", signal, err))
			return
		}
		if signal == "metrics" && h.metricStore != nil {
			request := message.(*collectormetrics.ExportMetricsServiceRequest)
			for _, group := range mapMetrics(request) {
				started := time.Now()
				if err := h.metricStore.InsertMetricRows(r.Context(), group.table, group.rows); err != nil {
					log.Printf("metrics persistence failed route=otlp table=%s rows=%d: %v", group.table, len(group.rows), err)
					h.reject(w, http.StatusServiceUnavailable, "metrics persistence unavailable")
					return
				}
				log.Printf("clickhouse ingestion succeeded route=otlp signal=metrics table=%s rows=%d duration=%s", group.table, len(group.rows), time.Since(started))
			}
		}
		eventStarted := time.Now()
		if err := h.persist(r.Context(), Event{Signal: signal, Source: "otlp", Payload: map[string]any{
			"content_type": r.Header.Get("Content-Type"),
			"bytes":        len(body),
		}}); err != nil {
			log.Printf("event persistence failed route=otlp signal=%s: %v", signal, err)
			h.reject(w, http.StatusServiceUnavailable, "persistence unavailable")
			return
		}
		log.Printf("clickhouse ingestion succeeded route=otlp signal=%s table=radar_events rows=1 duration=%s", signal, time.Since(eventStarted))
		h.accepted.Add(1)
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte{})
	}
}

func decodeOTLP(signal, contentType string, body []byte) (proto.Message, error) {
	var message proto.Message
	switch signal {
	case "traces":
		message = &collectortrace.ExportTraceServiceRequest{}
	case "metrics":
		message = &collectormetrics.ExportMetricsServiceRequest{}
	case "logs":
		message = &collectorlog.ExportLogsServiceRequest{}
	default:
		return nil, fmt.Errorf("unsupported signal")
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty payload")
	}
	if strings.Contains(strings.ToLower(contentType), "json") {
		if err := protojson.Unmarshal(body, message); err != nil {
			return nil, err
		}
		return message, nil
	}
	if err := proto.Unmarshal(body, message); err != nil {
		return nil, err
	}
	return message, nil
}

func NewHandler(stores ...EventStore) *Handler {
	var store EventStore
	var metricStore MetricStore
	var qanStore QANStore
	if len(stores) > 0 {
		store = stores[0]
		metricStore, _ = stores[0].(MetricStore)
		qanStore, _ = stores[0].(QANStore)
	}
	return &Handler{store: store, metricStore: metricStore, qanStore: qanStore}
}

func (h *Handler) persist(ctx context.Context, event Event) error {
	if h.store == nil {
		return nil
	}
	return h.store.InsertEvent(ctx, storage.Event{Signal: event.Signal, Source: event.Source, Payload: event.Payload})
}

func (h *Handler) QANCollect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.reject(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var request QANCollectRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20))
	if err := decoder.Decode(&request); err != nil {
		log.Printf("QAN ingestion rejected route=qan reason=invalid-json: %v", err)
		h.reject(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := request.Validate(); err != nil {
		log.Printf("QAN ingestion rejected route=qan reason=validation error=%v", err)
		h.reject(w, http.StatusBadRequest, err.Error())
		return
	}
	started := time.Now()
	if h.qanStore != nil {
		if err := h.qanStore.InsertQANBuckets(r.Context(), request.Buckets); err != nil {
			log.Printf("QAN persistence failed route=qan buckets=%d: %v", len(request.Buckets), err)
			h.reject(w, http.StatusServiceUnavailable, "QAN persistence unavailable")
			return
		}
	}
	log.Printf("clickhouse ingestion succeeded route=qan table=qan_metrics buckets=%d duration=%s", len(request.Buckets), time.Since(started))
	h.accepted.Add(1)
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte("accepted\n"))
}

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
	if event.Source == "otlp" && h.metricStore != nil && event.Signal == "metrics" {
		if encoded, ok := event.Payload["body_base64"].(string); ok {
			body, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				h.reject(w, http.StatusBadRequest, "invalid OTLP body encoding")
				return
			}
			message, err := decodeOTLP("metrics", "application/json", body)
			if err != nil {
				h.reject(w, http.StatusBadRequest, fmt.Sprintf("invalid OTLP metrics payload: %v", err))
				return
			}
			for _, group := range mapMetrics(message.(*collectormetrics.ExportMetricsServiceRequest)) {
				started := time.Now()
				if err := h.metricStore.InsertMetricRows(r.Context(), group.table, group.rows); err != nil {
					log.Printf("metrics persistence failed route=radar-event table=%s rows=%d: %v", group.table, len(group.rows), err)
					h.reject(w, http.StatusServiceUnavailable, "metrics persistence unavailable")
					return
				}
				log.Printf("clickhouse ingestion succeeded route=radar-event signal=metrics table=%s rows=%d duration=%s", group.table, len(group.rows), time.Since(started))
			}
		}
	}
	eventStarted := time.Now()
	if err := h.persist(r.Context(), event); err != nil {
		log.Printf("event persistence failed route=radar-event signal=%s source=%s: %v", event.Signal, event.Source, err)
		h.reject(w, http.StatusServiceUnavailable, "persistence unavailable")
		return
	}
	log.Printf("clickhouse ingestion succeeded route=radar-event signal=%s table=radar_events rows=1 duration=%s", event.Signal, time.Since(eventStarted))
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
