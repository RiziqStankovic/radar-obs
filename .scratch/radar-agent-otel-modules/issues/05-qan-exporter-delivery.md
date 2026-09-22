Status: resolved
Type: grilling
Blocked by: 02, 03

# QAN exporter delivery semantics

## Question

How should the Radar QAN exporter handle gateway URL construction, authentication, retries, timeouts, empty batches, duplicate delivery, and ClickHouse persistence errors while preserving the existing `POST /v1/qan/collect` behavior?

## Answer

Use an at-least-once delivery contract with a dedicated QAN queue and retry policy. The exporter accepts a gateway base endpoint and appends `/v1/qan/collect`; local development may omit auth, while production may provide an optional Authorization header without logging it.

Retry transient network failures, timeouts, HTTP 408, 429, 500, 502, 503, and 504 with exponential backoff, jitter, bounded maximum delay, and bounded total retry time. Do not retry 400, 401, 403, 404, or validation/contract errors. Empty batches are successful no-ops and must not generate HTTP requests.

Delivery is at-least-once. Gateway/ClickHouse persistence must tolerate possible duplicate delivery using a deterministic batch identity derived from stable agent/service/query/period fields, or an equivalent explicit idempotency key. The exporter must not depend on ClickHouse details.

QAN queue/retry failure must not stop or backpressure unrelated OTLP traces/logs/metrics pipelines. Queue growth is bounded; when exhausted, the exporter emits failure/drop telemetry and structured logs without DSNs, passwords, or raw query text. Gateway responses remain `202 Accepted` for success, `400 Bad Request` for permanent validation errors, and `503 Service Unavailable` for transient persistence failures.

Exporter telemetry includes request, success, retry, failure, dropped, bucket-count, queue-size, and last-success measurements.
