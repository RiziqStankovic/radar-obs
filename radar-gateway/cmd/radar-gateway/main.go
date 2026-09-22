package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gitrepo.xlaxiata.id/radar/radar-gateway/internal/ingest"
	"gitrepo.xlaxiata.id/radar/radar-gateway/internal/storage"
)

func main() {
	loadDotEnv()
	port := os.Getenv("RADAR_GATEWAY_PORT")
	if port == "" {
		port = "4318"
	}
	var eventStore ingest.EventStore
	clickhouseURL := os.Getenv("RADAR_CLICKHOUSE_URL")
	clickhouseDatabase := os.Getenv("RADAR_CLICKHOUSE_DATABASE")
	if clickhouseDatabase == "" {
		clickhouseDatabase = "default"
	}
	log.Printf("radar-gateway effective ClickHouse config url=%s database=%s user=%s", clickhouseURL, clickhouseDatabase, os.Getenv("RADAR_CLICKHOUSE_USER"))
	if clickhouseURL == "" {
		log.Fatal("RADAR_CLICKHOUSE_URL is required; refusing to start without ClickHouse persistence")
	}
	clickhouse, err := storage.NewClickHouse(
		clickhouseURL,
		clickhouseDatabase,
		os.Getenv("RADAR_CLICKHOUSE_USER"),
		os.Getenv("RADAR_CLICKHOUSE_PASSWORD"),
	)
	if err != nil {
		log.Fatal(err)
	}
	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := initializeClickHouse(startupCtx, clickhouse); err != nil {
		cancel()
		log.Fatal(err)
	}
	cancel()
	eventStore = clickhouse
	log.Printf("radar-gateway ClickHouse persistence enabled: %s database=%s", clickhouseURL, clickhouseDatabase)
	handler := ingest.NewHandler(eventStore)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/readyz", health)
	mux.HandleFunc("/metrics", handler.Metrics)
	mux.HandleFunc("/v1/radar/events", handler.Events)
	mux.HandleFunc("/v1/qan/collect", handler.QANCollect)
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

func initializeClickHouse(ctx context.Context, clickhouse interface {
	EnsureSchema(context.Context) error
	EnsureQANSchema(context.Context) error
	VerifyQANTable(context.Context) error
	VerifyMetricTables(context.Context) error
}) error {
	if err := clickhouse.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("initialize ClickHouse schema: %w", err)
	}
	if err := clickhouse.EnsureQANSchema(ctx); err != nil {
		return fmt.Errorf("initialize ClickHouse QAN schema: %w", err)
	}
	if err := clickhouse.VerifyQANTable(ctx); err != nil {
		return fmt.Errorf("verify ClickHouse QAN table: %w", err)
	}
	if err := clickhouse.VerifyMetricTables(ctx); err != nil {
		return fmt.Errorf("verify ClickHouse metric tables: %w", err)
	}
	return nil
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
			key, value, ok := splitEnvLine(line)
			if ok && os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
		return
	}
}

func splitEnvLine(line string) (string, string, bool) {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
	return key, value, key != ""
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}
