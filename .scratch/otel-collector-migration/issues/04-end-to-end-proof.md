# End-to-end ingestion proof

Type: prototype
Status: open
Blocked by: 01, 02, 03

## Question

What is the smallest deterministic end-to-end proof that a representative OTLP metrics payload sent through the local Radar agent/gateway Collector pipelines is persisted by the Contrib ClickHouse exporter into the existing `radar_metrics.otel_metrics_*` schema?

Define the payload, startup prerequisites, expected Collector logs, ClickHouse queries, and failure criteria. Include checks that distinguish Collector self-metrics from persisted application telemetry.
