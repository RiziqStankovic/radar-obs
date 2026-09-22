# Helm production install contract

Type: grilling
Status: resolved
Blocked by: 01, 02, 03

## Question

What resources must `radar-agent-chart` render for a production install, and what must it explicitly not render? Decide the DaemonSet, Service, ConfigMap, Secret references, RBAC, host mounts, probes, NetworkPolicy/PodMonitor options, values API, and compatibility with the production image contract.

## Answer

The production Helm install contract is:

- Render one per-node `DaemonSet` for the OCB-generated `radar-agent` image. The container command must use `/etc/radar-agent/collector.yaml`, matching the image contract from [OCB build and image contract](03-ocb-build-and-image-contract.md).
- Render a `ConfigMap` containing the Collector configuration and non-secret target metadata. The ConfigMap must never contain database passwords, gateway API keys, or ClickHouse credentials.
- Render a `ServiceAccount`. When `rbac.create=true`, render a `ClusterRole` and `ClusterRoleBinding` with only the Kubernetes API permissions needed by enabled collectors and leader election. Lease permissions are required for future cluster/database/QAN ownership.
- Render the OTLP `Service` when `service.enabled=true`; expose gRPC, HTTP, and health ports. The template must honor the flag rather than rendering unconditionally.
- Use Secret references for gateway API authentication and database target credentials. For database targets, values contain only `existingSecret` and key names. ClickHouse credentials are not part of the agent chart and remain owned by `radar-gateway`.
- Mount `/` from the node at `/hostfs` read-only only when host metrics are enabled. Do not mount the Docker socket or arbitrary writable host paths.
- Use liveness and readiness probes against the Collector health endpoint. Probe configuration must match the configured health port/path and should not depend on gateway or ClickHouse availability.
- Keep NetworkPolicy and PodMonitor optional chart features; they are not prerequisites for the first install contract and must not silently block an install when disabled.
- The values API is grouped under `agent.image`, `agent.ports`, `agent.config`, `agent.collectors`, `agent.gateway`, `agent.resources`, scheduling fields, and `rbac`/`service`. Collector flags are declarative, but a flag may only be enabled when its component exists in `builder-config.yaml` and its pipeline/configuration is rendered.
- `node.kubeletMetrics`, `cluster`, `database`, and `qan` remain disabled by default until their OCB component or native adapter and ownership implementation are complete. Helm validation should fail closed rather than deploy a pod that advertises unsupported collection.

Required implementation follow-ups are: make `service.enabled` conditional, add values/schema validation for unsupported collector combinations, scope RBAC to enabled capabilities, and add a render test using `helm template`. This ticket defines the install boundary; it does not claim Kubernetes deployment or real QAN has already passed.
