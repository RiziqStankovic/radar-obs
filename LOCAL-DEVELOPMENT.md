# Local development: separate OTel Collector services

The development/production-path runtime is:

- `radar-agent`: OCB-generated node-local OpenTelemetry Collector, forwarding OTLP to the gateway.
- `radar-gateway`: Go ingestion/control-plane service with QAN and ClickHouse persistence.

The gateway's OCB Collector artifacts are not the production gateway runtime. The existing `radar-otel-collector` deployment and schema remain compatibility references for raw OTLP storage.

## 1. Configure environment

From the repository root:

```powershell
Copy-Item .env.example .env
notepad .env
```

Set `RADAR_CLICKHOUSE_ENDPOINT` to the native ClickHouse protocol, for example:

```text
RADAR_CLICKHOUSE_ENDPOINT=tcp://100.100.182.44:9000
```

Do **not** use HTTP `8123` for the Contrib exporter, and do not set a comma/semicolon-separated database value. The exporter has one database per instance:

```text
RADAR_CLICKHOUSE_LOGS_DATABASE=radar_logs
RADAR_CLICKHOUSE_TRACES_DATABASE=radar_traces
RADAR_CLICKHOUSE_METRICS_DATABASE=radar_metrics
```

The target tables are the existing schema tables:

- `radar_logs.otel_logs`
- `radar_traces.otel_traces`
- `radar_metrics.otel_metrics_gauge`
- `radar_metrics.otel_metrics_sum`
- `radar_metrics.otel_metrics_histogram`
- `radar_metrics.otel_metrics_summary`
- `radar_metrics.otel_metrics_exp_histogram`

The configs use `create_schema: false` because `observability-schema-raw.sql` is the schema authority. Apply/verify that DDL before starting the gateway.

## 2. Build the Radar Agent Collector without Docker

Requires Go and network access to the Go module proxy:

```powershell
.\scripts\build-collectors.ps1
```

This installs OCB `v0.158.0`, matching the deployed `radar-otel-collector`, and generates the `radar-agent` binary under `radar-agent/generated`. The Go gateway is run with `go run` locally and is built by `radar-gateway/Dockerfile` for deployment.

## 3. Start the Go gateway first

Terminal 1:

```powershell
.\scripts\run-gateway.ps1
```

The Go gateway listens on `RADAR_GATEWAY_PORT` (use `4319` locally) and exposes:

- health: `http://127.0.0.1:4319/healthz`
- readiness: `http://127.0.0.1:4319/readyz`
- OTLP HTTP: `http://127.0.0.1:4319/v1/{metrics,traces,logs}`
- QAN: `http://127.0.0.1:4319/v1/qan/collect`

It verifies ClickHouse connectivity and required tables during startup.

## 4. Start the agent separately

Terminal 2:

```powershell
.\scripts\run-agent.ps1
```

Agent endpoints:

- OTLP HTTP: `http://127.0.0.1:4318/v1/{metrics,traces,logs}`
- OTLP gRPC: `127.0.0.1:4317`
- health: `http://127.0.0.1:13133`
- Collector telemetry: `127.0.0.1:8889`

The agent forwards all three signals to the gateway at `RADAR_GATEWAY_OTLP_ENDPOINT`.

## 5. Send representative OTLP

Use the existing example for a quick metric request, or send OTLP JSON to the agent:

```powershell
Invoke-WebRequest `
  -Uri http://127.0.0.1:4318/v1/metrics `
  -Method Post `
  -ContentType 'application/json' `
  -Body (Get-Content .\examples\otel-metrics.json -Raw)
```

For a direct gateway check, change the URL to `http://127.0.0.1:4319/v1/metrics`. The same pattern applies to `/v1/traces` and `/v1/logs` with valid OTLP JSON payloads.

## 6. Verify ClickHouse persistence

Use HTTP `8123` only for queries:

```powershell
curl.exe 'http://100.100.182.44:8123/?query=SELECT%20count()%20FROM%20radar_metrics.otel_metrics_gauge%20FORMAT%20TabSeparated'
curl.exe 'http://100.100.182.44:8123/?query=SELECT%20count()%20FROM%20radar_traces.otel_traces%20FORMAT%20TabSeparated'
curl.exe 'http://100.100.182.44:8123/?query=SELECT%20count()%20FROM%20radar_logs.otel_logs%20FORMAT%20TabSeparated'
```

Then inspect recent rows:

```sql
SELECT MetricName, Timestamp, Value
FROM radar_metrics.otel_metrics_gauge
ORDER BY Timestamp DESC
LIMIT 10;
```

For the QAN path, run the deterministic smoke test:

```powershell
.\\scripts\\test-qan-clickhouse.ps1
```

It starts the Go gateway, posts `qan-fixtures/collect_request.json`, verifies `radar_metrics.qan_metrics`, and asserts a persisted fixture row. If gateway logs show `connection refused`, `401`, `UNKNOWN_DATABASE`, or repeated errors, first check endpoint protocol, credentials, and that `radar_metrics` plus its existing `otel_metrics_*` tables exist. `RADAR_CLICKHOUSE_DATABASE=radar_logs,radar_metrics,radar_traces` is invalid.
