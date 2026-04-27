# Phase 8 — Local Secure Mesh (Unified Layer)

This document describes the unified mesh subsystem produced in Phase 8 (Prompt 5): proximity mesh + hybrid transports + long-range alternatives, with security and resilience hardening.

## Architecture

Layering (conceptual):

Application Layer
Secure Messaging Layer (Phase 4+)
Mesh Routing Layer (gossip + partition repair)
Transport Abstraction Layer (adaptive selection + multipath)
BLE | Wi-Fi Direct | LoRa | Opportunistic relays

Code layout (concrete):

- core/mesh/identity: identity aliases (rotating ephemeral node identity)
- core/mesh/routing: routing/message type aliases for integration points
- core/mesh/transport: transport alias (fused transport backend)
- core/mesh/relay: relay scoring types (trust/abuse signals)
- core/mesh/security: mesh policy profiles (stealth/balanced/resilience)
- core/mesh/partition_sync: inventory/want/cover kinds (partition repair)
- core/mesh/mesh_controller: orchestration entrypoint

Primary implementation components:

- Message router: core/proximity/message_router (chunking, reassembly, gossip, replay, partition sync)
- Anti-abuse: core/proximity/anti_flood_guard + core/proximity/relay_scoring quarantine
- Transport fusion: core/transport/transport_manager (selector-driven multipath + preference)
- Transports: BLE and Wi‑Fi Direct via core/transport/bridge_node_logic, LoRa via experimental/lora/lora_transport

## Orchestration Contract

The mesh controller provides one orchestration surface:

- chooses transport paths (selector profiles + preference)
- sends messages into the mesh router
- runs the receive loop that drives propagation
- applies security policy (cover traffic + multipath/profile switching)

Phase 4+ integration uses pkg/mesh.Mesh:

- pkg/mesh/proximity implements Broadcast/Receive over the Phase 8 proximity mesh

## Self-Healing Behaviors

- Link failure survival: transport selection automatically moves between available radios.
- Partition repair: inventory/want exchange requests missing messages across partitions once a bridge path exists.
- Continuity: store-in-cache + replay protection prevents duplicate amplification while still allowing catch-up.

## Security Hardening Review

Threats addressed:

- Flooding attacks: token bucket rate limiting (global and per-peer) + defensive dropping.
- Relay poisoning: relay scoring with invalid/limited tracking and quarantine windows.
- Replay attacks: replay bucket/window checks on message IDs.
- Malformed frames: invalid-frame penalties and isolation.
- Metadata leakage / traffic analysis: rotating ephemeral identities; optional cover traffic mode.

Threats partially addressed (needs later-phase trust/identity work):

- Sybil-style node abuse: mitigated by per-neighbor throttles/quarantine but not a complete Sybil defense.

## Privacy Hardening

- Ephemeral identities: core/proximity/identity_manager rotates keys and derives a changing NodeID.
- Linkability reduction: no stable routing fingerprints are required to forward; optional cover traffic reduces idle/active leakage.

## Resource Optimization Notes

- Cover traffic scheduling is event-driven: no periodic wake-ups when disabled; enabling/disabling cover traffic signals the router loop to reschedule.
- Rate limits bound CPU/memory churn under adversarial load (drop early).

## Final Scenario Test Matrix (A–E)

Implemented in core/mesh/mesh_controller/mesh_controller_final_sim_test.go:

- A: Bluetooth-only mesh delivery
- B: Partition healed via bridge nodes (LoRa bridge)
- C: Flooding + invalid frames with graceful degradation (legit message still delivered)
- D: Transport failures with automatic rerouting/fallback
- E: Mixed BLE + Wi‑Fi + LoRa fallback survival

