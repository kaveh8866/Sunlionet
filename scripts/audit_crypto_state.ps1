param(
    [string[]]$Packages = @(
        "./pkg/bundle",
        "./pkg/messaging",
        "./pkg/identity",
        "./pkg/ledger",
        "./pkg/ledgersync",
        "./pkg/mesh",
        "./pkg/release"
    ),
    [int]$Count = 1
)

$ErrorActionPreference = "Stop"

Write-Host "==> SunLionet crypto, signature, and state-machine validation"
$GoArgs = @("test") + $Packages + @("-count", $Count)
& go @GoArgs
