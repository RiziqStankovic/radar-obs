package radarqanexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/xexporter"
	"go.opentelemetry.io/collector/pdata/plog"
)

const typeStr = "radarqan"

type Config struct {
	Endpoint string        `mapstructure:"endpoint"`
	APIKey   string        `mapstructure:"api_key"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

func createDefaultConfig() component.Config {
	return &Config{Timeout: 10 * time.Second}
}

func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		xexporter.WithLogs(createLogs, component.StabilityLevelAlpha),
	)
}

type exporterImpl struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

func createLogs(_ context.Context, _ exporter.Settings, cfg component.Config) (exporter.Logs, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid radarqan config type %T", cfg)
	}
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, fmt.Errorf("radarqan endpoint is required")
	}
	endpoint := strings.TrimRight(config.Endpoint, "/") + "/v1/qan/collect"
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &exporterImpl{endpoint: endpoint, apiKey: config.APIKey, client: &http.Client{Timeout: timeout}}, nil
}

func (e *exporterImpl) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *exporterImpl) Start(context.Context, component.Host) error { return nil }

func (e *exporterImpl) Shutdown(context.Context) error { return nil }

func (e *exporterImpl) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		resourceLogs := logs.ResourceLogs().At(i)
		for j := 0; j < resourceLogs.ScopeLogs().Len(); j++ {
			records := resourceLogs.ScopeLogs().At(j).LogRecords()
			for k := 0; k < records.Len(); k++ {
				body := records.At(k).Body().Str()
				if strings.TrimSpace(body) == "" {
					continue
				}
				request, err := normalize(body)
				if err != nil {
					return err
				}
				payload, err := json.Marshal(request)
				if err != nil {
					return fmt.Errorf("marshal QAN request: %w", err)
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, strings.NewReader(string(payload)))
				if err != nil {
					return err
				}
				req.Header.Set("Content-Type", "application/json")
				if e.apiKey != "" {
					req.Header.Set("Authorization", "Bearer "+e.apiKey)
				}
				resp, err := e.client.Do(req)
				if err != nil {
					return fmt.Errorf("publish QAN: %w", err)
				}
				_ = resp.Body.Close()
				if resp.StatusCode/100 != 2 {
					return fmt.Errorf("QAN gateway returned %s", resp.Status)
				}
			}
		}
	}
	return nil
}

func normalize(body string) (map[string]any, error) {
	var bucket map[string]any
	if err := json.Unmarshal([]byte(body), &bucket); err != nil {
		return nil, fmt.Errorf("decode QAN bucket: %w", err)
	}
	return map[string]any{"buckets": []any{bucket}}, nil
}

var _ exporter.Logs = (*exporterImpl)(nil)
