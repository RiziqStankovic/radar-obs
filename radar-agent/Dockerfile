# Independently deployed radar-agent Collector distribution.
FROM golang:1.24-bookworm AS builder
WORKDIR /src
COPY builder-config.yaml ./
RUN go install go.opentelemetry.io/collector/cmd/builder@v0.158.0 \
    && builder --config=builder-config.yaml --skip-get-modules=false

FROM gcr.io/distroless/base-debian12:nonroot
COPY --from=builder /src/generated/radar-agent /radar-agent
COPY config/collector.yaml /etc/radar-agent/collector.yaml
EXPOSE 4317 4318 13133
ENTRYPOINT ["/radar-agent"]
CMD ["--config=/etc/radar-agent/collector.yaml"]
