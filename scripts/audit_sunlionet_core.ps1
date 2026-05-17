param(
    [switch]$SkipSecurityScan
)

$ErrorActionPreference = "Stop"

Write-Host "==> Core Go tests"
go test ./...

Write-Host "==> Crypto and state validation"
& "$PSScriptRoot\audit_crypto_state.ps1"

Write-Host "==> Chaos mock validation"
& "$PSScriptRoot\audit_chaos_mock.ps1"

if (-not $SkipSecurityScan) {
    Write-Host "==> Go security and vulnerability scan"
    & "$PSScriptRoot\audit_go_security.ps1"
}
