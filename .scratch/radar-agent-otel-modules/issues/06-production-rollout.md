Status: resolved
Type: task
Blocked by: 02, 03, 04

# Production rollout and rollback boundary

## Question

What build, image, Helm, health/readiness, migration, and rollback steps are required to replace the Go QAN prototype with the single custom OTel Collector image without disrupting the working OTLP logs/traces/metrics pipelines?

## Answer

Roll out in four controlled phases.

### Phase 1 — Build and unit validation

- Keep OCB/Contrib pinned to `v0.158.0`.
- Add the local OTel module containing `postgresqanreceiver` and `radarqanexporter` to `radar-agent/builder-config.yaml`.
- Build one custom OTel Collector image containing the existing OTLP, host, Kubernetes, database-metrics, and QAN components.
- Run receiver/exporter unit tests, gateway contract tests, Helm template validation, and a generated-collector config validation before publishing the image.
- Preserve the existing OTLP exporter/gateway endpoints and ClickHouse database split unchanged.

### Phase 2 — Non-disruptive verification

- Deploy the new image with `qan.enabled=false` first; verify OTLP logs, traces, metrics, host metrics, and cluster capability behavior against the existing acceptance checks.
- Enable QAN only for a dedicated non-production PostgreSQL profile or a single controlled cluster.
- Verify PostgreSQL baseline, delta collection, privacy policy, exporter retries, gateway `202`, and rows in `radar_metrics.qan_metrics`.
- Confirm standby pods do not poll or publish and that Lease failover works.
- Keep the existing Go QAN prototype disabled during the same target window to prevent duplicate QAN rows.

### Phase 3 — Production canary and cutover

- Publish an immutable image tag, never rely on `latest` for the cutover.
- Update the Helm values with simple customer-facing flags and optional database profiles/discovery; keep ownership and Lease names internal defaults.
- Start with one cluster or a controlled percentage of DaemonSet pods using a canary release strategy.
- Compare OTLP ingestion counts, QAN bucket counts, exporter error/retry metrics, gateway status, and ClickHouse rows.
- Enable QAN by default only after the canary has passed; do not run the Go QAN shim and OTel QAN receiver concurrently for the same target.

### Phase 4 — Rollback

- Roll back by restoring the previous immutable image tag and Helm values; OTLP pipelines remain available through the previous image.
- If only QAN is unhealthy, set `qan.enabled=false` while keeping `otlp`, node, cluster, and database metrics enabled.
- If a bad QAN deployment may have created duplicates, use the deterministic batch identity/idempotency mechanism and inspect the gateway/ClickHouse duplicate policy before re-enabling.
- Preserve the Go QAN prototype as a development fallback, not as a simultaneous production collector.

### Runtime health contract

- Collector health endpoint remains the existing health check endpoint.
- Readiness is true only when the Collector process is running and enabled capabilities have valid configuration; a standby capability is healthy/ready as standby, not failed.
- QAN exposes collection/export success, retry, failure, queue, last-success, and active/standby telemetry.
- A temporary PostgreSQL outage must not make OTLP pipelines unavailable; QAN reports degraded capability state and retries independently.

### Configuration boundary

- `values.yaml` owns enablement flags, portable discovery settings, external database profiles, intervals, privacy policy, and gateway base endpoint.
- Kubernetes Secrets are the production credential source; temporary plaintext bootstrap is compatibility-only and must not be rendered into collector ConfigMaps or logs.
- OTel traffic continues to the gateway OTLP endpoint; QAN traffic goes through the dedicated exporter to `POST /v1/qan/collect`.
- ClickHouse credentials and schema ownership remain in `radar-gateway`.
