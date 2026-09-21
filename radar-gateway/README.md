# Radar Gateway

`radar-gateway` is the centralized Radar ingestion runtime. It accepts OTLP HTTP traces, metrics, and logs, plus the internal Radar event endpoint. Normalization, deduplication, and ClickHouse persistence remain later phases.

## Run

```bash
go run ./cmd/radar-gateway
```

Endpoints:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`
- `POST /v1/traces` (OTLP HTTP protobuf or JSON)
- `POST /v1/metrics` (OTLP HTTP protobuf or JSON)
- `POST /v1/logs` (OTLP HTTP protobuf or JSON)
- `POST /v1/radar/events` (internal Radar JSON events)

The default listener is `4318`; override it with `RADAR_GATEWAY_PORT`.
