# Wayfinder map: Radar Agent production readiness

## Destination

Make the way to a production-installable `radar-agent` clear: decide the runtime boundary, image/build strategy, Helm contract, ownership model, collector activation path, and gateway/storage integration needed before implementation can safely proceed.

## Notes

- Local markdown tracker; child tickets live under `issues/`.
- This map expands beyond `.scratch/otel-collector-migration`, which intentionally excluded Kubernetes/Helm packaging and full database/QAN implementation.
- Compatibility authority for raw OTLP storage remains `cloudfren-homelab/observability-schema-raw.sql` and `cloudfren-homelab/prod/workload/radar-otel-collector/`, but the selected production path is `radar-agent -> radar-gateway -> ClickHouse`.
- Prefer OpenTelemetry Collector Contrib/OCB components over custom OTLP decoding, metric mapping, and direct ClickHouse persistence.
- Use `radar-agent` for the per-node customer-cluster DaemonSet runtime and the Go `radar-gateway` service for central auth/enrichment/routing/control-plane plus ClickHouse writes.
- This is planning-first. Do not implement the whole destination from inside one ticket; resolve decisions and define implementation slices.

## Decisions so far

- [Production runtime boundary](issues/01-production-runtime-boundary.md): production `radar-agent` is an OCB-generated OpenTelemetry Collector binary built from `radar-agent/builder-config.yaml`; the current Go HTTP shim is prototype/dev code, not the production runtime.
- [Gateway and storage integration path](issues/02-gateway-storage-integration-path.md): production ingestion path is `radar-agent -> radar-gateway -> ClickHouse`; production `radar-gateway` is the Go gateway service and owns auth, enrichment, routing, status ingestion, QAN, future domain modules, ClickHouse credentials, schema compatibility, retries/queues, and storage writes.
- [OCB build and image contract](issues/03-ocb-build-and-image-contract.md): production agent image is built by pinned OCB `v0.158.0` from `builder-config.yaml`, runs the generated `/radar-agent` binary, and currently supports only explicitly configured OTLP/host-metrics components.
- [Helm production install contract](issues/04-helm-production-install-contract.md): the chart owns the per-node DaemonSet, Collector ConfigMap, optional OTLP Service, ServiceAccount/RBAC, read-only host mount, health probes, and Secret references; ClickHouse credentials stay out of the agent chart and unsupported collectors must fail closed.
- [Ownership and leader election model](issues/05-ownership-and-leader-election-model.md): node/OTLP run on every DaemonSet pod; cluster, database, and QAN use separate Kubernetes Leases with 15s/10s/2s timing, QAN follows active database ownership, and standby/failover state is reported to the gateway.

## Not yet specified

- Exact rendered config and implementation behavior for each collector pipeline after ownership state is available.
- Exact status/heartbeat data model and ClickHouse schema compatibility strategy for the Go gateway direct-write responsibility.
- QAN schema and adapter contract after the database target model is decided.
- Rollout/migration order from current prototype images after the agent image/chart and Go gateway Dockerfile contracts are settled.

## Out of scope

- Full Datadog feature parity.
- Building a Datadog Cluster Agent clone unless a later decision changes the `radar-agent` single-DaemonSet destination.
- Replacing the existing ClickHouse OTLP schema for traces/metrics/logs.
