# ADR 0002: Production gateway is Go control-plane writing to ClickHouse

## Status

Accepted

## Context

The desired Radar topology is:

```text
radar-agent -> radar-gateway -> ClickHouse
```

`radar-gateway` is not only a telemetry pipe. It must own management authentication, agent/status APIs, tenant and cluster enrichment, filtering/routing, QAN ingestion, and future Radar-specific modules. A pure OTel Collector gateway can write OTLP telemetry to ClickHouse, but it is not a natural home for those control-plane APIs and Radar domain modules.

The existing `cloudfren-homelab/prod/workload/radar-otel-collector/` deployment and `observability-schema-raw.sql` remain compatibility references for raw OTLP ClickHouse tables for traces, metrics, and logs.

## Decision

The production ingestion path is:

```text
radar-agent -> radar-gateway -> ClickHouse
```

The production `radar-gateway` runtime is the Go gateway service, not an OCB-generated Collector binary.

`radar-agent` forwards telemetry and Radar domain events to `radar-gateway`. It must not hold ClickHouse credentials and must not write directly to ClickHouse.

`radar-gateway` owns the storage-facing and control-plane boundary:

- management auth;
- agent/status APIs;
- tenant and cluster enrichment;
- filtering/routing;
- QAN ingestion;
- future Radar domain modules;
- ClickHouse credentials;
- schema compatibility;
- retries/queues;
- writes to ClickHouse.

OCB/Collector code may still be used by `radar-gateway` later as an internal implementation detail if it helps preserve OTLP-to-ClickHouse compatibility, but it is not the public production gateway runtime unless a future ADR changes this decision.

## Consequences

- `radar-gateway/Dockerfile` must build and run the Go gateway service for production, not the current OCB Collector binary.
- `radar-gateway/builder-config.yaml` and `radar-gateway/config/collector.yaml` are prototype/internal-writer artifacts until explicitly reintroduced.
- Gateway implementation must preserve compatibility with existing raw OTLP ClickHouse tables for traces, metrics, and logs, or explicitly define migrations.
- Gateway must own storage credentials; agent chart values must only reference gateway credentials/auth.
- Agent export configuration targets gateway endpoints only.
- Existing `radar-otel-collector` values/schema remain useful as test fixtures and compatibility authorities.
