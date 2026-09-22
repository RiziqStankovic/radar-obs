package config

import "testing"

func TestValidateRejectsQANWithoutDatabase(t *testing.T) {
	cfg := Config{Collectors: CollectorConfig{QAN: true, Database: false}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want QAN requires database error")
	}
}

func TestValidateAcceptsQANWithDatabase(t *testing.T) {
	cfg := Config{Collectors: CollectorConfig{QAN: true, Database: true}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
