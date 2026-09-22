$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$envFile = Join-Path $root '.env'
$binary = Join-Path $root 'radar-agent\generated\radar-agent.exe'
$config = Join-Path $root 'radar-agent\config\collector.yaml'

if (-not (Test-Path $envFile)) {
  throw "Missing $envFile. Copy .env.example to .env first."
}
if (-not (Test-Path $binary)) {
  throw "Missing $binary. Run .\scripts\build-collectors.ps1 first."
}
if (-not (Test-Path $config)) {
  throw "Missing Collector config: $config"
}

Get-Content $envFile | ForEach-Object {
  if ($_ -match '^\s*([^#=][^=]*)=(.*)$') {
    [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim().Trim('"'), 'Process')
  }
}

Write-Host "Starting radar-agent Collector..."
Write-Host "OTLP HTTP: $env:RADAR_AGENT_OTLP_HTTP_ENDPOINT"
Write-Host "Gateway:   $env:RADAR_GATEWAY_OTLP_ENDPOINT"
& $binary --config $config
