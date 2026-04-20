# Android UI/UX Testing

This document tracks practical UI/UX coverage for the Android app main flow.

## Scope

- First-launch guidance (no configuration)
- Import configuration path (file picker / QR / paste link entry points)
- Primary action clarity (`Connect` / `Disconnect`)
- State visibility (`Disconnected`, `Connecting`, `Connected`, `Connection failed`)
- Advanced details toggle behavior
- Accessibility basics (tap targets, labels, loading text)

## Current Interaction Test Coverage

Instrumentation tests are in:

- `android/app/src/androidTest/java/com/SUNLIONET/agent/MainActivityTest.kt`

Covered checks:

- App launches and main screen renders
- Primary CTA is visible and tappable
- Status text is visible
- Connected state renders `Disconnect`
- Error state renders a readable message
- Advanced details toggle can be opened/closed
- Import-related CTA path is visible to the tester

## Run Android UI Tests

1. Start an emulator (API 29+ recommended).
2. From `android/`, run:

```bash
./gradlew lintDebug testDebugUnitTest connectedDebugAndroidTest
```

On Windows PowerShell:

```powershell
.\gradlew.bat lintDebug testDebugUnitTest connectedDebugAndroidTest
```

## UX Assumptions

- If no configuration bundle exists, the UI should guide the user to import configuration before connecting.
- Status is always communicated by text, not color alone.
- Raw technical details should stay behind advanced/details affordances.

## Known Limitations

- VPN permission dialog behavior still depends on device/emulator policy.
- Real network blocking and captive-portal style failures require physical-network validation.
- Accessibility audit is practical but not a full TalkBack script pass yet.
