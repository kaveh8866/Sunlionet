Write-Host '====================================================' -ForegroundColor Green
Write-Host '       SunLionet: Full Test Suite (Windows)         ' -ForegroundColor Green
Write-Host '====================================================' -ForegroundColor Green

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

Write-Host ''
Write-Host 'Downloading dependencies...' -ForegroundColor Yellow
go mod tidy
if ($LASTEXITCODE -ne 0) {
    Write-Host ''
    Write-Host 'FAILED: downloading dependencies.' -ForegroundColor Red
    exit 1
}

Write-Host ''
Write-Host 'Running Go Unit Tests...' -ForegroundColor Yellow
go test -v ./...
if ($LASTEXITCODE -eq 0) {
    Write-Host ''
    Write-Host 'OK: unit tests passed.' -ForegroundColor Green
} else {
    Write-Host ''
    Write-Host 'FAILED: unit tests. Check the output above.' -ForegroundColor Red
    exit 1
}

Write-Host ''
Write-Host 'Running Go Unit Tests (outside tag)...' -ForegroundColor Yellow
go test -v -tags outside ./...
if ($LASTEXITCODE -eq 0) {
    Write-Host ''
    Write-Host 'OK: outside-tag tests passed.' -ForegroundColor Green
} else {
    Write-Host ''
    Write-Host 'FAILED: outside-tag tests. Check the output above.' -ForegroundColor Red
    exit 1
}

Write-Host ''
Write-Host 'Running Go Linter (go vet)...' -ForegroundColor Yellow
go vet ./...
if ($LASTEXITCODE -eq 0) {
    Write-Host 'OK: no suspicious constructs found.' -ForegroundColor Green
}

Write-Host ''
Write-Host 'Running Go Linter (go vet, outside tag)...' -ForegroundColor Yellow
go vet -tags outside ./...
if ($LASTEXITCODE -eq 0) {
    Write-Host 'OK: no suspicious constructs found (outside).' -ForegroundColor Green
}

Write-Host ''
Write-Host 'Checking code formatting...' -ForegroundColor Yellow
$unformatted = gofmt -l .
if ([string]::IsNullOrWhiteSpace($unformatted)) {
    Write-Host 'OK: all files are properly formatted.' -ForegroundColor Green
} else {
    Write-Host 'FAILED: the following files are not formatted correctly:' -ForegroundColor Red
    Write-Host $unformatted
    Write-Host "Run 'gofmt -w .' to fix them."
}

Write-Host ''
Write-Host '====================================================' -ForegroundColor Green
Write-Host '  Test Suite Completed Successfully!                ' -ForegroundColor Green
Write-Host '====================================================' -ForegroundColor Green
