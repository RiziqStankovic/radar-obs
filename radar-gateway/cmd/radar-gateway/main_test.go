package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type startupStore struct {
	ensureErr       error
	ensureQANErr    error
	verifyQANErr    error
	verifyMetricErr error
}

func (s startupStore) EnsureSchema(context.Context) error       { return s.ensureErr }
func (s startupStore) EnsureQANSchema(context.Context) error    { return s.ensureQANErr }
func (s startupStore) VerifyQANTable(context.Context) error     { return s.verifyQANErr }
func (s startupStore) VerifyMetricTables(context.Context) error { return s.verifyMetricErr }

func TestInitializeClickHouseFailsOnQANSchemaError(t *testing.T) {
	err := initializeClickHouse(context.Background(), startupStore{ensureQANErr: errors.New("boom")})
	if err == nil || !strings.Contains(err.Error(), "initialize ClickHouse QAN schema") {
		t.Fatalf("error = %v, want QAN schema error", err)
	}
}

func TestInitializeClickHouseFailsOnQANVerifyError(t *testing.T) {
	err := initializeClickHouse(context.Background(), startupStore{verifyQANErr: errors.New("missing")})
	if err == nil || !strings.Contains(err.Error(), "verify ClickHouse QAN table") {
		t.Fatalf("error = %v, want QAN verify error", err)
	}
}
