Write-Host "====================================================" -ForegroundColor Green
Write-Host "       ShadowNet Agent: Full Test Suite (Windows)   " -ForegroundColor Green
Write-Host "====================================================" -ForegroundColor Green

if (!(Get-Command "go" -ErrorAction SilentlyContinue)) {
    if (Test-Path "C:\Program Files\Go\bin\go.exe") {
        Write-Host "Found Go at default installation path, temporarily adding it to PATH..." -ForegroundColor Yellow
        $env:Path += ";C:\Program Files\Go\bin"
    } else {
        Write-Host "Error: Go is not installed or not in PATH." -ForegroundColor Red
        Write-Host "Please download and install Go for Windows from: https://go.dev/dl/"
        exit 1
    }
}

Write-Host "`nDownloading dependencies..." -ForegroundColor Yellow
go mod tidy
if ($LASTEXITCODE -ne 0) {
    Write-Host "`n✗ Failed to download dependencies." -ForegroundColor Red
    exit 1
}

Write-Host "`nRunning Go Unit Tests..." -ForegroundColor Yellow
go test -v ./...
if ($LASTEXITCODE -eq 0) {
    Write-Host "`n✓ All unit tests passed successfully!" -ForegroundColor Green
} else {
    Write-Host "`n✗ Some unit tests failed. Check the output above." -ForegroundColor Red
    exit 1
}

Write-Host "`nRunning Go Unit Tests (outside tag)..." -ForegroundColor Yellow
go test -v -tags outside ./...
if ($LASTEXITCODE -eq 0) {
    Write-Host "`n✓ Outside-tag tests passed successfully!" -ForegroundColor Green
} else {
    Write-Host "`n✗ Outside-tag tests failed. Check the output above." -ForegroundColor Red
    exit 1
}

Write-Host "`nRunning Go Linter (go vet)..." -ForegroundColor Yellow
go vet ./...
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ No suspicious constructs found." -ForegroundColor Green
}

Write-Host "`nRunning Go Linter (go vet, outside tag)..." -ForegroundColor Yellow
go vet -tags outside ./...
if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ No suspicious constructs found (outside)." -ForegroundColor Green
}

Write-Host "`nChecking code formatting..." -ForegroundColor Yellow
$unformatted = gofmt -l .
if ([string]::IsNullOrWhiteSpace($unformatted)) {
    Write-Host "✓ All files are properly formatted." -ForegroundColor Green
} else {
    Write-Host "✗ The following files are not formatted correctly:" -ForegroundColor Red
    Write-Host $unformatted
    Write-Host "Run 'gofmt -w .' to fix them."
}

Write-Host "`n====================================================" -ForegroundColor Green
Write-Host "  Test Suite Completed Successfully!                " -ForegroundColor Green
Write-Host "====================================================" -ForegroundColor Green
