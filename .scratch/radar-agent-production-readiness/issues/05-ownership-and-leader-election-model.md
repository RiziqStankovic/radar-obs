# Ownership and leader election model

Type: grilling
Status: resolved
Blocked by: 01, 04

## Question

How should `radar-agent` prevent duplicate collection for cluster, database, and QAN collectors? Decide whether to use `k8sleaderelector`, separate Kubernetes Lease names, one global leader or per-target ownership, failover semantics, and what status each standby pod reports.

## Answer

Use Kubernetes Lease-based leader election inside the Radar Agent runtime, with one Lease per capability:

- `radar-cluster-collector` for cluster-wide collection;
- `radar-database-collector` for database collection;
- `radar-qan-collector` for QAN collection.

The DaemonSet remains per-node. Node-local host metrics and OTLP ingestion run on every eligible pod without leader election. Cluster, database, and QAN are globally owned capabilities for the MVP; per-target database ownership is deferred until target-level scheduling and status are designed.

QAN ownership follows the database collection context. A QAN target may run only when its referenced database target is active on the owning database collector. A non-owner reports standby and must not poll the database or publish QAN data. This prevents duplicate QAN rows while allowing the agent to remain installed on every node.

Use Kubernetes Lease timing with a 15-second lease duration, 10-second renew deadline, and 2-second retry period. On leader loss, another eligible DaemonSet pod may acquire the Lease after expiration and start the capability. A short collection gap during failover is acceptable; concurrent active collection is not. The old holder must stop its capability loop when leadership is lost.

Capability status is reported to the gateway with at least:

- capability name;
- `active`, `standby`, `disabled`, or `failed` status;
- Lease name and holder identity;
- transition timestamp;
- target count;
- failure reason when applicable.

The Gateway receives status and data but does not participate in election and does not own Kubernetes Leases. RBAC must grant Lease get/list/watch/create/update/patch permissions only when ownership-enabled capabilities are configured.

Implementation consequence: the production OCB distribution needs a Kubernetes leader-election extension/component or a custom Collector-native component that exposes ownership state to the capability pipelines. The existing Go fixture runtime does not count as the production implementation. Cluster/database/QAN collectors must be blocked or fail closed until they can consume this ownership state.
