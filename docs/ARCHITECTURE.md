# Architecture

Nostromo is organized as a strict, dependency-free layering. Every package only
imports packages below it (and the Go standard library).

```
cmd/nostromo        CLI: serve / keygen / validate / replay
        |
internal/server     HTTP routing, NIP-11, health/metrics, embedded console
        |
internal/relay      business logic: subscriptions, validation, fan-out, NIP-42
        |
   +----+----+
internal/store          internal/ws
memory index +          RFC 6455 framing
append-only JSONL       (upgrade, frames, fragments)
        \               /
         \             /
        internal/protocol
        Event, Filter, message vocabulary
                 |
        internal/crypto
        secp256k1 + BIP-340 Schnorr
```

## Request flow (WebSocket upgrade)

1. `server` detects the `Upgrade: websocket` headers and calls `ws.Upgrade`,
   which validates the handshake, computes `Sec-WebSocket-Accept`, hijacks the
   connection and returns a framed `*ws.Conn`.
2. `relay.Serve` runs the per-connection read loop. Each frame is decoded into
   a `protocol.ClientMessage`.
3. `EVENT` messages are validated (id + Schnorr signature), persisted (unless
   ephemeral) and acknowledged with a NIP-20 `OK`, then broadcast.
4. `REQ` registers a subscription, returns the stored match followed by
   `EOSE`, and stays open for live events.

## Event lifecycle

```
client EVENT
   -> protocol.Event.Validate   (canonical id + BIP-340 signature)
   -> expiration / kind checks
   -> store.Add
        regular            -> insert (dedup by id)
        replaceable/NIP-33 -> supersede prior ReplaceKey
        deletion NIP-09    -> remove referenced same-author events
        ephemeral NIP-16   -> not stored
   -> OK (NIP-20)
   -> relay.broadcast -> every matching live subscription
```

## Storage model

The live set is an in-memory index (`byID` plus a replace-key map). When a file
path is configured, every accepted event is appended to an append-only JSONL
log — the "event log" core. On startup the log is replayed, which naturally
rebuilds replaceable state and applies deletions. On graceful shutdown the log
can be compacted so it contains only the current live events.

## Concurrency

- Each connection is served by its own goroutine (the standard `net/http`
  model).
- The store and the live-client registry are guarded by `sync.RWMutex`.
- Every `ws.Conn` has a write mutex, so concurrent fan-out cannot interleave
  frames.
- Counters use `sync/atomic`.

## Design constraints

- **Standard library only.** WebSocket framing and secp256k1/Schnorr are
  implemented in-tree to keep the binary self-contained and auditable.
- **One binary, no runtime dependencies**, trivially cross-compiled via
  `make release`.
