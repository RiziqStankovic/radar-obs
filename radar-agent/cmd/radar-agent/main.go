package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"os/signal"
	"strings"
	"syscall"
	"time"

	"gitrepo.xlaxiata.id/radar/radar-agent/internal/collectors"
	"gitrepo.xlaxiata.id/radar/radar-agent/internal/config"
	"gitrepo.xlaxiata.id/radar/radar-agent/internal/gatewayclient"
	"gitrepo.xlaxiata.id/radar/radar-agent/internal/qan"
	runtimeagent "gitrepo.xlaxiata.id/radar/radar-agent/internal/runtime"
)

func main() {
	loadDotEnv()
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}

	gateway := gatewayclient.New(cfg.GatewayEndpoint)
	qanModule := runtimeagent.NewQANModule(cfg.Collectors.QAN, gateway, !cfg.Collectors.QANDisableExamples)
	if cfg.Collectors.QANPostgresEnabled {
		postgresCollector, err := qan.NewPostgresCollector(qan.PostgresConfig{
			DSN: cfg.Collectors.QANPostgresDSN, ServiceName: cfg.Collectors.QANPostgresServiceName,
			AgentID: cfg.Collectors.QANPostgresAgentID, Interval: cfg.Collectors.QANPostgresInterval,
			IncludeExamples: cfg.Collectors.QANPostgresQueryExamples && !cfg.Collectors.QANDisableExamples,
		})
		if err != nil {
			log.Fatal(err)
		}
		qanModule = runtimeagent.NewQANPostgresModule(cfg.Collectors.QAN, gateway, postgresCollector)
	}
	modules := runtimeagent.New(
		runtimeagent.NewModule("node", cfg.Collectors.Node, true, gateway),
		runtimeagent.NewModule("cluster", cfg.Collectors.Cluster, false, gateway),
		runtimeagent.NewModule("database", cfg.Collectors.Database, false, gateway),
		qanModule,
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
	healthServer := &http.Server{Addr: ":" + cfg.Port, Handler: mux}
	otlpMux := http.NewServeMux()
	otlpMux.HandleFunc("/v1/traces", otlpIngress("traces", gateway, cfg.Collectors.OTLP))
	otlpMux.HandleFunc("/v1/metrics", otlpIngress("metrics", gateway, cfg.Collectors.OTLP))
	otlpMux.HandleFunc("/v1/logs", otlpIngress("logs", gateway, cfg.Collectors.OTLP))
	otlpServer := &http.Server{Addr: ":" + cfg.OTLPHTTPPort, Handler: otlpMux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = healthServer.Shutdown(shutdownCtx)
		_ = otlpServer.Shutdown(shutdownCtx)
	}()

	go func() {
		log.Printf("radar-agent OTLP HTTP listening on :%s", cfg.OTLPHTTPPort)
		if err := otlpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("OTLP HTTP server stopped: %v", err)
		}
	}()
	log.Printf("radar-agent health server listening on :%s", cfg.Port)
	if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func loadDotEnv() {
	paths := []string{".env", "../.env"}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || line[0] == '#' {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
			if key != "" && os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
		return
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
			Signal: signal,
			Source: "otlp",
			Payload: map[string]any{
				"content_type": r.Header.Get("Content-Type"),
				"bytes":        len(payload),
				"body_base64":  base64.StdEncoding.EncodeToString(payload),
			},
		}); err != nil {
			log.Printf("OTLP %s forwarding failed: %v", signal, err)
			http.Error(w, "gateway unavailable: "+err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

var _ collectors.Sink = (*gatewayclient.Client)(nil)
