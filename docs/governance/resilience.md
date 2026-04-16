# Resilience Plan (Takedown-Resistant, Fork-Friendly)

## Threats to Survive

- Repository takedown on a primary host.
- Domain blocking or DNS tampering for primary website.
- Loss of a maintainer or multiple maintainers.
- Pressure on contributors or attempts to introduce backdoors.
- Disrupted distribution channels and payment rails.

## Principles

- Prefer multiple independent channels over any single “official” channel.
- Make verification independent of where artifacts are hosted.
- Assume hostile environments for distribution and communication.
- Encourage forks and independent builds without fracturing trust roots silently.

## Repo Takedown Recovery

### Mirror Strategy

- Maintain at least:
  - one major forge mirror (e.g., GitHub)
  - a second forge mirror (e.g., GitLab)
  - an optional self-hosted git mirror
- Mirrors should include:
  - tags and release notes
  - governance and verification docs
  - maintainer public keys and fingerprints

### If the Primary Repo Disappears

- Mirrors become authoritative as long as:
  - tags match signatures
  - release manifests and checksums validate
  - maintainer keys are unchanged and published in multiple locations

## Domain Blocking Recovery

### Website Resilience

- Keep documentation mirrored and downloadable.
- Prefer static sites and static exports that can be re-hosted anywhere.
- Publish docs snapshots as part of release assets (so docs travel with binaries).

### Discovery Without Domains

- Provide multiple “how to find the project” hints in docs:
  - mirror URLs
  - key fingerprints
  - offline verification steps

## Distribution Resilience

- Provide multiple download sources for the same signed artifacts.
- Encourage community re-hosting of release assets as long as they remain byte-identical and verifiable.
- Support offline distribution:
  - bundles and binaries transferable by USB/Bluetooth/local file sharing
  - verification steps that do not require network access

## Maintainer Loss and Handover

- Maintain a minimum maintainer set; keep release process documented.
- Require 2-of-N signatures for releases to avoid single-key dependency.
- Keep signing keys independent; no shared private keys.
- Rotate responsibilities:
  - release manager role rotates per release
  - SRG membership rotates on a schedule

## Backdoor and Compromise Resistance

### Process Controls

- Security-critical changes require SRG review.
- Avoid “drive-by” merges for sensitive modules.
- Require tests or reproducible verification steps for sensitive changes.

### Operational Controls

- Release artifacts must be generated from tagged commits and verified by more than one person.
- Publish checksums and signed manifests; discourage “download and trust”.

## Communication Resilience

- Avoid dependence on a single communication platform.
- Maintain at least:
  - a public discussion channel on the primary forge
  - a mirrored announcement channel on a second platform
  - a privacy-respecting messaging fallback for high-risk users
- Keep safety guidance available offline in releases and docs exports.

## Funding Resilience (Optional)

- Prefer privacy-friendly donations and aligned grants.
- Avoid ads, tracking monetization, and dependence on a single funder.
- Keep funding transparent:
  - public reporting of funding sources when safely possible
  - avoid giving funders control over trust roots or governance

## Incident Playbooks

### Mirror Compromise Suspected

- Do not rotate trust roots silently.
- Publish an incident note in all remaining channels.
- Re-issue verified keys and manifests only after multi-party review.

### Signing Key Compromise Suspected

- Freeze releases.
- Publish revocation notice in all mirrors and channels.
- Rotate keys with multi-party confirmation and updated fingerprints.

