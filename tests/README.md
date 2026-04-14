# Local Testing Environment for ShadowNet Agent

This directory contains the tools needed to test the ShadowNet Agent locally, without needing to be connected to the actual Iranian national intranet.

## Components

1. **Docker Compose (`docker-compose.yml`)**: Spawns two standard `sing-box` server endpoints.
   - **Reality Server**: Listens on TCP `8443`.
   - **Hysteria2 Server**: Listens on UDP `9443`.
2. **Mock DPI Injector (`tests/mock_dpi/main.go`)**: A Go-based L4 proxy that intercepts traffic on port `443` (TCP/UDP) and forwards it to the Docker containers. It randomly drops UDP packets and sends TCP RSTs (by closing connections with `SO_LINGER=0`) to simulate Iranian DPI interference.

## How to Run the Test

### Step 1: Start the Backend Servers
You must have Docker and Docker Compose installed.

```bash
docker-compose up -d
```

Verify they are running:
```bash
docker ps | grep sing-box
```

### Step 2: Start the Mock DPI Proxy
This proxy acts as the "Iranian Firewall". It binds to local port 443.

```bash
cd tests/mock_dpi
# Run with a 15% drop/RST rate (adjustable via -drop flag)
go run main.go -drop 0.15 -listen-tcp :443 -listen-udp :443
```
*Note: If port 443 requires sudo on your system, run it as `sudo go run main.go ...` or change the listen ports to something like `:8444`.*

### Step 3: Run the Inside Agent
In a separate terminal, start the main ShadowNet Inside agent.
Modify the agent's seed config (`bundle.snb.json` or your mocked initial state) to point to the **Mock DPI Proxy** (e.g., `127.0.0.1:443`).

```bash
go run cmd/inside/main.go
```

### Step 4: Observe the Pipeline
1. The Inside agent will attempt to connect to the proxy via Reality (TCP).
2. The `Mock DPI` proxy will eventually roll its 15% chance and drop the connection with a TCP RST.
3. The `detector` module in the Inside agent will log a `CheckSNIReset` or generic TCP drop event.
4. If 3 anomalies occur within 60 seconds, the Go daemon wakes the LLM.
5. You will see the LLM generate a new JSON action plan (e.g., switching to Hysteria2).
6. `generator.go` creates the new config pointing to the UDP proxy.
7. `sbctl` hot-reloads `sing-box` with the new config.
8. The Mock DPI proxy will now intercept the UDP traffic and randomly drop packets, testing the UDP fallback.
