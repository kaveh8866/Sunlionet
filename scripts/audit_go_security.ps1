param(
    [string]$PackagePattern = "./..."
)

$ErrorActionPreference = "Stop"

function Resolve-GoPackages {
    param(
        [Parameter(Mandatory = $true)][string]$Pattern
    )

    if ($Pattern -eq "./...") {
        return @(go list $Pattern)
    }

    return @($Pattern)
}

function Invoke-OptionalTool {
    param(
        [Parameter(Mandatory = $true)][string]$Name,
        [Parameter(Mandatory = $true)][scriptblock]$InstalledCommand,
        [Parameter(Mandatory = $true)][scriptblock]$GoRunCommand
    )

    Write-Host "==> $Name"
    if (Get-Command $Name -ErrorAction SilentlyContinue) {
        & $InstalledCommand
        return $LASTEXITCODE
    }

    Write-Host "$Name is not installed; using go run binding."
    & $GoRunCommand
    return $LASTEXITCODE
}

$Packages = Resolve-GoPackages -Pattern $PackagePattern
$Failures = 0

$GosecExit = Invoke-OptionalTool `
    -Name "gosec" `
    -InstalledCommand { gosec -exclude-dir=website $PackagePattern } `
    -GoRunCommand { go run github.com/securego/gosec/v2/cmd/gosec@latest -- -exclude-dir=website $PackagePattern }
if ($GosecExit -ne 0) {
    $Failures++
}

$GovulncheckExit = Invoke-OptionalTool `
    -Name "govulncheck" `
    -InstalledCommand { govulncheck $Packages } `
    -GoRunCommand { go run golang.org/x/vuln/cmd/govulncheck@latest $Packages }
if ($GovulncheckExit -ne 0) {
    $Failures++
}

if ($Failures -gt 0) {
    exit 1
}
