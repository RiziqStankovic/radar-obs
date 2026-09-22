# OCB build and image contract

Type: task
Status: resolved
Blocked by: 01

## Question

What exact build layout, OCB config, Dockerfile stages, generated binary path, and validation commands should produce the production `radar-agent` image? The answer must define how `builder-config.yaml` is used and how to prove the image is not accidentally running upstream `otelcol-contrib` unchanged.

## Answer

The production image contract is the existing two-stage `radar-agent/Dockerfile`:

1. Use `golang:1.24-bookworm` as the build stage.
2. Copy `builder-config.yaml` into `/src`.
3. Install the pinned OCB builder `go.opentelemetry.io/collector/cmd/builder@v0.158.0`.
4. Run `builder --config=builder-config.yaml --skip-get-modules=false`.
5. Copy `/src/generated/radar-agent` into a distroless nonroot runtime image as `/radar-agent`.
6. Copy `config/collector.yaml` as `/etc/radar-agent/collector.yaml` and start with `--config=/etc/radar-agent/collector.yaml`.

`builder-config.yaml` is the source of truth for the distribution. It pins the module name/version and explicitly lists the receivers, processors, exporters, and extensions. The image must never replace the generated binary with a downloaded upstream `otelcol-contrib` binary.

The current component set is the MVP OTLP/host-metrics set: OTLP receiver, hostmetrics receiver, memory limiter, batch, OTLP HTTP exporter, and health check. Kubelet, Kubernetes attributes, cluster, database, and QAN components require explicit additions to both the builder config and runtime config before they can be enabled.

Validation contract:

- Run `go install .../builder@v0.158.0` and the builder command successfully.
- Assert the generated `/src/generated/radar-agent` exists and is the Docker runtime binary.
- Build the image and inspect its entrypoint/cmd; it must be `/radar-agent`, not `otelcol-contrib`.
- Start the image with the shipped collector config and verify the health endpoint on `13133`.
- Verify the generated distribution metadata/component list contains the pinned custom module `gitrepo.xlaxiata.id/radar/radar-agent-collector` and the configured components.

This ticket establishes the image/build contract only. It does not claim that cluster-wide collection, leader election, or real QAN is implemented; those remain separate tickets.
