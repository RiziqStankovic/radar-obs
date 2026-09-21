package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"os/signal"
	"syscall"
	"time"

	"gitrepo.xlaxiata.id/radar/radar-agent/internal/collectors"
	"gitrepo.xlaxiata.id/radar/radar-agent/internal/config"
	"gitrepo.xlaxiata.id/radar/radar-agent/internal/gatewayclient"
	runtimeagent "gitrepo.xlaxiata.id/radar/radar-agent/internal/runtime"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	gateway := gatewayclient.New(cfg.GatewayEndpoint)
	modules := runtimeagent.New(
		runtimeagent.NewModule("node", cfg.Collectors.Node, true, gateway),
		runtimeagent.NewModule("cluster", cfg.Collectors.Cluster, false, gateway),
		runtimeagent.NewModule("database", cfg.Collectors.Database, false, gateway),
		runtimeagent.NewModule("qan", cfg.Collectors.QAN, false, gateway),
		runtimeagent.NewModule("otlp", cfg.Collectors.OTLP, true, gateway),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := modules.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer modules.Stop(context.Background())

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(modules.Status())
	})
	mux.HandleFunc("/metrics", metrics)
	mux.HandleFunc("/v1/traces", otlpIngress("traces", gateway, cfg.Collectors.OTLP))
	mux.HandleFunc("/v1/metrics", otlpIngress("metrics", gateway, cfg.Collectors.OTLP))
	mux.HandleFunc("/v1/logs", otlpIngress("logs", gateway, cfg.Collectors.OTLP))

	server := &http.Server{Addr: ":" + cfg.Port, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("radar-agent listening on :%s", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte("# TYPE radar_agent_info gauge\nradar_agent_info 1\n"))
}

func otlpIngress(signal string, sink collectors.Sink, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			http.Error(w, "OTLP collector disabled", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if len(payload) == 0 {
			http.Error(w, "empty request body", http.StatusBadRequest)
			return
		}
		if err := sink.Publish(r.Context(), collectors.Event{
			Signal:  signal,
			Source:  "otlp",
			Payload: map[string]any{"content_type": r.Header.Get("Content-Type"), "bytes": len(payload)},
		}); err != nil {
			http.Error(w, "gateway unavailable", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

var _ collectors.Sink = (*gatewayclient.Client)(nil)
