# ADR 0001: Radar Agent production runtime is an OCB-generated Collector

## Status

Accepted

## Context

`radar-agent` currently contains two possible runtime shapes: a custom Go HTTP shim under `cmd/radar-agent` and an OpenTelemetry Collector configuration plus `builder-config.yaml`. The Dockerfile previously ran the upstream `otelcol-contrib` image directly, while the Go shim was not used by the image. This made the final runtime boundary unclear and blocked production packaging, Helm configuration, leader election, and collector activation work.

The project goal is a Datadog-like install experience: one Radar Agent image, one DaemonSet pod per eligible node, and multiple collection capabilities inside that single agent process. The repository instructions also prefer OpenTelemetry Collector Contrib components over custom OTLP receivers, processors, exporters, or ClickHouse persistence.

## Decision

The production `radar-agent` runtime is an OCB-generated OpenTelemetry Collector binary built from `radar-agent/builder-config.yaml`.

The production Docker image must execute that generated Radar Agent Collector binary, not the upstream `otelcol-contrib` binary unchanged and not the current custom Go HTTP shim.

Custom Radar functionality should be added through Collector components, processors, receivers, extensions, exporters, or configuration/adapters compatible with the OCB distribution. The existing Go shim may remain temporarily as prototype/dev code only, but it is not the production runtime contract.

## Consequences

- `builder-config.yaml` becomes part of the production build contract.
- The image build must prove it runs the generated `radar-agent` Collector binary.
- Helm chart configuration should target Collector configuration and Collector health semantics.
- Cluster/database/QAN implementation should prefer Collector pipelines and custom Collector components over independent HTTP forwarding code.
- A later cleanup can remove or relocate the Go shim once migration is complete.
