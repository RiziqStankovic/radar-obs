Status: resolved
Type: research
Blocked by: none

# OTel v0.158 custom component and OCB module layout

## Question

What repository/module layout and factory interfaces are required for a local `postgresqanreceiver` and `radarqanexporter` to build into the existing OCB v0.158.0 distribution? Identify whether the components can live in this repository/module, how builder-config should reference them, and what test/build constraints apply.

## Answer

Use a separate local Go module for the OTel components; do not place them in the existing `radar-agent` prototype module. Recommended layout:

```text
radar-agent/
  otel-modules/
    go.mod                         # module gitrepo.xlaxiata.id/radar/radar-agent-otel-modules
    receiver/postgresqanreceiver/
      factory.go, receiver.go, config.go, ...
    exporter/radarqanexporter/
      factory.go, exporter.go, config.go, ...
```

The package import paths are therefore:

- `gitrepo.xlaxiata.id/radar/radar-agent-otel-modules/receiver/postgresqanreceiver`
- `gitrepo.xlaxiata.id/radar/radar-agent-otel-modules/exporter/radarqanexporter`

The existing `radar-agent/builder-config.yaml` is pinned to OTel/OCB `v0.158.0`, with `dist.module: gitrepo.xlaxiata.id/radar/radar-agent-collector`. Add the custom modules as local components while keeping the `gomod` version compatible with the generated distribution:

```yaml
receivers:
  - gomod: gitrepo.xlaxiata.id/radar/radar-agent-otel-modules v0.0.0
    import: gitrepo.xlaxiata.id/radar/radar-agent-otel-modules/receiver/postgresqanreceiver
    path: ./otel-modules

exporters:
  - gomod: gitrepo.xlaxiata.id/radar/radar-agent-otel-modules v0.0.0
    import: gitrepo.xlaxiata.id/radar/radar-agent-otel-modules/exporter/radarqanexporter
    path: ./otel-modules
```

`path` is the module root, not the individual package directory. The module must declare OTel Collector dependencies at `v0.158.0` (and the matching `go.opentelemetry.io/collector/pdata`, `consumer`, `component`, `receiver`, `exporter`, config, helper, and contrib dependencies as needed). `v0.0.0` is only a local placeholder; OCB resolves the package from `path`. Alternatively, publish/tag the module and omit `path`, but an in-repository local module is the concrete option for this repo.

For the selected QAN seam (receiver emits OTel logs and exporter sends them to the gateway), the v0.158.0 interfaces are:

- `postgresqanreceiver.NewFactory() receiver.Factory`, implemented with `receiver/xreceiver.NewFactory(...)` (the cache has `go.opentelemetry.io/collector/receiver/xreceiver@v0.158.0`). Register `xreceiver.WithLogs(createLogs, component.StabilityLevel...)`; `createLogs` has signature `func(context.Context, receiver.Settings, component.Config, consumer.Logs) (receiver.Logs, error)`. The receiver implements `Start(context.Context, component.Host) error` and `Shutdown(context.Context) error` and forwards QAN records as `plog.Logs` to `consumer.Logs`.
- `radarqanexporter.NewFactory() exporter.Factory`, implemented with `exporter/xexporter.NewFactory(...)` (cache: `go.opentelemetry.io/collector/exporter/xexporter@v0.158.0`). Register `xexporter.WithLogs(createLogs, component.StabilityLevel...)`; `createLogs` has signature `func(context.Context, exporter.Settings, component.Config) (exporter.Logs, error)`. The returned exporter implements `consumer.Logs` (`ConsumeLogs(context.Context, plog.Logs) error`) plus `Start`/`Shutdown`. Use `exporterhelper.NewLogs` for retry/queue/timeout behavior if appropriate.

Concrete cache evidence and commands/facts:

- `C:/Users/Riziq Maulana/go/pkg/mod/go.opentelemetry.io/collector/cmd/builder@v0.158.0/README.md`: OCB accepts `gomod`, optional `import`, and optional local `path`; module entries are top-level `receivers`/`exporters`. It performs generate, module resolution, and compilation; strict major/minor version checks apply, and `CGO_ENABLED=0` is the default.
- `C:/Users/Riziq Maulana/go/pkg/mod/go.opentelemetry.io/collector/receiver/xreceiver@v0.158.0/receiver.go`: `NewFactory`, `WithLogs`, and the receiver log creator contract.
- `C:/Users/Riziq Maulana/go/pkg/mod/go.opentelemetry.io/collector/exporter/xexporter@v0.158.0/exporter.go`: `NewFactory`, `WithLogs`, and the exporter log creator contract.
- Existing build command: `scripts/build-collectors.ps1` runs `go install go.opentelemetry.io/collector/cmd/builder@v0.158.0`, then from `radar-agent` runs `builder.exe --config=builder-config.yaml --skip-get-modules=false`.
- Existing `radar-agent/go.mod` is `gitrepo.xlaxiata.id/radar/radar-agent` and does not contain the OTel component module; this is why a separate `otel-modules/go.mod` avoids coupling the prototype to the generated OCB distribution.

Before enabling a production build, run from `radar-agent`:

```powershell
$env:CGO_ENABLED = "0"
& "$env:USERPROFILE\go\bin\builder.exe" --config=builder-config.yaml --skip-get-modules=false
```

Then test the generated receiver/exporter configuration with a logs pipeline. Component unit tests should use the v0.158.0 `receivertest`, `exportertest`, `componenttest`, and `consumer` helpers; an integration test must verify the QAN log body/attributes and gateway request mapping without putting `query_id` or fingerprint into metric labels.
