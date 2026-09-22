package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type CollectorConfig struct {
	Node                     bool
	OTLP                     bool
	Cluster                  bool
	Database                 bool
	QAN                      bool
	QANDisableExamples       bool
	QANPostgresEnabled       bool
	QANPostgresDSN           string
	QANPostgresServiceName   string
	QANPostgresAgentID       string
	QANPostgresInterval      time.Duration
	QANPostgresQueryExamples bool
}

type Config struct {
	Port            string
	OTLPHTTPPort    string
	GatewayEndpoint string
	ClusterID       string
	TenantID        string
	Collectors      CollectorConfig
}

func Load() Config {
	return Config{
		Port:            env("RADAR_AGENT_PORT", "8080"),
		OTLPHTTPPort:    env("RADAR_OTLP_HTTP_PORT", "4318"),
		GatewayEndpoint: os.Getenv("RADAR_GATEWAY_ENDPOINT"),
		ClusterID:       os.Getenv("RADAR_CLUSTER_ID"),
		TenantID:        os.Getenv("RADAR_TENANT_ID"),
		Collectors: CollectorConfig{
			Node:                     enabled("RADAR_COLLECTOR_NODE_ENABLED", true),
			OTLP:                     enabled("RADAR_COLLECTOR_OTLP_ENABLED", true),
			Cluster:                  enabled("RADAR_COLLECTOR_CLUSTER_ENABLED", false),
			Database:                 enabled("RADAR_COLLECTOR_DATABASE_ENABLED", false),
			QAN:                      enabled("RADAR_COLLECTOR_QAN_ENABLED", false),
			QANDisableExamples:       enabled("RADAR_QAN_DISABLE_QUERY_EXAMPLES", false),
			QANPostgresEnabled:       enabled("RADAR_QAN_POSTGRES_ENABLED", false),
			QANPostgresDSN:           os.Getenv("RADAR_QAN_POSTGRES_DSN"),
			QANPostgresServiceName:   env("RADAR_QAN_POSTGRES_SERVICE_NAME", "postgresql"),
			QANPostgresAgentID:       env("RADAR_QAN_POSTGRES_AGENT_ID", "radar-agent"),
			QANPostgresInterval:      duration("RADAR_QAN_POSTGRES_INTERVAL", time.Minute),
			QANPostgresQueryExamples: enabled("RADAR_QAN_POSTGRES_QUERY_EXAMPLES", false),
		},
	}
}

func (c Config) Validate() error {
	if c.Collectors.QAN && !c.Collectors.Database {
		return fmt.Errorf("QAN collector requires database collector")
	}
	if c.Collectors.QANPostgresEnabled && !c.Collectors.QAN {
		return fmt.Errorf("PostgreSQL QAN collector requires QAN collector")
	}
	if c.Collectors.QANPostgresEnabled && c.Collectors.QANPostgresDSN == "" {
		return fmt.Errorf("PostgreSQL QAN collector requires RADAR_QAN_POSTGRES_DSN")
	}
	return nil
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
	if err != nil {
		return fallback
	}
	return parsed
}

func duration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
