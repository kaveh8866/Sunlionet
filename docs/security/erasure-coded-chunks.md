# Erasure-Coded Configuration Chunks

SunLionet configuration bundles remain globally signed as complete bundle bytes.
The chunk layer is only a transport reliability layer: it never rewrites,
canonicalizes, or signs the reconstructed bundle. After reconstruction, the
normal Ed25519 bundle verifier must run on the recovered byte stream.

## Encoding Model

Given payload `P`, choose:

- `N`: number of data shards.
- `M`: number of parity shards.
- `S`: fixed shard byte length.

The payload is split into `N` data shards of length `S`, with zero padding in the
final shard. Parity shards are generated over GF(256) using a systematic
Reed-Solomon-style Cauchy matrix:

```text
data row i      = unit vector e_i, for 0 <= i < N
parity row p    = [1 / (x_p + y_0), ... 1 / (x_p + y_(N-1))]
x_p             = p + 1
y_j             = 128 + j
total chunks    = N + M
```

Any `N` distinct valid chunks form an invertible `N x N` submatrix. The client
inverts that matrix, solves for the original `N` data shards, removes padding
using `PayloadSize`, checks `PayloadSHA256`, then passes the final bytes to
the existing bundle verifier.

## Chunk Envelope

Each binary chunk is:

| Field | Size | Description |
| --- | ---: | --- |
| Magic | 4 | ASCII `SNCE` |
| Version | 1 | `0x01` |
| Flags | 1 | Reserved, currently zero |
| BundleID | 16 | First 16 bytes of `SHA256(payload)` |
| Index | 2 | Big-endian monotonic shard index, `0 <= index < N+M` |
| DataShards | 2 | Big-endian `N` |
| ParityShards | 2 | Big-endian `M` |
| PayloadSize | 4 | Big-endian original payload length |
| ShardSize | 2 | Big-endian fixed shard size `S` |
| PayloadSHA256 | 32 | Global digest of final bundle bytes |
| ChunkSHA256 | 32 | Digest over header metadata plus shard bytes |
| Data | `S` | Data or parity shard bytes |

Text transports can wrap the binary chunk as:

```text
SNBEC/1 <base64url(binary-chunk)>
```

## Reassembly State Machine

The client accepts chunks in any order. For each `BundleID`, it caches only
distinct chunks with matching matrix metadata and matching localized checksums.
The default cache budget is 2 MiB. If the cache would exceed its budget, the
partial bundle is discarded fail-closed.

Once any `N` valid chunks are present:

1. Select `N` distinct chunk rows.
2. Invert the corresponding GF(256) encoding matrix.
3. Recover the original data shards.
4. Truncate to `PayloadSize`.
5. Verify `PayloadSHA256` and `BundleID`.
6. Flush the temporary chunk cache.
7. Run normal Ed25519 bundle verification on the reconstructed bytes.

Parity chunks are cryptographically isolated from signing: they help recover
transport bytes, but only the final reconstructed bundle receives trust.
