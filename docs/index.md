<p align="center">
  <img src="../assets/brand/sunlionet-color-256.png" alt="SunLionet" width="128" height="128" />
</p>

# SunLionet

SunLionet is an offline-first, bundle-based privacy and resilient communication system designed for high-risk, restricted networks (including users in Iran). It ships as two coordinated binaries from one shared codebase.

Migration note: this repository is still named `SUNLIONET-agent` during the transition, so some internal identifiers may still use legacy `SUNLIONET-*` naming.

- English docs: this site
- فارسی: [صفحه فارسی](/fa/docs)

## Inside vs Outside

| | SunLionet Inside | SunLionet Outside (Helper) |
|---|---|---|
| Primary role | Maintain connectivity locally | Generate and distribute fresh seeds |
| Network access | Restricted/unstable | Unrestricted |
| Risk | Very high (seizure) | Lower |
| Default data flow | Receive-only | Send bundles to Inside |
| LLM usage | Bounded advisor (sparse calls) | Heavier generation/testing allowed |
| Offline mode | Bluetooth mesh fallback | N/A |

## Key documents

- Getting started: [getting-started.md](getting-started.md)
- Production map: [production-map.md](production-map.md)
- Architecture: [architecture.md](architecture.md)
- Installation: [install.md](install.md)
- sing-box profiles & mutation strategy: [profiles.md](profiles.md)
- Bundle format (sign/encrypt/version): [bundle-format.md](bundle-format.md)
- Signal protocol: [signal.md](signal.md)
- Threat model: [threat-model.md](threat-model.md)
- Security boundaries: [security/boundaries.md](security/boundaries.md)
- Security audit checklist (engineering): [security/checklist.md](security/checklist.md)
- Release blockers: [release/checklist.md](release/checklist.md)
- Pi orchestrator integration (local, optional): [orchestrator/pi-integration.md](orchestrator/pi-integration.md)
- LLM setup notes: [LLM_SETUP.md](LLM_SETUP.md)
- Local skills and experiments (non-release): [web/phase6-blockchain-skills-installation.md](web/phase6-blockchain-skills-installation.md)

## Reports

- Iran internet blackout: https://github.com/kaveh8866/Sunlionet/blob/main/wiki/Iran-Internet-Blackout.md
