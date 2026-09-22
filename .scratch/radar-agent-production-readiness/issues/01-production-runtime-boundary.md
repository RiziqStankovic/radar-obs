# Production runtime boundary

Type: grilling
Status: resolved
Blocked by:

## Question

What is the final production runtime for `radar-agent`: an OCB-generated OpenTelemetry Collector binary, the current custom Go HTTP shim, or a hybrid? Decide what binary the Docker image must run, what happens to the existing Go `cmd/radar-agent`, and what interfaces custom Radar logic may use.

## Answer

Decision: production `radar-agent` is an OCB-generated OpenTelemetry Collector binary built from `radar-agent/builder-config.yaml`.

The production Docker image must run the generated Radar Agent Collector binary. It must not run upstream `otelcol-contrib` unchanged, and it must not use the current custom Go HTTP shim as the production runtime.

The existing Go shim may remain temporarily as prototype/dev code while migration proceeds, but it is outside the production runtime contract. Custom Radar behavior should be added through Collector-native extension points: receivers, processors, exporters, extensions, adapters, and Collector configuration generated into the OCB distribution.

This decision is recorded in `docs/adr/0001-radar-agent-production-runtime.md` and the glossary term `Radar Agent` is recorded in `CONTEXT.md`.
