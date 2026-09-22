# Radar Gateway

`radar-gateway` is the production Go ingestion and control-plane runtime. It accepts OTLP HTTP traces, metrics, and logs, Radar events, and QAN batches, then persists accepted data to ClickHouse.

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
- `POST /v1/qan/collect` (internal Query Analytics JSON batches)

The default listener is `4318`; override it with `RADAR_GATEWAY_PORT`. The local production-path examples use `4319` so the gateway can run alongside other local services.

## ClickHouse persistence

Set `RADAR_CLICKHOUSE_URL` to the ClickHouse HTTP endpoint, for example `http://localhost:8123`. On startup, the gateway creates the `radar_events` and `qan_metrics` tables if they do not exist, and verifies the required OTLP metric tables. For the current QAN path, use `RADAR_CLICKHOUSE_DATABASE=radar_metrics`; this value must contain exactly one database name. Comma-separated values such as `radar_logs,radar_metrics,radar_traces` are not supported by the current single-store implementation. Use `RADAR_CLICKHOUSE_USER` and `RADAR_CLICKHOUSE_PASSWORD` when authentication is enabled. Keep the password in a local secret/environment manager, never in Git.

```powershell
$env:RADAR_CLICKHOUSE_URL = "http://localhost:8123"
$env:RADAR_CLICKHOUSE_DATABASE = "radar_metrics"
$env:RADAR_CLICKHOUSE_USER = "default"
$env:RADAR_CLICKHOUSE_PASSWORD = "<your-password>"
$env:RADAR_GATEWAY_PORT = "4319"
go run ./cmd/radar-gateway
```

When ClickHouse persistence is enabled, an event or QAN batch is only acknowledged after the insert succeeds. If ClickHouse is unavailable, ingestion returns `503` instead of silently losing the data.

Verify persisted events with ClickHouse HTTP:

```powershell
curl "http://localhost:8123/?query=SELECT%20received_at%2Csignal%2Csource%2Cpayload%20FROM%20radar_events%20ORDER%20BY%20received_at%20DESC%20LIMIT%201%20FORMAT%20PrettyCompact"
```

For local development on the same machine as `radar-agent`, run the gateway on another port, for example `RADAR_GATEWAY_PORT=4319`, then start the agent with `RADAR_GATEWAY_ENDPOINT=http://localhost:4319`. Set `RADAR_CLICKHOUSE_URL` in the gateway process so events forwarded by the agent are stored.

## Query Analytics ingestion

`POST /v1/qan/collect` accepts aggregate Query Analytics buckets. The endpoint rejects empty batches and buckets missing required identity, dimension, period, or aggregate fields. Valid batches are inserted into the dedicated `radar_metrics.qan_metrics` ClickHouse table, not `radar_events.payload`.

Minimal request example, matching `radar-obs/qan-fixtures/collect_request.json`:

```bash
curl -i -X POST http://localhost:4319/v1/qan/collect \
  -H "Content-Type: application/json" \
  --data-binary @../qan-fixtures/collect_request.json
```

Required bucket fields are `query_id`, `fingerprint`, `service_name`, `service_type`, `agent_id`, `agent_type`, `period_start_unix_secs`, `period_length_secs`, and `num_queries`.

Verify recent QAN rows with ClickHouse HTTP:

```powershell
curl "http://localhost:8123/?database=radar_metrics&query=SELECT%20period_start%2Cquery_id%2Cservice_name%2Cfingerprint%2Cnum_queries%20FROM%20qan_metrics%20ORDER%20BY%20received_at%20DESC%20LIMIT%205%20FORMAT%20PrettyCompact"
```
