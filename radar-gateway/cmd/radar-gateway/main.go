package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gitrepo.xlaxiata.id/radar/radar-gateway/internal/ingest"
)

func main() {
	port := os.Getenv("RADAR_GATEWAY_PORT")
	if port == "" {
		port = "4318"
	}
	handler := ingest.NewHandler()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/readyz", health)
	mux.HandleFunc("/metrics", handler.Metrics)
	mux.HandleFunc("/v1/radar/events", handler.Events)
	mux.HandleFunc("/v1/traces", handler.OTLP("traces"))
	mux.HandleFunc("/v1/metrics", handler.OTLP("metrics"))
	mux.HandleFunc("/v1/logs", handler.OTLP("logs"))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	server := &http.Server{Addr: ":" + port, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("radar-gateway listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}
