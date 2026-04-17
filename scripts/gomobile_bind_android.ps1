$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$outDir = Join-Path $repoRoot "android\app\libs"
$outFile = Join-Path $outDir "shadownet.aar"

if (-not (Get-Command gomobile -ErrorAction SilentlyContinue)) {
  throw "gomobile not found in PATH. Install: go install golang.org/x/mobile/cmd/gomobile@latest"
}

if (-not (Test-Path $outDir)) {
  New-Item -ItemType Directory -Force -Path $outDir | Out-Null
}

Push-Location $repoRoot
try {
  gomobile init
  gomobile bind -target=android -javapkg=com.shadownet.mobile -o $outFile ./pkg/mobile
  Write-Output "Wrote $outFile"
} finally {
  Pop-Location
}

