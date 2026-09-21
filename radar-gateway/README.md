# Radar Gateway

`radar-gateway` is the centralized Radar ingestion runtime. This Phase 0 skeleton exposes health, readiness, and metrics endpoints; authentication, ingestion, normalization, deduplication, and ClickHouse persistence are added in later phases.

## Run

```bash
go run ./cmd/radar-gateway
```

Endpoints:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`
