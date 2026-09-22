Status: resolved
Type: grilling
Blocked by: none

# QAN data seam inside OTel Collector

## Question

Should PostgreSQL QAN flow through a standard OTel logs pipeline using an encoded QAN event, or should the custom receiver/exporter use another seam? Define the smallest stable interface that preserves query_id, fingerprint, interval deltas, metrics, privacy settings, and the gateway `CollectRequest` contract without turning query fingerprints into high-cardinality metric labels.

## Answer

Use the standard OTel logs pipeline as an internal event seam:

```text
postgresqanreceiver
        ↓ plog.Logs
logs/qan pipeline
        ↓
radarqanexporter
        ↓ POST /v1/qan/collect
radar-gateway
```

Each QAN bucket is encoded as structured JSON in the log body. Stable metadata such as `radar.qan=true`, `db.system=postgresql`, and `service.name` may be OTel attributes. `query_id` and `fingerprint` must remain in the structured body, not metric attributes, to avoid high-cardinality metrics.

The body must preserve the existing Radar QAN contract: query identity, fingerprint, service and agent metadata, period start/length, query count, metrics, extra metrics, and privacy-controlled example data. The receiver owns PostgreSQL polling, snapshots, and counter deltas; the exporter owns conversion to `CollectRequest`, HTTP delivery, retry, and gateway error handling.
