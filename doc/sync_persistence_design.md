# SunLionet Phase 4: Persistence, Synchronization, and Multi-Device State Layer Design

## 1. Current State Mapping (Repo-Aware)

- **Local Storage Model**: Currently, state is persisted in monolithic JSON files encrypted with AES-256-GCM (`identity.enc`, `chat.enc`, `community.enc`, `profile.enc`, `policy.enc`). Each `Store` reads the entire file on `Load()` and rewrites the entire file on `Save()`. This approach is simple but doesn't scale for large conversation histories.
- **Multi-Device Sync (`pkg/devsync`)**: The repo contains an event-sourced synchronization primitive (`devsync.Service`) utilizing Lamport clocks, sequence numbers, and an outbox queue. However, it is imported but currently **unused** in the mobile integration layer (`pkg/mobile/mobile.go`).
- **Device Linking (`pkg/identity/device_link.go`)**: Primitives exist for generating a `DeviceJoinRequest` and returning a signed `DeviceJoinPackage` containing a `DeviceCert`. However, the flow lacks secure out-of-band key transfer (e.g., sharing the `MasterKey` or `Persona` private keys securely).
- **Messaging Delivery (`pkg/relay`)**: Messages are sent to and polled from mailboxes. Currently, there is no robust offline queueing or retry mechanism integrated with the user-facing APIs.

## 2. Local Data Storage Model

To maintain the constraint of "NO redesigning crypto primitives from scratch" and "Reuse the current repository structure," the storage model must evolve iteratively:
- **Phase 4 MVP**: Retain the AES-GCM encrypted JSON blobs but introduce **sharding by conversation**. Instead of one monolithic `chat.enc`, we use `chat_index.enc` (metadata, unread counts) and `chat_{thread_id}.enc` (message arrays bounded to the last 1,000 messages). 
- **Long-term**: Migrate to SQLCipher (SQLite with 256-bit AES encryption) using the `master_key` to derive the database key. This provides indexing and query capabilities without loading the entire state into memory.

## 3. Conversation Persistence Model

Conversations will be managed via two primary structures:
- **Conversation Index**: Stores thread metadata (thread ID, participants, last activity timestamp, unread count, thread type).
- **Message Log**: An append-only structure per thread. Messages have explicit states:
  - `Draft`: Written locally, not yet queued.
  - `Outbox`: Queued for relay delivery (tracked in `devsync.Outbox`).
  - `Sent`: Acknowledged by the relay.
  - `Delivered`: Acknowledged by the recipient's device (requires delivery receipts).
  - `Read`: Acknowledged as read by the recipient.

## 4. Multi-Device Synchronization Strategy

We will adopt an **Event-Sourced, Device-Centric** sync model leveraging the existing `pkg/devsync` package.
- **Lamport Clocks & Seq Numbers**: Each device maintains its sequence number and a Lamport clock. Changes to local state (e.g., message read, contact added, persona updated) are recorded as `devsync.Event` entries.
- **Sync Mailbox**: Each persona maintains a dedicated "Sync Mailbox" on the relay. Devices publish `devsync.Batch` payloads encrypted to the Persona's shared sync key (or pairwise device keys) to this mailbox.
- **Conflict Resolution**: Last-Writer-Wins (LWW) based on Lamport timestamps, with ties broken by `DeviceID`.

## 5. Device Linking Flow

The current `DeviceJoinRequest` / `DeviceJoinPackage` flow requires an out-of-band channel to transfer the `master_key`. We will use a **QR Code based authorization**:
1. **New Device**: Generates a temporary Ed25519 keypair and displays a QR code containing `sn4dj:base64(...)` (the `DeviceJoinRequest`).
2. **Primary Device**: Scans the QR code, verifies the request, and generates a `DeviceCert`.
3. **Key Transfer**: The primary device encrypts the `master_key` and Persona private keys using the New Device's temporary public key.
4. **Fulfillment**: The primary device packages the `DeviceJoinPackage` and the encrypted keys into a payload and transmits it over a temporary relay mailbox or local network connection.
5. **Onboarding**: The New Device decrypts the package, applies the master key, and begins syncing the state via `devsync`.

## 6. Message Synchronization

- **Fan-out Encryption**: When sending a message, the sender encrypts it pairwise for *every* trusted device of the recipient AND *every* other trusted device of the sender.
- **Relay Delivery**: The encrypted envelopes are pushed to the recipient's mailbox.
- **Deduplication**: Devices use a combination of `MessageID` and `devsync.Seen` maps to silently discard duplicate deliveries.

## 7. Offline-First Behavior

- **Local Writes First**: All user actions (sending messages, creating contacts, modifying profiles) are immediately written to the local store and appended to the `devsync.Outbox`.
- **Background Sync**: A background worker continuously monitors network connectivity. When online, it attempts to flush the `devsync.Outbox` to the relay.
- **Exponential Backoff**: The `devsync.Service.MarkEventRetry()` mechanism is used to handle relay failures with exponential backoff.

## 8. Backup & Recovery Model

- **Seed Phrase / Master Key**: The user's `master_key` acts as the root of trust. We generate a BIP39-style mnemonic phrase encoding the 32-byte master key.
- **Encrypted Cloud Backups (Optional)**: Users can export a monolithic bundle of their `.enc` files. This bundle is symmetrically encrypted using a key derived from the master key and can be stored in any cloud provider.
- **Recovery**: Entering the seed phrase restores the master key, allowing the user to decrypt their backup bundle or link as a new device to an existing active device.

## 9. Data Retention & Lifecycle

- **Self-Destructing Messages**: Messages can carry an `ExpiresAt` attribute. The local `Load()`/`Save()` pruning logic will strictly drop messages where `now > ExpiresAt`.
- **Local Storage Limits**: The `devsync.Prune()` logic currently bounds events to 50,000. Chat threads will similarly implement a rolling window (e.g., retaining only the last 10,000 messages per thread) to prevent unbound disk usage.
- **Account Deletion**: Wipes the `stateDir` entirely and broadcasts a revocation event to the relay to tear down mailboxes.

## 10. Security Considerations

- **Device Compromise**: If a device is lost, the primary device issues a revocation event. The lost device's keys are removed from the Persona's trusted device list, preventing it from receiving future messages. The relay drops its active sessions.
- **No Plaintext**: No unencrypted data is ever written to disk. The `master_key` is kept only in memory during the application lifecycle and is securely wiped on exit.
- **Forward Secrecy**: The existing prekey infrastructure guarantees that even if a device key is compromised, past messages cannot be decrypted.

## 11. Integration with Existing Code

- **Storage**: Modify `chat.Store` to shard conversations.
- **Mobile Bridge**: Expose `devsync` via `Bridge.kt` and `mobile.go` so the Android UI can trigger explicit syncs.
- **Identity**: Extend `mobile.go` to support `LinkDevice(qrCode string)` and `ApproveDevice(joinRequest string)`.
- **Relay**: Wire `devsync.Outbox` to `relay.Sender`.

## 12. Phase 4 MVP Scope

For the immediate Phase 4 release, the scope is constrained to:
1. **Encrypted Local Storage**: Keep the current JSON-GCM monolithic files but ensure they are correctly flushed on lifecycle events.
2. **Two-Device Support**: Enable one primary device and one linked device using the QR-based device linking flow.
3. **Basic Multi-Device Sync**: Use `devsync` to replicate new outgoing messages and contact additions between the two devices.
4. **Offline Queueing**: Wire the `devsync.Outbox` to ensure messages composed offline are sent when the network is restored.
