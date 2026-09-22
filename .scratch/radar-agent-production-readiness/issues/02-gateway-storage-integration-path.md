# Gateway and storage integration path

Type: grilling
Status: resolved
Blocked by:

## Question

Should `radar-agent` send telemetry directly to the existing `radar-otel-collector`, through `radar-gateway`, or support both? Decide where auth, tenant/cluster enrichment, filtering, status ingestion, OTLP forwarding, and ClickHouse credentials belong.

## Answer

Decision: the production ingestion path is `radar-agent -> radar-gateway -> ClickHouse`, and the production `radar-gateway` runtime is the Go gateway service.

`radar-agent` forwards telemetry and Radar domain events to `radar-gateway`. `radar-agent` must not hold ClickHouse credentials or write directly to ClickHouse.

`radar-gateway` owns both the storage-facing boundary and the Radar control-plane/API boundary: management auth, agent/status APIs, tenant/cluster enrichment, filtering/routing, QAN ingestion, future Radar domain modules, ClickHouse credentials, schema compatibility, retries/queues, and writes to ClickHouse.

The existing `radar-otel-collector` and `observability-schema-raw.sql` remain compatibility references for raw OTLP traces, metrics, and logs, but `radar-otel-collector` is not part of the production path for this `radar-obs` topology unless a future decision reintroduces it as an internal implementation detail of the gateway.

Consequence: `radar-gateway/Dockerfile` must build and run the Go gateway service for production. Any OCB/Collector gateway artifacts are prototype/internal-writer artifacts until a future decision explicitly uses them.
