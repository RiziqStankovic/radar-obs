# Distribution boundary for agent and gateway

Type: grilling
Status: resolved

## Question

Should `radar-agent` and `radar-gateway` be two separately generated OTel Collector distributions, or one generated distribution run with different configurations?

Decide the ownership boundary for shared components, Radar-specific code, configuration, binaries, and local commands. Include the migration boundary from the current custom Go servers.

## Answer

`radar-agent` and `radar-gateway` are two independently deployed services with separate runtime configuration, ports, health/readiness, credentials, scaling, and release lifecycle.

They may share OTel Collector Contrib component versions and build tooling, but they must not depend on one process, one deployment, or one configuration at runtime.

The deployment boundary is:

- `radar-agent`: node-local collection and OTLP forwarding to the gateway.
- `radar-gateway`: centralized OTLP ingestion, Radar enrichment/auth where needed, batching, and ClickHouse export.

For local development, each service must have an independently runnable command. Whether they are built from one shared Collector distribution or two named distributions remains an implementation choice, but the produced runtime configuration and deployment artifacts must remain separate.
