Status: resolved
Type: grilling
Blocked by: none

# Helm target and Secret contract

## Question

What exact `values.yaml` contract should represent database targets and QAN targets? Decide whether QAN references a database target, which fields are YAML metadata, which fields come only from Kubernetes Secrets, and how PostgreSQL DSNs are constructed without exposing credentials.

## Answer

Use a reusable customer-facing contract that works across Kubernetes clusters:

```yaml
agent:
  collectors:
    node:
      enabled: true
    otlp:
      enabled: true
    cluster:
      enabled: true
    database:
      enabled: true
      discovery:
        enabled: true
        selector:
          radar.xlaxiata.id/monitor: "true"
      profiles: []
    qan:
      enabled: true
      includeQueryExamples: false
```

For in-cluster databases, the platform may discover annotated/labeled Kubernetes Services. For external databases, customers provide reusable connection profiles:

```yaml
profiles:
  - name: postgres-main
    engine: postgresql
    endpoint: postgres.example.com:5432
    database: app
    secretRef: postgres-main
    sslMode: disable
```

The target profile contains connection metadata, never plaintext credentials in the normal production contract. Username/password are read from a Kubernetes Secret. For the initial development/production bootstrap, plaintext values may be temporarily accepted as a compatibility fallback, but the chart must not render them into logs or the generated collector ConfigMap and should warn that SecretRef is preferred.

QAN does not duplicate connection configuration or require a customer-facing `databaseTarget`. It automatically follows every enabled database profile whose engine supports QAN. QAN ownership is `follow-database`, not a separate independent lease.

Ownership is automatic and portable:

- node and OTLP capabilities run on every DaemonSet pod;
- cluster-wide capability uses one internal leader Lease;
- shared/external database profiles use automatic leader ownership;
- node-local profiles (explicit local scope or local endpoint) run on the matching node without a global Lease;
- QAN follows the ownership state of its database profile;
- active/standby/failover behavior is internal and should not require customer configuration.

Advanced ownership and lease names remain chart/runtime defaults, not normal customer settings. Discovery must use portable Kubernetes APIs and explicit profile fallback; it must not assume a specific node label or a proprietary cluster layout. The chart validates profile names, engines, endpoints, Secret references, and QAN eligibility before rendering.

The QAN exporter endpoint is configured at agent level and sends to the Go gateway QAN route, while ordinary OTel signals continue to use the gateway OTLP endpoint:

```text
OTLP   → gateway OTLP endpoint
QAN    → gateway /v1/qan/collect
```
