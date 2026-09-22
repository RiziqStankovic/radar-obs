# Local build and run workflow

Type: task
Status: resolved
Blocked by: 02

## Question

What exact local commands and repository layout will generate and run the selected OTel Collector distribution(s) without Docker, including config files, environment loading, health endpoints, OTLP ports, and ClickHouse credentials?

The workflow is complete only when another developer can generate the binary, start it locally, send a representative OTLP payload, and inspect the process logs.

## Resolution

Use two independently generated OCB distributions pinned to Contrib `v0.158.0`, matching the existing `radar-otel-collector` image:

- `radar-agent/builder-config.yaml` -> `radar-agent/generated/radar-agent`
- `radar-gateway/builder-config.yaml` -> `radar-gateway/generated/radar-gateway`

The agent includes OTLP, hostmetrics, batching, memory limiting, healthcheck, and OTLP HTTP forwarding. The gateway includes OTLP, batching, memory limiting, healthcheck, and native `clickhouseexporter`.

Run `.\\scripts\\build-collectors.ps1` from the repository root. Run `.\\scripts\\run-gateway.ps1` and `.\\scripts\\run-agent.ps1` in separate terminals after copying `.env.example` to `.env`. Gateway uses OTLP HTTP `4319`, OTLP gRPC `4320`, health `13134`; agent uses OTLP HTTP `4318`, OTLP gRPC `4317`, health `13133`.

ClickHouse exporter configuration uses native `tcp://...:9000`, separate databases for logs/traces/metrics, and explicit `otel_metrics_exp_histogram` naming. Credentials are read from environment variables; no password is committed. The local run and query commands are documented in `LOCAL-DEVELOPMENT.md`.
