$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$envFile = Join-Path $root '.env'
if (-not (Test-Path $envFile)) {
  throw "Missing $envFile. Copy .env.example to .env and set ClickHouse credentials."
}
Get-Content $envFile | ForEach-Object {
  if ($_ -match '^\s*([^#=][^=]*)=(.*)$') {
    [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim().Trim('"'), 'Process')
  }
}
if ([string]::IsNullOrWhiteSpace($env:RADAR_CLICKHOUSE_URL)) {
  throw 'RADAR_CLICKHOUSE_URL is empty. Set the ClickHouse HTTP endpoint in .env.'
}
if ([string]::IsNullOrWhiteSpace($env:RADAR_CLICKHOUSE_PASSWORD)) {
  throw 'RADAR_CLICKHOUSE_PASSWORD is empty. Set the ClickHouse password in .env.'
}
if ([string]::IsNullOrWhiteSpace($env:RADAR_CLICKHOUSE_DATABASE)) {
  $env:RADAR_CLICKHOUSE_DATABASE = 'radar_metrics'
}
if ([string]::IsNullOrWhiteSpace($env:RADAR_GATEWAY_PORT)) {
  $env:RADAR_GATEWAY_PORT = '4319'
}

Write-Host "Starting Go radar-gateway on port $env:RADAR_GATEWAY_PORT..."
Push-Location (Join-Path $root 'radar-gateway')
try {
  & go run ./cmd/radar-gateway
}
finally {
  Pop-Location
}
