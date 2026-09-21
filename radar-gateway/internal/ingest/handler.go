package ingest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
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
