# ShadowNet Agent

ShadowNet is an offline-first censorship-circumvention agent built for high-risk DPI environments (e.g., Iran). It ships as two coordinated binaries from one shared codebase.

- English docs: this site
- فارسی: [صفحه فارسی](fa/index.md)

## Inside vs Outside

| | ShadowNet Inside (Iran) | ShadowNet Outside (Exile/Helper) |
|---|---|---|
| Primary role | Maintain connectivity locally | Generate and distribute fresh seeds |
| Network access | Restricted/unstable | Unrestricted |
| Risk | Very high (seizure) | Lower |
| Default data flow | Receive-only | Send bundles to Inside |
| LLM usage | Bounded advisor (sparse calls) | Heavier generation/testing allowed |
| Offline mode | Bluetooth mesh fallback | N/A |

## Key documents

- Getting started: [getting-started.md](getting-started.md)
- Architecture: [architecture.md](architecture.md)
- Installation: [install.md](install.md)
- sing-box profiles & mutation strategy: [profiles.md](profiles.md)
- Bundle format (sign/encrypt/version): [bundle-format.md](bundle-format.md)
- Signal protocol: [signal.md](signal.md)
- Threat model: [threat-model.md](threat-model.md)
- LLM setup notes: [LLM_SETUP.md](LLM_SETUP.md)

## Reports

- Iran internet blackout: [iran-internet-blackout.md](iran-internet-blackout.md) (PDF: [iran-internet-blackout.pdf](iran-internet-blackout.pdf))
