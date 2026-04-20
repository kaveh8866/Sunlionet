---
name: "network-security-lab-setup"
description: "Sets up a controlled network security lab (HTTP/SNMP/SMB targets) and validates with curl/nmap. Invoke when you need an isolated lab for enumeration/testing."
---

# Network Security Lab Setup (Local, Controlled)

This skill provides practical, repeatable steps to bring up a **safe, local lab** for network security exercises.

## Safety Rules

- Use only **isolated** lab networks or hosts you own/are authorized to test.
- Avoid scanning shared corporate/Wi‑Fi networks.
- Never store secrets (community strings, passwords, auth keys) in git.

## Dependencies (Windows)

Required:

- `curl.exe`
- `python` (for a quick HTTP target)

Recommended:

- `nmap` (enumeration/verification)

Checks:

```powershell
where.exe curl
python --version
where.exe nmap
```

If `nmap` is missing:

```powershell
winget install --id Insecure.Nmap -e --accept-source-agreements --accept-package-agreements
```

## Minimal Lab: HTTP Target (No Admin)

Start a local HTTP server in a dedicated folder:

```powershell
mkdir lab-http
cd lab-http
python -m http.server 18080
```

Validate from another terminal:

```powershell
curl.exe -v http://127.0.0.1:18080/
```

Optional port verification (authorized):

```powershell
nmap -Pn -p 18080 127.0.0.1
```

Expected:

- `curl` returns `HTTP/1.0 200 OK` (or `HTTP/1.1 200 OK`)
- `nmap` shows `18080/tcp open`

Stop server with Ctrl+C.

## Extended Targets (Admin / Optional)

These require elevated privileges and are optional:

### SMB Share Target (Windows)

Use a dedicated local test user and a dedicated folder. Validate with:

```powershell
nmap -Pn -p 445 <target-host>
```

### SNMP Target (Windows/Linux)

Prefer a dedicated VM/WSL/Docker container. Validate with:

```powershell
nmap -sU -Pn -p 161 <target-host>
```

## “Lab Initialized” Definition (Pass Criteria)

- Required tools exist (`curl`, `python`; optionally `nmap`)
- HTTP target starts without errors
- `curl` succeeds against the target endpoint
- If `nmap` is used, required ports appear open on the lab target
