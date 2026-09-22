# Radar Observability Context

## Glossary

- **Radar Agent**: The per-node customer-cluster observability runtime installed by `radar-agent-chart` as a Kubernetes DaemonSet. In production it is an OCB-generated OpenTelemetry Collector binary, not the legacy Go HTTP shim.
- **Radar Gateway**: The central Go ingestion/control-plane runtime responsible for management auth, agent/status APIs, tenant/cluster enrichment, filtering/routing, QAN ingestion, future Radar domain modules, ClickHouse credentials, schema compatibility, retries/queues, and writes to ClickHouse.
- **Radar OTel Collector**: The existing central OpenTelemetry Collector deployment used as a compatibility reference for raw OTLP traces, metrics, logs, and ClickHouse exporter behavior. It is not part of the selected production path unless reintroduced by a future decision.
- **Collector capability**: A user-facing Radar feature flag, such as node, OTLP, cluster, database, or QAN collection, that enables pipelines or adapters inside the Radar Agent process rather than creating a separate collector pod.
- **Production runtime**: The binary executed by the production container image. For Radar Agent, this means the OCB-generated Collector binary built from `builder-config.yaml`. For Radar Gateway, this means the Go gateway service.
- **Production ingestion path**: The selected storage path `radar-agent -> radar-gateway -> ClickHouse`.
- **Capability ownership**: The runtime responsibility for a non-node collector. Node and OTLP are node-local and active on every DaemonSet pod; cluster, database, and QAN are globally owned capabilities protected by separate Kubernetes Leases.
- **Capability standby**: The intentional non-owner state for a leader-elected capability. A standby pod reports status but does not poll sources or publish duplicate data.
- **QAN ownership context**: QAN collection is subordinate to its referenced database target; it may run only when the database collection context is active.
