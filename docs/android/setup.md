# Android MVP Setup

## Requirements

- Android Studio Iguana+ / AGP 8.5+
- Android device/emulator with API 26+
- Go toolchain (for gomobile bridge generation)

## Build Steps

1. Build Go mobile bindings for `pkg/mobilebridge` (AAR/JAR) and place them in the Android app dependency path.
2. Open `android/` in Android Studio.
3. Sync Gradle.
4. Build and install `app` on a device.

## Required Permissions

- `BIND_VPN_SERVICE` (service permission)
- `FOREGROUND_SERVICE`
- `INTERNET`
- `POST_NOTIFICATIONS` (Android 13+)

## Runtime Flow

1. Tap **Connect**.
2. Grant VPN permission.
3. `ShadowNetVpnService` establishes TUN (legacy class name during transition).
4. `AgentService` starts Go agent loop and renders config.
5. `SingBoxController` starts `sing-box run -c <config>`.

## Assets

- Templates are under `android/app/src/main/assets/templates/`.
- Place per-arch sing-box binaries at:
  - `android/app/src/main/assets/sing-box/arm64-v8a/sing-box`
  - `android/app/src/main/assets/sing-box/armeabi-v7a/sing-box`

## Notes

- If sing-box asset is missing, app stays fail-safe and reports an explicit error in logs.
- No cloud dependency is required for runtime operation.
