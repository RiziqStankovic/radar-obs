# Wayfinder map: Radar Agent single-image OTel modules

## Destination

Produce an implementation-ready design for one production `radar-agent` OTel Collector image containing four configurable capabilities: `otlp`, `cluster-wide`, host/database resource metrics, and PostgreSQL QAN. The design must preserve the working `radar-agent -> radar-gateway -> ClickHouse` path, keep logs/traces/metrics storage safe, define database/QAN targets in Helm YAML, keep credentials in Kubernetes Secrets, and route QAN through a dedicated exporter to `radar-gateway`.

## Notes

- Local markdown tracker; child tickets live under `issues/`.
- Read `CONTEXT.md` and relevant ADRs before resolving tickets.
- OCB/Contrib version is currently pinned to `v0.158.0`.
- Prefer built-in OTel modules for OTLP, host metrics, and Kubernetes metrics; custom code should be limited to PostgreSQL QAN receiver/exporter unless a decision proves otherwise.
- `radar-agent-chart/values.yaml` is the configuration contract: enablement flags and target metadata belong in YAML; passwords and usernames belong in referenced Kubernetes Secrets.
- The QAN exporter must send to Go `radar-gateway` `POST /v1/qan/collect`; ClickHouse credentials remain gateway-owned.
- This is planning-first. Resolve decisions before implementing the full destination.

## Decisions so far

- `issues/01-qan-data-seam.md` — QAN uses an OTel logs event seam: `postgresqanreceiver` emits structured JSON log bodies, and `radarqanexporter` converts them to the existing `CollectRequest` and posts to `/v1/qan/collect`; query identity stays out of metric labels.
- `issues/02-ocb-custom-component-layout.md` — Custom QAN receiver/exporter berada di module OTel terpisah dan di-include ke OCB v0.158.0 melalui local `path`; receiver menghasilkan logs, exporter mengonsumsi logs dan memakai helper retry/queue.
- `issues/03-helm-target-secret-contract.md` — Customer-facing Helm config stays reusable and simple: enable flags plus optional database profiles; in-cluster discovery is portable via Kubernetes labels/annotations, external databases use profiles, credentials prefer SecretRef, QAN automatically follows eligible database profiles and ownership.
- `issues/05-qan-exporter-delivery.md` — QAN exporter uses bounded at-least-once delivery with a dedicated queue, transient-only retry, empty-batch no-op, deterministic idempotency support, optional auth, and failure isolation from OTLP pipelines.
- `issues/06-production-rollout.md` — Roll out the single custom OTel image in build, non-disruptive verification, canary/cutover, and rollback phases; preserve OTLP endpoints, isolate QAN failures, use immutable tags, and disable the Go QAN shim during the same target window.
- `issues/04-built-in-collector-modules.md` — Use `k8sclusterreceiver` with cluster leader election, keep `hostmetrics`/`kubeletstats` node-local, use `postgresqlreceiver`/`mysqlreceiver` for database metrics under database ownership, and route ordinary signals via the gateway OTLP HTTP endpoint.

## Not yet specified

- How database targets map to host/database metrics versus QAN targets.
- How ownership/leader election is represented inside the single Collector process.

## Out of scope

- Replacing the existing ClickHouse schemas for OTel logs, traces, or metrics.
- Moving ClickHouse credentials into the agent image or Helm chart.
- Full Datadog feature parity.
- Implementing unrelated database engines before PostgreSQL QAN is production-ready.
