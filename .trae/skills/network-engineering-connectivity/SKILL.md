---
name: "network-engineering-connectivity"
description: "Helps diagnose connectivity and mesh/VPN issues using ssh/curl/nmap/tailscale. Invoke when debugging network reachability, tunnels, relays, or topology."
---

# Network Engineering & Connectivity

This skill focuses on practical, privacy-conscious connectivity diagnostics for distributed systems and mesh networks.

## What This Skill Covers

- Connectivity triage (DNS, TCP/UDP reachability, TLS, latency)
- HTTP API probing with curl
- SSH access + SOCKS/port-forward patterns
- Network discovery/port scanning with nmap (only on networks you own/are authorized to test)
- Tailscale mesh VPN checks (node presence, routes, exit nodes, ACL sanity)

## Compatibility (Windows)

Baseline tools expected:

- `curl` (Windows ships `curl.exe` on modern builds)
- `ssh` (Windows OpenSSH client)
- Optional but recommended:
  - `tailscale`
  - `nmap`

Quick checks:

```powershell
where.exe curl
where.exe ssh
where.exe tailscale
where.exe nmap
```

## Installing Missing Dependencies (Windows)

If `tailscale` and/or `nmap` are missing, install with **Windows Package Manager** (recommended) or Chocolatey.

### Option A: winget

Run in an elevated PowerShell:

```powershell
winget install --id Tailscale.Tailscale -e
winget install --id Insecure.Nmap -e
```

Verify:

```powershell
tailscale version
nmap --version
```

### Option B: Chocolatey

Run in an elevated PowerShell:

```powershell
choco install tailscale -y
choco install nmap -y
```

## Topology Parameters (Local-Only)

Keep topology configuration local (do not commit secrets).

Capture:

- **Your node role**: workstation / server / relay / subnet-router / exit-node
- **Addressing**:
  - LAN CIDRs (e.g. `192.168.1.0/24`)
  - Tailscale 100.x node IPs (or MagicDNS names)
- **Critical services**:
  - Host:port list (e.g. `api.internal:443`, `relay:8443`, `db:5432`)
- **Allowed paths**:
  - Direct LAN vs VPN-only vs SSH-tunneled

## Diagnostic Playbook

### 1) Basic reachability

DNS:

```powershell
Resolve-DnsName example.com
```

TCP connect (PowerShell):

```powershell
Test-NetConnection -ComputerName example.com -Port 443
```

HTTP/TLS with curl:

```powershell
curl -v https://example.com/health
curl -v --connect-timeout 5 --max-time 20 https://<host>:<port>/
```

### 2) SSH health + tunneling patterns

Direct SSH:

```powershell
ssh -v user@host
```

SOCKS proxy (local 1080):

```powershell
ssh -D 1080 -N user@bastion
```

Then test via curl:

```powershell
curl --proxy socks5h://127.0.0.1:1080 -v https://example.com/
```

Local port forward:

```powershell
ssh -L 15432:db.internal:5432 -N user@bastion
```

### 3) Tailscale checks (if installed)

Status:

```powershell
tailscale status
```

If you run a subnet router, validate routes are advertised/accepted in admin console and that ACLs permit the flows you test.

### 4) Nmap discovery (authorized networks only)

Quick port check:

```powershell
nmap -Pn -p 22,80,443 <host-or-ip>
```

Subnet scan (keep it minimal):

```powershell
nmap -sn 192.168.1.0/24
```

## Connectivity Validation Checklist (Expected “Pass”)

- DNS resolves expected names (LAN and VPN/MagicDNS)
- TCP connect succeeds to required service ports
- curl health endpoints return expected status codes
- SSH can reach bastion/relay when required
- (If used) Tailscale shows peers up, routes consistent
- (If used) Nmap confirms required ports open and unexpected ports closed

## Integration Notes (Repository)

This skill is a local workflow helper. It does not modify application networking code by itself.

If you need to integrate network checks into CI/tests, add them as explicit, non-privileged, opt-in scripts and avoid any scanning behavior by default.
