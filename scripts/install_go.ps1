$ErrorActionPreference = "Stop"

Write-Host "====================================================" -ForegroundColor Cyan
Write-Host "       SunLionet: Go Installer for Windows          " -ForegroundColor Cyan
Write-Host "====================================================" -ForegroundColor Cyan

$GoVersion = "1.22.1"
$DownloadUrl = "https://go.dev/dl/go${GoVersion}.windows-amd64.msi"
$InstallerPath = "$env:TEMP\go_installer.msi"

Write-Host "1. Downloading Go version $GoVersion..." -ForegroundColor Yellow
Invoke-WebRequest -Uri $DownloadUrl -OutFile $InstallerPath

Write-Host "2. Installing Go (this may require Administrator privileges)..." -ForegroundColor Yellow
Write-Host "A Windows Installer popup may appear. Please click 'Yes' to allow the installation." -ForegroundColor Cyan

# Run the MSI installer quietly
Start-Process -FilePath "msiexec.exe" -ArgumentList "/i `"$InstallerPath`" /quiet /norestart" -Wait -NoNewWindow

Write-Host "3. Cleaning up installer..." -ForegroundColor Yellow
Remove-Item -Path $InstallerPath -Force

Write-Host "4. Adding Go to current session PATH..." -ForegroundColor Yellow
$env:Path += ";C:\Program Files\Go\bin"

if (Get-Command "go" -ErrorAction SilentlyContinue) {
    Write-Host "`n✓ Go has been successfully installed!" -ForegroundColor Green
    go version
    Write-Host "`nIMPORTANT: You must RESTART your terminal (or Trae IDE) for the PATH changes to be permanent." -ForegroundColor Magenta
    Write-Host "After restarting, run '.\scripts\run_tests.ps1' again." -ForegroundColor Cyan
} else {
    Write-Host "`n✗ Installation seemed to finish, but 'go' is still not found." -ForegroundColor Red
    Write-Host "You may need to manually download it from https://go.dev/dl/ and install it." -ForegroundColor Red
}
