package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gitrepo.xlaxiata.id/radar/radar-agent/internal/collectors"
	"gitrepo.xlaxiata.id/radar/radar-agent/internal/gatewayclient"
	runtimeagent "gitrepo.xlaxiata.id/radar/radar-agent/internal/runtime"
)

func main() {
	port := env("RADAR_AGENT_PORT", "8080")
	gateway := gatewayclient.New(os.Getenv("RADAR_GATEWAY_ENDPOINT"))
	modules := runtimeagent.New(
		runtimeagent.NewModule("node", enabled("RADAR_COLLECTOR_NODE_ENABLED", true), true, gateway),
		runtimeagent.NewModule("cluster", enabled("RADAR_COLLECTOR_CLUSTER_ENABLED", true), false, gateway),
		runtimeagent.NewModule("database", enabled("RADAR_COLLECTOR_DATABASE_ENABLED", false), false, gateway),
		runtimeagent.NewModule("qan", enabled("RADAR_COLLECTOR_QAN_ENABLED", false), false, gateway),
		runtimeagent.NewModule("otlp", enabled("RADAR_COLLECTOR_OTLP_ENABLED", true), true, gateway),
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
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte("# TYPE radar_agent_info gauge\nradar_agent_info 1\n"))
	})

	server := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("radar-agent listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func enabled(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

var _ collectors.Sink = (*gatewayclient.Client)(nil)
