# Wayfinder map: OTel Collector migration

## Destination

Replace custom Radar ingestion with locally runnable OpenTelemetry Collector Contrib distributions for `radar-agent` and `radar-gateway`, using the Contrib ClickHouse exporter and proving OTLP ingestion reaches the existing ClickHouse schema used by `radar-otel-collector`.

## Notes

- Local markdown tracker; child tickets live under `issues/`.
- Treat `observability-schema-raw.sql` and the existing `radar-otel-collector` deployment as compatibility authorities.
- Prefer existing OpenTelemetry Collector Contrib components over custom OTLP decoding, metric mapping, and ClickHouse persistence.
- The map is planning-first. Resolve decisions before implementing the migration.

## Decisions so far

<!-- Closed ticket links and one-line gists are appended here. -->

- [Schema and existing collector compatibility](issues/01-schema-and-existing-collector.md): Existing DDL matches Contrib ClickHouse exporter map mode; use native TCP 9000, three signal databases, and explicit `otel_metrics_exp_histogram` naming.
- [Distribution boundary for agent and gateway](issues/02-distribution-boundary.md): Agent and gateway are independently deployed services with separate runtime configuration and lifecycle; shared build components are allowed.
- [Local build and run workflow](issues/03-local-build-and-run.md): Pin both OCB distributions to Contrib v0.158.0, build independently without Docker, use separate OTLP/health ports, and configure native ClickHouse exporter through environment variables.

## Not yet specified

- End-to-end acceptance checks and rollback boundary from the current custom Go servers.
- How existing Radar auth, tenant/cluster enrichment, and future custom adapters fit into Collector pipelines.

## Out of scope

- Kubernetes/Helm packaging changes before the local Collector runtime is proven.
- Implementing database/QAN receivers beyond selecting their Collector components.
