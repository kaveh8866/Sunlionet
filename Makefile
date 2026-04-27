.PHONY: build test lint clean android-bind release-linux release-windows release-android release-all

build:
	go build ./...

test:
	go test ./...

lint:
	go vet ./...
	gitleaks detect --source . --verbose --redact || true
	govulncheck ./... || true

clean:
	rm -rf website/public/downloads/v*
	rm -rf website/.tmp_release
	rm -f android/app/libs/SUNLIONET.aar

android-bind:
	powershell -ExecutionPolicy Bypass -File ./scripts/gomobile_bind_android.ps1

release-linux:
	powershell -ExecutionPolicy Bypass -File ./scripts/build_release.ps1 -Version "v0.1.0"

release-windows:
	powershell -ExecutionPolicy Bypass -File ./scripts/build_release.ps1 -Version "v0.1.0"

release-android: android-bind
	cd android && ./gradlew assembleRelease

release-all: release-linux release-windows release-android
