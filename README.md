<div align="center">

# 🛰️ Nostromo

### 零依赖、单二进制的 Nostr 事件日志 Relay —— 为开发者与 AI Agent 而生

**语言 / Language：** [简体中文](README.md) · [繁體中文](README_TW.md) · [English](README_EN.md)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report](https://img.shields.io/badge/go%20report-standard%20library-00ADD8?logo=go&logoColor=white)](#-技术栈)
[![Release](https://img.shields.io/badge/release-v1.0.0-blue.svg)](#-下载)

</div>

> **⬇️ 下载：** 预编译可执行文件请前往 [**Releases 页面**](https://github.com/gitstq/nostromo-relay/releases)，支持 Windows / macOS / Linux（amd64 + arm64）。

---

## 🎉 项目介绍

**Nostromo** 是一个**开箱即用的轻量级 [Nostr](https://nostr.com/) 中继（Relay）**。它把每一条消息都建模为一条**带密码学签名、只可追加的事件**，无论作者是人还是 AI Agent，都拥有统一的数据结构、身份模型与审计轨迹。

它解决了这些痛点：

- 🧰 **现有 Relay 太重**：主流实现依赖数据库与复杂配置，本地调试一个 Nostr 客户端 / Agent 时启动成本高。
- 👀 **过程不可观测**：开发者需要实时看到事件流、按条件过滤、快速验证，而不是面对黑盒。
- 🔁 **测试难以复现**：缺少便捷的事件录制与回放手段。
- 🔐 **身份与权限耦合**：希望像对待团队成员一样，用独立密钥来界定 Agent，而非一堆权限开关。

**自研差异化亮点：**

- 🪶 **真正零依赖**：仅使用 Go 标准库，手写 **RFC 6455 WebSocket** 与 **secp256k1 / BIP-340 Schnorr**，整个程序约一个 5MB 的静态二进制。
- 🧪 **为开发者与 Agent 设计**：内置**实时调试台**、**事件录制 / 回放**、清晰的命令结果，可嵌入、可断言。
- ⚡ **一条命令全平台构建**：原生交叉编译到 Windows / macOS / Linux（amd64 + arm64），无运行时依赖。

💡 **灵感来源**：本项目参考了 GitHub Trending 上 `block/buzz` 所倡导的「**用户自主掌控的签名事件日志 Relay**」这一产品理念（Nostr 为开放标准 NIP）。与 Buzz 那个 581MB 的完整协作工作区不同，Nostromo 聚焦于**轻量、零依赖、可观测、可复现**的 Relay 内核，全部代码独立自研。详见 [预研简报](docs/RESEARCH_BRIEF.md)。

---

## ✨ 核心特性

- 📨 **完整的 NIP-01 协议**：事件发布（`EVENT`）、订阅（`REQ`）、关闭（`CLOSE`），支持任意 `#e / #p / #a / #t …` 标签查询（**NIP-12**）。
- 🔁 **可替换与临时事件（NIP-16）**：自动处理元数据等可替换事件与临时事件，临时事件**实时广播但不落盘**。
- 🏷️ **参数化可替换事件（NIP-33）**：按 `d` 标签维护同一作者、同一类型的多个独立状态。
- 🗑️ **事件删除（NIP-09）**：处理删除事件，仅移除同一作者引用的事件。
- ⏳ **过期时间（NIP-40）**：到期事件自动从查询结果中消失。
- ✅ **命令结果（NIP-20）**：每个事件都返回明确的 `OK / 失败原因`。
- 🪪 **客户端认证（NIP-42）**：基于随机挑战的 Schnorr 认证，可配置为订阅前置条件。
- 🔍 **全文搜索（NIP-50）**：多关键词「与」匹配，大小写不敏感。
- 🖥️ **内置实时调试台**：浏览器打开即可连接、过滤、观察事件流，并可发送任意原始消息。
- 💾 **事件日志持久化**：append-only JSONL，启动时**重放重建**，关闭时可**压缩（compaction）**。
- 🧰 **配套 CLI**：`serve / keygen / validate / replay` 四个子命令，覆盖运行、密钥、校验与回放。
- 🛡️ **安全默认**：消息大小限制、写帧互斥、无第三方依赖，攻击面小。

---

## 🚀 快速开始

### 环境要求

- 运行预编译版本：**无需任何运行时**（静态二进制）。
- 从源码构建：**Go 1.23+**（仅用标准库，`go mod tidy` 后无外部依赖）。

### 方式一：直接下载（推荐）

前往 [**Releases**](https://github.com/gitstq/nostromo-relay/releases)，下载对应平台的文件，解压后直接运行：

```bash
# 内存模式启动（数据不持久化），默认监听 :7777
./nostromo serve --db ""
```

### 方式二：从源码构建

```bash
git clone https://github.com/gitstq/nostromo-relay.git
cd nostromo-relay
make build           # 产物在 bin/nostromo
./bin/nostromo serve
```

### 方式三：Docker

```bash
docker run --rm -p 7777:7777 -v nostromo-data:/data nostromo:1.0.0
# 或使用 docker compose
docker compose up
```

启动后：

| 入口 | 地址 |
| --- | --- |
| 🖥️ 调试台 | <http://localhost:7777/> |
| 🔌 WebSocket | `ws://localhost:7777/` |
| 📋 Relay 信息（NIP-11） | `curl -H "Accept: application/nostr+json" http://localhost:7777/` |
| ❤️ 健康检查 | <http://localhost:7777/healthz> |
| 📊 指标 | <http://localhost:7777/metrics> |

### 30 秒体验完整闭环

```bash
# 1) 生成一对密钥
./nostromo keygen

# 2) 生成一批已签名示例事件
go run ./examples/seed -out seed.jsonl

# 3) 回放到正在运行的 Relay
./nostromo replay --to ws://127.0.0.1:7777 seed.jsonl

# 4) 校验事件文件
./nostromo validate seed.jsonl
```

---

## 📖 详细使用指南

### `serve` —— 启动 Relay

```bash
nostromo serve [flags]
```

| 参数 | 默认值 | 说明 |
| --- | --- | --- |
| `--addr` | `:7777` | 监听地址 |
| `--db` | `data/events.jsonl` | 事件日志文件；传 `""` 为纯内存模式 |
| `--name` | `Nostromo` | Relay 名称 |
| `--desc` | 内置描述 | Relay 描述 |
| `--contact` | 空 | 联系方式（邮箱或 URL） |
| `--pubkey` | 空 | 运营者公钥 |
| `--require-auth` | `false` | 订阅前必须完成 NIP-42 认证 |
| `--compact` | `true` | 优雅关闭时压缩事件日志 |

### `keygen` —— 生成密钥

```bash
nostromo keygen
# private key (keep secret): 7a256d3d...
# public key (x-only)      : e45823fd...
```

### `validate` —— 校验事件

支持单个事件 JSON 或多行 JSONL，也可从标准输入读取：

```bash
nostromo validate event.json
cat events.jsonl | nostromo validate -
```

### `replay` —— 回放事件

把文件中的事件依次发布到目标 Relay，并打印每条的 `OK` 结果：

```bash
nostromo replay --to ws://127.0.0.1:7777 events.jsonl
```

### 典型使用场景

- 🧑‍💻 **本地开发 Nostr 客户端 / Agent**：用 `--db ""` 起一个零状态、可重置的本地 Relay。
- 🤖 **给 AI Agent 一个可观测的事件总线**：在调试台实时观察 Agent 发出的事件，用回放复现问题。
- 🏠 **个人 / 社区自托管**：用文件持久化 + Docker / systemd 长期运行。

### 客户端连接示例（JavaScript）

```javascript
const ws = new WebSocket("ws://localhost:7777/");
ws.onopen = () => {
  // 订阅所有 kind 1 文本事件
  ws.send(JSON.stringify(["REQ", "my-sub", { kinds: [1], limit: 100 }]));
};
ws.onmessage = (ev) => {
  const [type, ...rest] = JSON.parse(ev.data);
  console.log(type, rest); // EVENT / EOSE ...
};
```

> 📸 **演示截图 / 动图占位**：调试台截图可放在 `docs/assets/`，并在此处引用。

---

## 💡 设计思路与迭代规划

**设计理念：**

- 🧱 **事件即日志**：所有动作都是同一种「签名事件」，天然带来统一身份与可审计性。
- 🧅 **极简内核 + 可观测**：先保证内核小而正确，再把调试台、回放等开发者体验做扎实。
- 🔓 **零依赖即安全**：不引入第三方库，WebSocket 与曲线运算都在仓库内，便于审计与复现。

**技术选型理由：** 选择 Go，是因为其标准库强大、静态二进制天然跨平台、交叉编译只需设置 `GOOS / GOARCH`，非常契合「单文件、零运行时依赖」的目标。

**后续迭代计划（Roadmap）：**

- 📈 更丰富的存储后端与索引（在保持核心零依赖的前提下提供可选适配器）。
- 🧩 更多 NIP（如 NIP-13 工作量证明反垃圾、NIP-59 封装消息的工具支持）。
- 🧪 官方多语言 SDK 示例与负载测试工具。
- 📡 多 Relay 镜像 / 集群部署的参考方案。

**社区贡献方向：** 新 NIP 支持、测试用例、文档翻译、性能优化都非常欢迎，见 [贡献指南](CONTRIBUTING.md)。

---

## 📦 打包与部署指南

### 全平台打包

```bash
make release          # 或 ./scripts/build.sh 1.0.0
```

产物位于 `bin/release/`，包含：

```
nostromo_linux_amd64 / nostromo_linux_arm64
nostromo_darwin_amd64 / nostromo_darwin_arm64
nostromo_windows_amd64.exe / nostromo_windows_arm64.exe
checksums.txt
```

### 部署

- **Docker / docker compose**：见根目录 [`Dockerfile`](Dockerfile) 与 [`docker-compose.yml`](docker-compose.yml)。
- **systemd（Linux）**：服务单元见 [`deploy/nostromo.service`](deploy/nostromo.service)：

  ```bash
  sudo cp bin/nostromo /usr/local/bin/nostromo
  sudo cp deploy/nostromo.service /etc/systemd/system/
  sudo useradd -r -s /usr/sbin/nologin nostromo
  sudo mkdir -p /var/lib/nostromo && sudo chown nostromo:nostromo /var/lib/nostromo
  sudo systemctl enable --now nostromo
  ```

- **反向代理 / TLS**：Relay 信任 `X-Forwarded-Proto`，可置于 Caddy / Nginx 之后启用 `wss`。

架构细节见 [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)。

---

## 🤝 贡献指南

欢迎 Issue、PR 与文档翻译！提交信息遵循 Conventional Commits：

- `feat: 新增……` / `fix: 修复……` / `docs: 文档……` / `refactor: 重构……` / `test: 测试……`

提交前请运行质量门：

```bash
make check   # gofmt + go vet + go test
```

完整规范见 [**CONTRIBUTING.md**](CONTRIBUTING.md)。

---

## 📄 开源协议说明

本项目基于 **[MIT License](LICENSE)** 开源，可自由使用、修改与分发。Nostr 为开放协议，相关 NIP 标准版权归其各自作者所有。

<div align="center">

**Made with 🛰️ by [gitstq](https://github.com/gitstq)**

</div>
