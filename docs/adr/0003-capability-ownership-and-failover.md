# ADR 0003: Leader-elected ownership for cluster, database, and QAN capabilities

## Status

Accepted

## Context

`radar-agent` is installed as a DaemonSet, so multiple pods can observe the same Kubernetes cluster or database targets. Node-local host metrics and OTLP intake are intentionally duplicated by design because each pod owns its node, but cluster-wide and database-derived collection must not run concurrently on every pod. QAN is especially sensitive to duplicate polling because the same query buckets could be sent to the gateway multiple times.

The gateway is the central ingestion and storage boundary, but it should not own Kubernetes coordination. Ownership must be visible to the agent runtime so a non-owner can stop collection rather than merely dropping duplicate data after the fact.

## Decision

Use Kubernetes Lease-based leader election inside the Radar Agent runtime.

Use separate Lease names:

- `radar-cluster-collector` for cluster-wide collection;
- `radar-database-collector` for database collection;
- `radar-qan-collector` for QAN collection.

Node and OTLP capabilities remain active on every eligible DaemonSet pod without election. Cluster, database, and QAN use global capability ownership for the MVP. Per-target database ownership is deferred.

QAN follows the database collection context: a QAN target may run only when its referenced database target is active on the database owner. Standby pods report status but do not poll or publish. Leadership loss stops the associated collection loop. Use a 15-second lease duration, 10-second renew deadline, and 2-second retry period.

## Consequences

- The agent runtime needs a Collector-native leader-election integration or component before cluster/database/QAN can be production-enabled.
- The chart requires Lease RBAC only for ownership-enabled capabilities.
- The gateway receives capability status and data but does not participate in Lease election.
- Failover can introduce a short collection gap, but normal operation must not have multiple active owners.
- Status must include capability, state, Lease name, holder identity, transition time, target count, and failure reason where applicable.
