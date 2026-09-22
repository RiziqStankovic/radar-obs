# Radar Agent

`radar-agent` is the independently deployed, node-local OpenTelemetry Collector distribution. It receives OTLP and host metrics, then forwards logs, traces, and metrics to `radar-gateway`.

## Run

```powershell
.\scripts\build-collectors.ps1
.\scripts\run-agent.ps1
```

Default listeners:

- `:13133` for Collector health
- `:4317` for OTLP gRPC
- `:4318` for OTLP HTTP ingestion

Health endpoint on `:13133`:

- `GET /`

OTLP HTTP endpoints on `:4318`:

- `POST /v1/traces`
- `POST /v1/metrics`
- `POST /v1/logs`

Copy the repository `.env.example` to `.env`. Set `RADAR_GATEWAY_OTLP_ENDPOINT=http://127.0.0.1:4319` when running the local gateway alongside the agent. Agent and gateway are separate processes and have separate port/health configuration.

## Query Analytics

QAN is disabled by default. Enable it only with database collection enabled:

```powershell
$env:RADAR_COLLECTOR_DATABASE_ENABLED = "true"
$env:RADAR_COLLECTOR_QAN_ENABLED = "true"
$env:RADAR_GATEWAY_ENDPOINT = "http://localhost:4319"
go run ./cmd/radar-agent
```

If `RADAR_COLLECTOR_QAN_ENABLED=true` and `RADAR_COLLECTOR_DATABASE_ENABLED=false`, the agent fails startup with a configuration error because QAN depends on database collection context.

The Go prototype now supports a PostgreSQL `pg_stat_statements` source. It reads cumulative counters, emits interval deltas, and publishes non-empty buckets through `POST /v1/qan/collect`. The production OCB image remains separate and does not automatically gain this source.

Configure a local PostgreSQL source:

```powershell
$env:RADAR_COLLECTOR_DATABASE_ENABLED = "true"
$env:RADAR_COLLECTOR_QAN_ENABLED = "true"
$env:RADAR_QAN_POSTGRES_ENABLED = "true"
$env:RADAR_QAN_POSTGRES_DSN = "postgres://USER:PASSWORD@127.0.0.1:5432/DB?sslmode=disable"
$env:RADAR_QAN_POSTGRES_SERVICE_NAME = "postgres-local"
$env:RADAR_QAN_POSTGRES_INTERVAL = "60s"
$env:RADAR_QAN_POSTGRES_QUERY_EXAMPLES = "false"
$env:RADAR_GATEWAY_ENDPOINT = "http://localhost:4319"
go run ./cmd/radar-agent
```

The PostgreSQL client target should be `127.0.0.1:5432` or `localhost:5432`; `0.0.0.0` is a server bind address, not a client destination. Enable the extension on the PostgreSQL server:

```sql
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

Some installations also require `shared_preload_libraries = 'pg_stat_statements'` and a restart. The first collector snapshot establishes a baseline and emits no bucket; subsequent snapshots publish counter deltas. Query examples are disabled by default to avoid sending raw SQL values.

The current QAN implementation publishes a development fixture bucket through `POST /v1/qan/collect` only when running the Go prototype (`go run ./cmd/radar-agent`). The production OCB image does not yet contain a QAN/database receiver, so setting Helm `collectors.qan.enabled` alone must not be treated as enabling real QAN collection. The Helm chart now carries production-shaped target metadata and mounts database credentials from Kubernetes Secrets for the future adapter; concrete sources such as MySQL slowlog, MySQL performance schema, PostgreSQL statistics, or MongoDB profiler still need to be implemented in the production runtime.

Query examples can include sensitive data. Disable raw query examples while retaining fingerprints and aggregate metrics with:

```powershell
$env:RADAR_QAN_DISABLE_QUERY_EXAMPLES = "true"
```

## Production target contract

The Helm chart accepts database targets under `agent.collectors.database.targets` and QAN mappings under `agent.collectors.qan.targets`. Each database target requires `name`, `engine`, `endpoint`, and `existingSecret`; the Secret must provide the configured username and password keys. The chart renders target metadata to `/etc/radar-agent/qan-targets.yaml` and mounts credentials under `/etc/radar-agent/secrets/<target>`. ClickHouse credentials are intentionally not part of this chart; they remain owned by `radar-gateway`.

## Local QAN verification

1. Start `radar-gateway` with ClickHouse persistence configured and `RADAR_GATEWAY_PORT=4319`.
2. Start `radar-agent` with `RADAR_GATEWAY_ENDPOINT=http://localhost:4319`, `RADAR_COLLECTOR_DATABASE_ENABLED=true`, and `RADAR_COLLECTOR_QAN_ENABLED=true`.
3. Query ClickHouse for recent QAN rows:

```powershell
curl "http://localhost:8123/?database=default&query=SELECT%20period_start%2Cquery_id%2Cservice_name%2Cfingerprint%2Cnum_queries%20FROM%20qan_metrics%20ORDER%20BY%20received_at%20DESC%20LIMIT%205%20FORMAT%20PrettyCompact"
```
