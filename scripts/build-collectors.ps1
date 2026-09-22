$ErrorActionPreference = 'Stop'

# Requires Go and network access to the Go module proxy.
# OCB is pinned to the same release as the generated distributions.
$ocbVersion = 'v0.158.0'
$ocb = Join-Path $env:USERPROFILE 'go\bin\builder.exe'

go install "go.opentelemetry.io/collector/cmd/builder@$ocbVersion"

Push-Location (Join-Path $PSScriptRoot '..\radar-agent')
try {
  & $ocb --config=builder-config.yaml --skip-get-modules=false
}
finally { Pop-Location }

Push-Location (Join-Path $PSScriptRoot '..\radar-gateway')
try {
  & $ocb --config=builder-config.yaml --skip-get-modules=false
}
finally { Pop-Location }

Write-Host 'Built radar-agent/generated/radar-agent.exe and radar-gateway/generated/radar-gateway.exe'
