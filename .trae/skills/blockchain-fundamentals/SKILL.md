---
name: "blockchain-fundamentals"
description: "Provides practical blockchain primitives and architecture guidance. Invoke when designing ledgers, signatures, Merkle proofs, consensus, or P2P flows."
---

# Blockchain Fundamentals

Use this skill as the technical reference for blockchain protocol design, cryptographic primitives, and distributed-system behavior.

## When To Invoke

- When implementing or reviewing hashes, signatures, or proof structures.
- When defining transaction lifecycle, mempool handling, or confirmation logic.
- When designing P2P propagation, block validation, or consensus behavior.

## Core Capabilities

- Implementation patterns for hashing and digital signatures.
- Merkle tree construction and proof verification guidance.
- PoW / PoS / BFT comparison and system-level tradeoffs.
- Transaction lifecycle analysis and confirmation debugging.
- P2P network and block-propagation architecture references.

## Prerequisites

- Familiarity with byte-level encoding and canonical serialization.
- Reproducible test harness for vectors and deterministic fixtures.
- Security review workflow for cryptographic changes.

## Recommended Workflow

1. Define canonical encoding and hash/sign boundaries.
2. Add deterministic test vectors for cryptographic routines.
3. Validate adversarial cases (replay, malleability, reordering).
4. Model network latency/finality assumptions.
5. Document threat model and invariants before release.

## Validation Checklist

- Canonical serialization is deterministic across platforms.
- Signature verification rejects malformed payloads.
- Merkle proofs verify and reject tampered branches.
- Consensus/finality assumptions are explicit and tested.
- Integration tests cover transaction happy path and failure cases.

