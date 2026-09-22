$ErrorActionPreference = "Stop"

$payloadPath = Join-Path $PSScriptRoot "otel-metrics.json"
$payload = Get-Content -Raw $payloadPath

$response = Invoke-WebRequest `
  -Uri "http://localhost:4318/v1/metrics" `
  -Method Post `
  -ContentType "application/json" `
  -Body $payload

Write-Host "Agent response: $($response.StatusCode) $($response.StatusDescription)"
Write-Host "Query ClickHouse through the gateway's configured database to verify the row."
