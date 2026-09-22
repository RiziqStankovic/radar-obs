# Schema and existing collector compatibility

Type: research
Status: resolved

## Question

What exact ClickHouse exporter version, endpoint settings, database/table settings, and OTLP-to-ClickHouse schema behavior are required to match `cloudfren-homelab/observability-schema-raw.sql` and the existing `cloudfren-homelab/prod/workload/radar-otel-collector/` deployment?

Compare the local Contrib exporter source, the raw DDL, and the existing collector configuration. Record incompatibilities and the required compatibility contract before implementation.

## Answer

The existing ClickHouse schema is compatible with the Contrib ClickHouse exporter when configured in map-based mode. The compatibility contract is:

- Use the native ClickHouse protocol, `tcp://...:9000`, not HTTP `8123`.
- Route logs to `radar_logs.otel_logs`.
- Route traces to `radar_traces.otel_traces`.
- Route metrics to `radar_metrics.otel_metrics_gauge`, `otel_metrics_sum`, `otel_metrics_histogram`, `otel_metrics_summary`, and `otel_metrics_exp_histogram`.
- Explicitly configure the exporter table name `otel_metrics_exp_histogram`; its default is `otel_metrics_exponential_histogram`, which does not match this schema.
- Keep exporter JSON mode disabled; the DDL uses `Map(LowCardinality(String), String)` columns.
- Existing trace lookup tables/MVs and custom log/trace rollups remain schema-managed objects; exporter success alone does not prove those MVs.
- Resolve the version mismatch before implementation: local Contrib source is v0.161.0, existing image is v0.158.0, and chart dependency is v0.153.0. Select one reproducible runtime version.
- Replace any hard-coded ClickHouse password with the intended secret source.

The current custom gateway's HTTP ClickHouse path is not the target exporter path. End-to-end acceptance must prove native TCP writes for logs, traces, and each metric type, then query the target tables and relevant materialized views.
