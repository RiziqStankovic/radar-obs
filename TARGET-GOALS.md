# Radar Observability Agent — Target Goals

**Repository:** `radar-obs`

**Status:** Proposed

**Scope:** `radar-agent` as a single-image, Kubernetes-side collection runtime based on OpenTelemetry Collector Contrib.

## 1. Product goal

Build a Radar-branded observability agent that users install once in their Kubernetes cluster. The agent collects node, Kubernetes, application, database, and QAN telemetry, then forwards the collected data to `radar-gateway`.

The user experience must resemble a Datadog Agent installation:

```text
one Radar Agent image
one DaemonSet Pod per eligible node
one values/configuration surface
multiple collection capabilities inside the agent
```

Collectors are not separate Deployments, Pods, images, or Helm releases. They are OTel Collector pipelines and Radar adapters inside the same `radar-agent` image.

## 2. Target architecture

```text
Customer Kubernetes Cluster

Node A                         Node B                         Node C
└── radar-agent Pod             └── radar-agent Pod             └── radar-agent Pod
    └── radar-agent image           └── radar-agent image           └── radar-agent image
        ├── node pipeline               ├── node pipeline               ├── node pipeline
        ├── OTLP pipeline                ├── OTLP pipeline                ├── OTLP pipeline
        ├── cluster pipeline             ├── cluster pipeline             ├── cluster pipeline
        ├── database pipeline            ├── database pipeline            ├── database pipeline
        └── QAN pipeline                 └── QAN pipeline                 └── QAN pipeline
                    \__________________________  __________________________/
                                               \/
                                      radar-gateway
                                               ↓
                                          ClickHouse
```

The agent is a collect-and-forward runtime. Heavy normalization, tenant routing, deduplication, long-term storage, and ClickHouse writes belong to the gateway tier.

## 3. Runtime packaging goal

The final image must contain one Radar-customized OpenTelemetry Collector distribution:

```text
radar-agent image
├── radar-agent binary
├── OTel Collector Contrib receivers
├── OTel Collector Contrib processors
├── OTel Collector Contrib exporters
├── Radar-specific adapters/processors
└── Collector configuration
```

The image must not require separate collector images for:

- host metrics;
- kubelet metrics;
- pod/container logs;
- OTLP application telemetry;
- Kubernetes cluster data;
- database metrics;
- QAN collection.

The initial engine should be built with OpenTelemetry Collector Builder (OCB) using a curated component set. The full Contrib distribution should not be embedded blindly; only required components should be included to control image size, attack surface, and upgrade cost.

## 4. Collection capabilities

### 4.1 Node collection

The agent must collect node-local data on every eligible node:

- CPU and load;
- memory and swap;
- filesystem and disk;
- network;
- process metrics where supported;
- kubelet node, pod, and container metrics;
- container and pod logs when enabled.

Expected OTel components include:

```text
hostmetricsreceiver
kubeletstatsreceiver
filelogreceiver
k8sattributesprocessor
```

Node collection does not use leader election.

### 4.2 Application telemetry

The agent must receive telemetry from application Pods using OTLP:

```text
OTLP gRPC :4317
OTLP HTTP :4318
```

Supported signals:

- traces;
- metrics;
- logs.

The agent forwards application telemetry to the gateway after lightweight batching and resource enrichment.

Application auto-instrumentation is a producer-side concern. The OpenTelemetry Operator may inject SDKs into application Pods, but the operator is not part of the agent process and does not create a collector Pod per language.

### 4.3 Kubernetes cluster collection

The agent must support cluster-wide collection using Kubernetes ownership:

- Kubernetes events;
- workload inventory;
- Pods, Nodes, Services, and Namespaces;
- Deployments, StatefulSets, DaemonSets, Jobs, and related state.

Only one agent instance may actively collect duplicate-sensitive cluster-wide data at a time. Ownership must use Kubernetes `Lease` or an equivalent mechanism.

Expected OTel components include:

```text
k8sclusterreceiver
k8seventsreceiver
k8sobjectsreceiver
k8sleaderelector extension
```

### 4.4 Database collection

The agent must collect database telemetry from explicitly configured targets. Initial engine adapters should prioritize:

- PostgreSQL;
- MySQL;
- MongoDB;
- Redis.

Expected OTel components include:

```text
postgresqlreceiver
mysqlreceiver
mongodbreceiver
redisreceiver
```

Database targets must support:

- stable target identity;
- engine selection;
- endpoint and TLS settings;
- Kubernetes Secret references for credentials;
- collection interval;
- ownership mode;
- retry and timeout settings.

Credentials must never be placed directly in Git values or emitted in logs/telemetry.

### 4.5 QAN collection

QAN is a Radar-specific capability. Enabling QAN must activate a QAN pipeline or adapter inside `radar-agent`; it must not install PMM Server or create an additional QAN Pod by default.

```yaml
collectors:
  qan:
    enabled: true
```

means:

```text
enable QAN inside radar-agent
```

It does not mean:

```text
install PMM Server
```

QAN adapters will use engine-specific sources such as:

```text
PostgreSQL → pg_stat_statements
MySQL      → performance_schema / slow query data
MongoDB    → profiler / slow operations
Redis      → SLOWLOG / command statistics
```

The QAN contract must preserve at least:

- tenant/installation ID;
- cluster ID;
- database and target ID;
- database engine;
- query fingerprint;
- normalized query where policy permits;
- collection period;
- calls/execution count;
- latency statistics;
- rows examined/returned where available;
- error count where available.

PMM may be supported later as an optional external source adapter. It must not be a hidden dependency of `qan.enabled`.

## 5. Ownership model

The same `radar-agent` image runs on every eligible node, but not every pipeline has the same ownership semantics:

| Capability | Default ownership | Expected state |
|---|---|---|
| Node metrics/logs | Local | Active on every agent |
| Application OTLP | Local | Active on every agent |
| Cluster data | Leader | One active owner, others standby |
| Database targets | Leader initially | One active owner per target set |
| QAN targets | Leader initially | One active owner per target set |

A standby collector must not scrape, query, or buffer duplicate data while waiting for ownership. Ownership loss must stop collection before another owner proceeds.

Recommended Leases:

```text
radar-cluster-collector
radar-database-collector
radar-qan-collector
```

The exact leader Pod is not stable and must not be part of the user contract.

## 6. User configuration goal

The primary configuration must use Radar concepts rather than raw Collector YAML:

```yaml
global:
  clusterId: production-cluster
  tenantId: tenant-a

agent:
  enabled: true

  image:
    repository: ghcr.io/riziqstankovic/radar-agent
    tag: v0.1.0

  collectors:
    node:
      enabled: true
      hostMetrics: true
      kubeletMetrics: true
      logs: false

    otlp:
      enabled: true

    cluster:
      enabled: true
      ownership: leader
      leaseName: radar-cluster-collector

    database:
      enabled: true
      ownership: leader
      targets:
        - name: postgres-main
          engine: postgresql
          endpoint: postgres.database.svc.cluster.local:5432
          existingSecret: radar-postgres-monitoring

    qan:
      enabled: true
      ownership: leader
      targets:
        - name: postgres-main
          databaseTarget: postgres-main
          interval: 30s

  gateway:
    endpoint: https://ingest.radar.example.com
    existingSecret: radar-agent-auth
```

`enabled: true` means the runtime loads the corresponding pipeline/module in the existing `radar-agent` process. It does not create a new Pod, image, Service, or Helm release.

## 7. Chart goal

The initial `radar-agent-chart` is a packaging/configuration contract for the agent image. The chart must remain a single chart for the agent, not one chart per collector.

Current intended contents:

```text
radar-agent-chart/
├── Chart.yaml
└── values.yaml
```

If the chart is later made directly installable, its templates must render only the agent deployment resources:

- one `radar-agent` DaemonSet;
- one node-local OTLP/health Service;
- ServiceAccount;
- Kubernetes RBAC;
- Lease permissions;
- optional ConfigMap/Secret references;
- optional PodMonitor/NetworkPolicy.

It must not render a Pod or Service for every collector.

## 8. Gateway contract

The gateway is a separate Radar image and is outside the first agent-only implementation scope:

```text
radar-agent image
    ↓ OTLP/Radar ingestion
radar-gateway image
    ↓
ClickHouse
```

The agent must only perform lightweight collection, enrichment, batching, retry, and forwarding. It must not require ClickHouse credentials or direct ClickHouse access.

The agent-to-gateway protocol must eventually include:

- schema version;
- tenant ID;
- installation ID;
- cluster ID;
- agent/source ID;
- signal;
- observed timestamp;
- payload/resource attributes;
- idempotency identity where applicable.

## 9. Milestones

### M0 — OTel distribution foundation

- Build `radar-agent` with OCB.
- Include curated OTel Contrib components.
- Run one binary in one image.
- Load a valid Collector configuration.
- Expose health, OTLP, and Collector telemetry endpoints.

### M1 — Node and application telemetry

- Enable host metrics.
- Enable kubelet metrics.
- Enable optional file log collection.
- Enable OTLP traces, metrics, and logs.
- Add Kubernetes resource enrichment.
- Forward to a gateway-compatible OTLP endpoint.

### M2 — Kubernetes collection and ownership

- Add Kubernetes API collection.
- Add Lease acquisition/renewal/loss handling.
- Add cluster and event pipelines.
- Verify no duplicate cluster collection during normal operation.

### M3 — Database targets

- Add target configuration and Secret resolution.
- Add PostgreSQL and MySQL first.
- Add MongoDB and Redis after the target contract is stable.
- Add per-target health and ownership status.

### M4 — QAN

- Define Radar QAN envelope.
- Implement PostgreSQL QAN adapter.
- Implement MySQL QAN adapter.
- Add duplicate-safe period/fingerprint identity.
- Keep PMM optional and external.

### M5 — Operational hardening

- Add bounded queues and backpressure.
- Add retry and circuit breaking.
- Add resource limits and drop priorities.
- Add NetworkPolicy and minimal RBAC.
- Add rolling upgrade and failover tests.
- Publish versioned images and chart artifacts.

## 10. Acceptance criteria

The agent goal is met when:

- one `radar-agent` image contains the selected OTel Collector Contrib engine and Radar adapters;
- one `radar-agent` Pod runs per eligible node;
- enabling a collector does not create a new Pod or image;
- node and application telemetry is collected locally;
- OTLP traces, metrics, and logs can be forwarded to the gateway;
- cluster/database/QAN ownership prevents duplicate active collection;
- database credentials are read from Secret references;
- enabling QAN does not install PMM Server implicitly;
- the agent has no ClickHouse credentials or direct ClickHouse dependency;
- `/healthz`, `/readyz`, and Collector telemetry expose useful runtime status;
- image build and configuration validation run in CI;
- the chart values describe Radar concepts without requiring users to write raw Collector pipelines.

## 11. Explicit non-goals

The first agent release does not attempt to:

- provide full Datadog feature parity;
- install PMM Server as a side effect of QAN;
- write directly to ClickHouse;
- run one Pod per collector;
- expose every OTel Contrib component as a user-facing setting;
- guarantee identical QAN semantics across database engines without engine-specific adapters;
- hide database privilege requirements from the user.

## 12. Definition of success

A user should be able to install the Radar Agent once, configure collector flags and targets, and receive collected telemetry in Radar without managing separate collector Pods, PMM infrastructure, or raw OTel Collector deployments.

```text
install one agent
configure values
collect locally
forward to Radar
```
