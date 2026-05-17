param(
    [string[]]$Packages = @(
        "./tests/simulation/...",
        "./pkg/ledgersync",
        "./core/transport/multipath_router"
    ),
    [int]$Count = 1
)

$ErrorActionPreference = "Stop"

Write-Host "==> SunLionet chaos mock: packet loss, blackout, and degradation simulations"
$GoArgs = @("test") + $Packages + @("-count", $Count)
& go @GoArgs
