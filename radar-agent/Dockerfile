FROM otel/opentelemetry-collector-contrib:0.161.0

ENV RADAR_AGENT_HEALTH_ENDPOINT=0.0.0.0:13133 \
    RADAR_OTLP_GRPC_ENDPOINT=0.0.0.0:4317 \
    RADAR_OTLP_HTTP_ENDPOINT=0.0.0.0:4318 \
    RADAR_HOST_METRICS_INTERVAL=30s \
    RADAR_KUBELET_METRICS_INTERVAL=30s \
    RADAR_KUBELET_INSECURE_SKIP_VERIFY=false \
    RADAR_HOST_ROOT_PATH=/hostfs \
    RADAR_GATEWAY_OTLP_ENDPOINT=http://radar-gateway:4318

COPY config/otel-config.yaml /etc/radar-agent/otel-config.yaml
EXPOSE 4317 4318 13133
ENTRYPOINT ["/otelcol-contrib"]
CMD ["--config=/etc/radar-agent/otel-config.yaml"]
