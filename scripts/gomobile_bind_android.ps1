$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$outDir = Join-Path $repoRoot "android\app\libs"
$outFile = Join-Path $outDir "shadownet.aar"
$androidApi = 26

if (-not (Get-Command gomobile -ErrorAction SilentlyContinue)) {
  throw "gomobile not found in PATH. Install: go install golang.org/x/mobile/cmd/gomobile@latest"
}

if (-not (Test-Path $outDir)) {
  New-Item -ItemType Directory -Force -Path $outDir | Out-Null
}

Push-Location $repoRoot
try {
  gomobile init
  if ($LASTEXITCODE -ne 0) {
    throw "gomobile init failed with exit code $LASTEXITCODE"
  }
  gomobile bind -androidapi $androidApi -target android -javapkg "com.shadownet.mobile" -o $outFile ./pkg/mobile
  if ($LASTEXITCODE -ne 0) {
    throw "gomobile bind failed with exit code $LASTEXITCODE"
  }
  if (-not (Test-Path $outFile)) {
    throw "gomobile bind finished but output file not found: $outFile"
  }
  Write-Output "Wrote $outFile"
} finally {
  Pop-Location
}
