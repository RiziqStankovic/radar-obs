package config

import (
	"fmt"
	"os"
	"strconv"
)

type CollectorConfig struct {
	Node     bool
	OTLP     bool
	Cluster  bool
	Database bool
	QAN      bool
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
			Node:     enabled("RADAR_COLLECTOR_NODE_ENABLED", true),
			OTLP:     enabled("RADAR_COLLECTOR_OTLP_ENABLED", true),
			Cluster:  enabled("RADAR_COLLECTOR_CLUSTER_ENABLED", false),
			Database: enabled("RADAR_COLLECTOR_DATABASE_ENABLED", false),
			QAN:      enabled("RADAR_COLLECTOR_QAN_ENABLED", false),
		},
	}
}

func (c Config) Validate() error {
	if c.Collectors.QAN && !c.Collectors.Database {
		return fmt.Errorf("QAN collector requires database collector")
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
