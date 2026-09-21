FROM alpine:3.22 AS ocb
ARG OCB_VERSION=0.161.0
RUN apk add --no-cache curl
RUN curl -fsSL "https://github.com/open-telemetry/opentelemetry-collector/releases/download/cmd%2Fbuilder%2Fv${OCB_VERSION}/ocb_${OCB_VERSION}_linux_amd64.tar.gz" \
  | tar -xz -C /usr/local/bin ocb

FROM golang:1.26 AS build
WORKDIR /src
COPY --from=ocb /usr/local/bin/ocb /usr/local/bin/ocb
COPY builder-config.yaml ./builder-config.yaml
RUN ocb --config builder-config.yaml --skip-compilation
WORKDIR /src/radar-agent
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/radar-agent ./cmd/radar-agent

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/radar-agent /radar-agent
COPY config/otel-config.yaml /etc/radar-agent/otel-config.yaml
EXPOSE 4317 4318 8080
ENTRYPOINT ["/radar-agent", "--config=/etc/radar-agent/otel-config.yaml"]
