# SunLionet Mirror and Distribution Strategy

## 1. Objective
To ensure that SunLionet binaries, configurations, and documentation remain accessible to users even in environments where primary distribution channels (e.g., GitHub, official website) are actively blocked or throttled.

## 2. Distribution Tiers

### Tier 1: Primary Official Sources
- **Official Website**: `https://sunlionet.org` (Hosted on IPFS/Arweave + CDN)
- **GitHub Releases**: `https://github.com/SunLionet/Iran-Agent-Vpn/releases`
- **F-Droid**: Official repository (for Android APK)

### Tier 2: Official Mirrors (Managed)
- **Telegram Channel**: `@SunLionetReleases` (Automated upload of artifacts + RELEASES.json)
- **IPFS Gateway**: `https://gateway.ipfs.io/ipns/sunlionet.org`
- **Onion Service**: `http://sunlionet...onion` (For extreme censorship environments)

### Tier 3: Community Mirrors (Unmanaged)
- **BitTorrent**: Official magnet links for large release bundles.
- **Trusted Relay Mirrors**: Third-party repositories (e.g., University mirrors) that sync with GitHub.

## 3. Verification and Trust
To distinguish official mirrors from unsafe/malicious ones, all distribution channels must provide:
1. **SHA-256 Checksums**: Published in the `RELEASES.json` signed by the SunLionet release key.
2. **GPG Signatures**: Every binary is accompanied by a `.sig` or `.asc` file.
3. **In-App Verification**: The Android app verifies the signature of imported bundles before activation.

## 4. Resilience Actions
- **Auto-Mirroring**: GitHub Actions automatically pushes new releases to the Telegram bot and IPFS.
- **Censorship Detection**: The website includes a "Can't reach us?" section with alternative links and a QR code for the Telegram channel.
- **DNS-over-HTTPS (DoH)**: Official domains use DoH-ready providers to prevent DNS poisoning.

## 5. User Guidance
During a source outage, users should:
1. Check the official Telegram channel for the latest working mirror link.
2. Verify downloaded files using the `sha256sum -c` command against the hashes found in the Telegram post.
3. Use the built-in "Check for Updates" feature in the app, which iterates through a prioritized list of mirror URLs.
