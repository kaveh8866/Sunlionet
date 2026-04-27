param(
  [string]$Version = "v0.1.0"
)

$ErrorActionPreference = "Stop"

$goExe = "C:\Program Files\Go\bin\go.exe"
if (!(Test-Path $goExe)) {
  throw "go.exe not found at $goExe"
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$outDir = Join-Path $repoRoot ("website\public\downloads\" + $Version)
$tmpDir = Join-Path $repoRoot ("website\.tmp_release\" + $Version)

New-Item -ItemType Directory -Force -Path $outDir | Out-Null
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null

function Write-TextFile {
  param([string]$Path, [string]$Content)
  $dir = Split-Path -Parent $Path
  if ($dir -and !(Test-Path $dir)) {
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
  }
  [System.IO.File]::WriteAllText($Path, $Content.Replace("`r`n", "`n"), [System.Text.Encoding]::UTF8)
}

function Sha256File {
  param([string]$Path)
  (Get-FileHash -Algorithm SHA256 $Path).Hash.ToLowerInvariant()
}

function Build-GoBinary {
  param(
    [string]$Name,
    [string]$Tags,
    [string]$GoOS,
    [string]$GoArch,
    [string]$OutPath,
    [string]$Pkg
  )

  $env:GOOS = $GoOS
  $env:GOARCH = $GoArch
  $env:CGO_ENABLED = "0"

  & $goExe build -trimpath -tags $Tags -ldflags "-s -w -X main.version=$Version" -o $OutPath $Pkg | Out-Null
  if (!(Test-Path $OutPath)) {
    throw "build failed: $Name ($GoOS/$GoArch)"
  }
}

$installLinux = @'
#!/usr/bin/env sh
set -eu
mode="${1:-inside}"
bin="sunlionet-${mode}"
src="./${bin}"
dst="/usr/local/bin/${bin}"
install -m 0755 "$src" "$dst"
mkdir -p /etc/sunlionet
if [ -f "./${bin}.service" ]; then
  install -m 0644 "./${bin}.service" "/etc/systemd/system/${bin}.service"
  systemctl daemon-reload || true
fi
printf "%s\n" "installed ${dst}"
'@

$insideService = @'
[Unit]
Description=SunLionet (Inside)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=-/etc/sunlionet/inside.env
ExecStart=/usr/local/bin/sunlionet-inside
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
'@

$outsideService = @'
[Unit]
Description=SunLionet (Outside)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=-/etc/sunlionet/outside.env
ExecStart=/usr/local/bin/sunlionet-outside
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
'@

Write-TextFile -Path (Join-Path $tmpDir "install-linux.sh") -Content $installLinux
Write-TextFile -Path (Join-Path $tmpDir "sunlionet-inside.service") -Content $insideService
Write-TextFile -Path (Join-Path $tmpDir "sunlionet-outside.service") -Content $outsideService

function New-TarGzPackage {
  param([string]$BaseName, [string]$WorkDir)
  $tarPath = Join-Path $outDir ($BaseName + ".tar.gz")
  if (Test-Path $tarPath) { Remove-Item -Force $tarPath }
  tar -czf $tarPath -C $WorkDir . | Out-Null
  $sha = Sha256File $tarPath
  Write-TextFile -Path ($tarPath + ".sha256") -Content ($sha + "  " + (Split-Path -Leaf $tarPath) + "`n")
}

function New-ZipPackage {
  param([string]$BaseName, [string]$WorkDir)
  $zipPath = Join-Path $outDir ($BaseName + ".zip")
  if (Test-Path $zipPath) { Remove-Item -Force $zipPath }
  Compress-Archive -Force -Path (Join-Path $WorkDir "*") -DestinationPath $zipPath
  $sha = Sha256File $zipPath
  Write-TextFile -Path ($zipPath + ".sha256") -Content ($sha + "  " + (Split-Path -Leaf $zipPath) + "`n")
}

$releaseManifest = @{
  version = $Version
  date = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"
  artifacts = @()
}

function Build-And-Package {
  param(
    [string]$Mode,
    [string]$Tags,
    [string]$GoOS,
    [string]$GoArch,
    [string]$Format
  )

  $binBase = "sunlionet-$Mode-$Version-$GoOS-$GoArch"
  $work = Join-Path $tmpDir $binBase
  if (Test-Path $work) { Remove-Item -Recurse -Force $work }
  New-Item -ItemType Directory -Force -Path $work | Out-Null

  $binName = "sunlionet-$Mode"
  if ($GoOS -eq "windows") { $binName = $binName + ".exe" }
  $binPath = Join-Path $work $binName

  $pkg = ".\cmd\$Mode\"
  Build-GoBinary -Name $binBase -Tags $Tags -GoOS $GoOS -GoArch $GoArch -OutPath $binPath -Pkg $pkg

  Copy-Item -Force (Join-Path $tmpDir "install-linux.sh") (Join-Path $work "install-linux.sh")
  if ($Mode -eq "inside") {
    Copy-Item -Force (Join-Path $tmpDir "sunlionet-inside.service") (Join-Path $work "sunlionet-inside.service")
  }
  if ($Mode -eq "outside") {
    Copy-Item -Force (Join-Path $tmpDir "sunlionet-outside.service") (Join-Path $work "sunlionet-outside.service")
  }

  $artifactPath = ""
  $artifactName = ""
  if ($Format -eq "targz") {
    New-TarGzPackage -BaseName $binBase -WorkDir $work
    $artifactName = $binBase + ".tar.gz"
    $artifactPath = Join-Path $outDir $artifactName
  } elseif ($Format -eq "zip") {
    New-ZipPackage -BaseName $binBase -WorkDir $work
    $artifactName = $binBase + ".zip"
    $artifactPath = Join-Path $outDir $artifactName
  } elseif ($Format -eq "raw") {
    $artifactName = "sunlionet-$Mode-$Version-$GoOS-$GoArch"
    $artifactPath = Join-Path $outDir $artifactName
    Copy-Item -Force $binPath $artifactPath
    $sha = Sha256File $artifactPath
    Write-TextFile -Path ($artifactPath + ".sha256") -Content ($sha + "  " + (Split-Path -Leaf $artifactPath) + "`n")
  } else {
    throw "unknown format: $Format"
  }

  $sha256 = Sha256File $artifactPath
  $releaseManifest.artifacts += @{
    name = $artifactName
    mode = $Mode
    os = $GoOS
    arch = $GoArch
    sha256 = $sha256
    size = (Get-Item $artifactPath).Length
  }
}

Build-And-Package -Mode "inside" -Tags "inside" -GoOS "linux" -GoArch "amd64" -Format "targz"
Build-And-Package -Mode "inside" -Tags "inside" -GoOS "linux" -GoArch "arm64" -Format "targz"
Build-And-Package -Mode "inside" -Tags "inside" -GoOS "darwin" -GoArch "arm64" -Format "targz"
Build-And-Package -Mode "inside" -Tags "inside" -GoOS "windows" -GoArch "amd64" -Format "zip"
Build-And-Package -Mode "inside" -Tags "inside" -GoOS "android" -GoArch "arm64" -Format "raw"

Build-And-Package -Mode "outside" -Tags "outside" -GoOS "linux" -GoArch "amd64" -Format "targz"
Build-And-Package -Mode "outside" -Tags "outside" -GoOS "linux" -GoArch "arm64" -Format "targz"
Build-And-Package -Mode "outside" -Tags "outside" -GoOS "darwin" -GoArch "arm64" -Format "targz"
Build-And-Package -Mode "outside" -Tags "outside" -GoOS "windows" -GoArch "amd64" -Format "zip"

$manifestJson = $releaseManifest | ConvertTo-Json -Depth 10
Write-TextFile -Path (Join-Path $outDir "RELEASES.json") -Content $manifestJson

Write-TextFile -Path (Join-Path $outDir "VERSION.txt") -Content ($Version + "`n")

Write-Host "Release artifacts written to: $outDir"
