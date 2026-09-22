$ErrorActionPreference = 'Stop'

$root = Split-Path $PSScriptRoot -Parent
$envFile = Join-Path $root '.env'
$fixture = Join-Path $root 'qan-fixtures\collect_request.json'
$gatewayPort = if ($env:RADAR_GATEWAY_PORT) { $env:RADAR_GATEWAY_PORT } else { '4319' }

if (-not (Test-Path $envFile)) {
  throw "Missing $envFile. Copy .env.example to .env and set ClickHouse credentials."
}
if (-not (Test-Path $fixture)) {
  throw "Missing QAN fixture: $fixture"
}

Get-Content $envFile | ForEach-Object {
  if ($_ -match '^\s*([^#=][^=]*)=(.*)$') {
    [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim().Trim('"'), 'Process')
  }
}

if ([string]::IsNullOrWhiteSpace($env:RADAR_CLICKHOUSE_URL)) {
  throw 'RADAR_CLICKHOUSE_URL is empty. The QAN smoke test expects the ClickHouse HTTP endpoint, for example http://localhost:8123.'
}
if ([string]::IsNullOrWhiteSpace($env:RADAR_CLICKHOUSE_USER)) {
  throw 'RADAR_CLICKHOUSE_USER is empty.'
}
if ([string]::IsNullOrWhiteSpace($env:RADAR_CLICKHOUSE_PASSWORD)) {
  throw 'RADAR_CLICKHOUSE_PASSWORD is empty.'
}

$database = if ($env:RADAR_CLICKHOUSE_DATABASE) { $env:RADAR_CLICKHOUSE_DATABASE } else { 'radar_metrics' }
if ($database -ne 'radar_metrics') {
  throw "QAN smoke test requires RADAR_CLICKHOUSE_DATABASE=radar_metrics because the gateway also verifies the existing OTLP metric tables."
}

$headers = @{
  'X-ClickHouse-User' = $env:RADAR_CLICKHOUSE_USER
  'X-ClickHouse-Key' = $env:RADAR_CLICKHOUSE_PASSWORD
}

function Invoke-ClickHouseQuery([string] $query) {
  $uri = $env:RADAR_CLICKHOUSE_URL.TrimEnd('/') + '/?database=' + [uri]::EscapeDataString($database) + '&query=' + [uri]::EscapeDataString($query)
  return (Invoke-WebRequest -UseBasicParsing -Uri $uri -Headers $headers -Method Get).Content.Trim()
}

$gateway = $null
$gatewayLog = Join-Path $env:TEMP ('radar-gateway-qan-' + [guid]::NewGuid().ToString('N') + '.log')
$gatewayErrorLog = Join-Path $env:TEMP ('radar-gateway-qan-' + [guid]::NewGuid().ToString('N') + '.err.log')
try {
  $env:RADAR_GATEWAY_PORT = $gatewayPort
  $gateway = Start-Process -FilePath 'go' -ArgumentList 'run ./cmd/radar-gateway' -WorkingDirectory (Join-Path $root 'radar-gateway') -RedirectStandardOutput $gatewayLog -RedirectStandardError $gatewayErrorLog -PassThru -WindowStyle Hidden

  $ready = $false
  for ($attempt = 0; $attempt -lt 60; $attempt++) {
    Start-Sleep -Milliseconds 500
    try {
      $response = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$gatewayPort/readyz" -Method Get -TimeoutSec 2
      if ($response.StatusCode -eq 200) {
        $ready = $true
        break
      }
    }
    catch {
      if ($gateway.HasExited) { break }
    }
  }
  if (-not $ready) {
    $details = @(
          if (Test-Path $gatewayLog) { Get-Content $gatewayLog -Raw }
          if (Test-Path $gatewayErrorLog) { Get-Content $gatewayErrorLog -Raw }
        ) -join "`n"
    throw "Go radar-gateway did not become ready.`n$details"
  }

  $body = Get-Content $fixture -Raw
  $qanResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$gatewayPort/v1/qan/collect" -Method Post -ContentType 'application/json' -Body $body -TimeoutSec 10
  if ($qanResponse.StatusCode -ne 202) {
    throw "QAN endpoint returned HTTP $($qanResponse.StatusCode), expected 202."
  }

  $table = Invoke-ClickHouseQuery "SELECT count() FROM system.tables WHERE database = '$database' AND name = 'qan_metrics' FORMAT TabSeparated"
  if ($table -ne '1') {
    throw "Expected $database.qan_metrics to exist, got: '$table'"
  }

  $rows = Invoke-ClickHouseQuery "SELECT count() FROM qan_metrics WHERE query_id = '7d1f1c4b8d' FORMAT TabSeparated"
  if ([int64]$rows -lt 1) {
    throw "Expected at least one persisted QAN fixture row, got: '$rows'"
  }

  Write-Host "PASS: QAN fixture accepted by Go radar-gateway and persisted in $database.qan_metrics ($rows matching row(s))."
}
finally {
  if ($gateway -and -not $gateway.HasExited) {
    Stop-Process -Id $gateway.Id -Force -ErrorAction SilentlyContinue
  }
  if (Test-Path $gatewayLog) {
    Remove-Item $gatewayLog -Force -ErrorAction SilentlyContinue
  }
  if (Test-Path $gatewayErrorLog) {
    Remove-Item $gatewayErrorLog -Force -ErrorAction SilentlyContinue
  }
}
