# Radar Agent

`radar-agent` is the per-node Radar collection runtime. This Phase 0 skeleton exposes health, readiness, and metrics endpoints; collector modules, ownership, queues, and gateway export are added in later phases.

## Run

```bash
go run ./cmd/radar-agent
```

Endpoints:

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`
