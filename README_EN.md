<div align="center">

# 🛰️ Nostromo

### A zero-dependency, single-binary Nostr event-log relay for developers and AI agents

**Language:** [简体中文](README.md) · [繁體中文](README_TW.md) · [English](README_EN.md)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report](https://img.shields.io/badge/go%20report-standard%20library-00ADD8?logo=go&logoColor=white)](#-tech-stack)
[![Release](https://img.shields.io/badge/release-v1.0.0-blue.svg)](#-download)

</div>

> **⬇️ Download:** Prebuilt binaries are on the [**Releases page**](https://github.com/gitstq/nostromo-relay/releases) for Windows / macOS / Linux (amd64 + arm64).

---

## 🎉 Introduction

**Nostromo** is a **batteries-included, lightweight [Nostr](https://nostr.com/) relay**. It models every message as a **cryptographically signed, append-only event** — whether the author is a person or an AI agent, they share the same data shape, identity model and audit trail.

It addresses real pain points:

- 🧰 **Existing relays are heavyweight**: mainstream implementations depend on databases and elaborate configuration, which makes spinning one up to debug a Nostr client/agent painful.
- 👀 **The process is opaque**: developers need to watch the event stream in real time, filter it and verify behavior quickly rather than face a black box.
- 🔁 **Tests are hard to reproduce**: there is no convenient way to record and replay events.
- 🔐 **Identity and permissions are tangled**: you want to scope an agent with its own keypair the way you would a teammate, not with a pile of permission flags.

**Differentiators:**

- 🪶 **Truly zero-dependency**: it uses only the Go standard library and hand-implements **RFC 6455 WebSocket** plus **secp256k1 / BIP-340 Schnorr**, shipping as a single ~5 MB static binary.
- 🧪 **Built for developers and agents**: an embedded **real-time console**, **event record/replay** and explicit command results — embeddable and assertable.
- ⚡ **One command builds for every platform**: native cross-compilation to Windows / macOS / Linux (amd64 + arm64) with no runtime dependencies.

💡 **Inspiration:** Nostromo references the product philosophy of `block/buzz` on GitHub Trending — a **self-owned, signed event-log relay** (Nostr is the open NIP standard). Unlike Buzz, which is a 581 MB full collaboration workspace, Nostromo focuses on a **lightweight, zero-dependency, observable and reproducible** relay core, with all code written independently. See the [research brief](docs/RESEARCH_BRIEF.md).

---

## ✨ Features

- 📨 **Full NIP-01 protocol**: publish (`EVENT`), subscribe (`REQ`), close (`CLOSE`), with arbitrary `#e / #p / #a / #t …` tag queries (**NIP-12**).
- 🔁 **Replaceable & ephemeral events (NIP-16)**: metadata and friends are handled automatically; ephemeral events are **broadcast live but never persisted**.
- 🏷️ **Parameterized replaceable events (NIP-33)**: maintain multiple independent states per author/kind via the `d` tag.
- 🗑️ **Event deletion (NIP-09)**: honors deletion events, removing only same-author referenced events.
- ⏳ **Expiration (NIP-40)**: expired events automatically drop out of query results.
- ✅ **Command results (NIP-20)**: every event returns an explicit `OK` or a failure reason.
- 🪪 **Client authentication (NIP-42)**: Schnorr auth over a random challenge, optionally required before subscribing.
- 🔍 **Full-text search (NIP-50)**: case-insensitive, all-terms AND matching.
- 🖥️ **Embedded real-time console**: open it in a browser to connect, filter and watch the stream, and to send arbitrary raw messages.
- 💾 **Event-log persistence**: append-only JSONL, **replayed on startup** and optionally **compacted on shutdown**.
- 🧰 **Bundled CLI**: four subcommands — `serve / keygen / validate / replay`.
- 🛡️ **Secure defaults**: message-size limits, write serialization and no third-party dependencies keep the attack surface small.

---

## 🚀 Quick Start

### Requirements

- Prebuilt binary: **no runtime required at all** (static binary).
- Build from source: **Go 1.23+** (standard library only; no external dependencies after `go mod tidy`).

### Option 1 — Download (recommended)

Head to [**Releases**](https://github.com/gitstq/nostromo-relay/releases), download the file for your platform, extract and run:

```bash
# Start in memory mode (nothing is persisted), listening on :7777 by default
./nostromo serve --db ""
```

### Option 2 — Build from source

```bash
git clone https://github.com/gitstq/nostromo-relay.git
cd nostromo-relay
make build           # produces bin/nostromo
./bin/nostromo serve
```

### Option 3 — Docker

```bash
docker run --rm -p 7777:7777 -v nostromo-data:/data nostromo:1.0.0
# or with docker compose
docker compose up
```

Once running:

| Entry point | Address |
| --- | --- |
| 🖥️ Console | <http://localhost:7777/> |
| 🔌 WebSocket | `ws://localhost:7777/` |
| 📋 Relay info (NIP-11) | `curl -H "Accept: application/nostr+json" http://localhost:7777/` |
| ❤️ Health check | <http://localhost:7777/healthz> |
| 📊 Metrics | <http://localhost:7777/metrics> |

### See the whole loop in 30 seconds

```bash
# 1) Generate a keypair
./nostromo keygen

# 2) Generate a batch of signed sample events
go run ./examples/seed -out seed.jsonl

# 3) Replay them into the running relay
./nostromo replay --to ws://127.0.0.1:7777 seed.jsonl

# 4) Validate the file
./nostromo validate seed.jsonl
```

---

## 📖 Usage Guide

### `serve` — start the relay

```bash
nostromo serve [flags]
```

| Flag | Default | Description |
| --- | --- | --- |
| `--addr` | `:7777` | Listen address |
| `--db` | `data/events.jsonl` | Event-log file; pass `""` for memory only |
| `--name` | `Nostromo` | Relay name |
| `--desc` | built-in | Relay description |
| `--contact` | empty | Contact (email or URL) |
| `--pubkey` | empty | Operator pubkey |
| `--require-auth` | `false` | Require NIP-42 auth before subscribing |
| `--compact` | `true` | Compact the event log on graceful shutdown |

### `keygen` — generate keys

```bash
nostromo keygen
# private key (keep secret): 7a256d3d...
# public key (x-only)      : e45823fd...
```

### `validate` — validate events

Accepts a single event JSON or multi-line JSONL, and can read from stdin:

```bash
nostromo validate event.json
cat events.jsonl | nostromo validate -
```

### `replay` — replay events

Publishes the events in a file to the target relay in order and prints each `OK` result:

```bash
nostromo replay --to ws://127.0.0.1:7777 events.jsonl
```

### Typical scenarios

- 🧑‍💻 **Local development of a Nostr client/agent**: run with `--db ""` for a zero-state, resettable local relay.
- 🤖 **An observable event bus for AI agents**: watch the agent's events in the console and reproduce issues via replay.
- 🏠 **Personal/community self-hosting**: run long-term with file persistence plus Docker/systemd.

### Client example (JavaScript)

```javascript
const ws = new WebSocket("ws://localhost:7777/");
ws.onopen = () => {
  // Subscribe to all kind 1 text notes
  ws.send(JSON.stringify(["REQ", "my-sub", { kinds: [1], limit: 100 }]));
};
ws.onmessage = (ev) => {
  const [type, ...rest] = JSON.parse(ev.data);
  console.log(type, rest); // EVENT / EOSE ...
};
```

> 📸 **Screenshot/GIF placeholder**: console screenshots can live in `docs/assets/` and be referenced here.

---

## 💡 Design & Roadmap

**Design philosophy:**

- 🧱 **Events are the log**: every action is the same kind of signed event, which yields a unified identity and auditability by construction.
- 🧅 **Minimal core plus observability**: keep the core small and correct, then make the console and replay first-class developer experiences.
- 🔓 **Zero-dependency as a security property**: with no third-party libraries, the WebSocket and curve math stay in-tree, auditable and reproducible.

**Why this stack?** Go offers a powerful standard library, naturally cross-platform static binaries and cross-compilation by simply setting `GOOS / GOARCH` — a perfect match for the "single file, no runtime dependencies" goal.

**Roadmap:**

- 📈 Richer storage backends and indexes (optional adapters while the core stays dependency-free).
- 🧩 More NIPs (e.g. NIP-13 proof-of-work anti-spam and tooling for NIP-59 sealed messages).
- 🧪 Official multi-language SDK examples and load-testing tooling.
- 📡 Reference patterns for multi-relay mirroring / clustered deployments.

**Ways to contribute:** new NIP support, test cases, documentation translation and performance work are all very welcome — see the [contributing guide](CONTRIBUTING.md).

---

## 📦 Packaging & Deployment

### Build for every platform

```bash
make release          # or ./scripts/build.sh 1.0.0
```

Artifacts land in `bin/release/`:

```
nostromo_linux_amd64 / nostromo_linux_arm64
nostromo_darwin_amd64 / nostromo_darwin_arm64
nostromo_windows_amd64.exe / nostromo_windows_arm64.exe
checksums.txt
```

### Deployment

- **Docker / docker compose**: see [`Dockerfile`](Dockerfile) and [`docker-compose.yml`](docker-compose.yml) in the root.
- **systemd (Linux)**: the unit is at [`deploy/nostromo.service`](deploy/nostromo.service):

  ```bash
  sudo cp bin/nostromo /usr/local/bin/nostromo
  sudo cp deploy/nostromo.service /etc/systemd/system/
  sudo useradd -r -s /usr/sbin/nologin nostromo
  sudo mkdir -p /var/lib/nostromo && sudo chown nostromo:nostromo /var/lib/nostromo
  sudo systemctl enable --now nostromo
  ```

- **Reverse proxy / TLS**: the relay honors `X-Forwarded-Proto` and can sit behind Caddy / Nginx to expose `wss`.

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the internals.

---

## 🤝 Contributing

Issues, PRs and translations are welcome! Commit messages follow Conventional Commits:

- `feat: add …` / `fix: …` / `docs: …` / `refactor: …` / `test: …`

Run the quality gate before submitting:

```bash
make check   # gofmt + go vet + go test
```

Full guidelines: [**CONTRIBUTING.md**](CONTRIBUTING.md).

---

## 📄 License

Released under the **[MIT License](LICENSE)** — free to use, modify and distribute. Nostr is an open protocol; the NIP standards are copyrighted by their respective authors.

<div align="center">

**Made with 🛰️ by [gitstq](https://github.com/gitstq)**

</div>
