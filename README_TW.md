<div align="center">

# 🛰️ Nostromo

### 零相依、單一執行檔的 Nostr 事件日誌 Relay —— 為開發者與 AI Agent 而生

**語言：** [简体中文](README.md) · [繁體中文](README_TW.md) · [English](README_EN.md)

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report](https://img.shields.io/badge/go%20report-standard%20library-00ADD8?logo=go&logoColor=white)](#-技術棧)
[![Release](https://img.shields.io/badge/release-v1.0.0-blue.svg)](#-下載)

</div>

> **⬇️ 下載：** 預編譯執行檔請前往 [**Releases 頁面**](https://github.com/gitstq/nostromo-relay/releases)，支援 Windows / macOS / Linux（amd64 + arm64）。

---

## 🎉 專案介紹

**Nostromo** 是一個**開箱即用的輕量級 [Nostr](https://nostr.com/) 中繼器（Relay）**。它把每一則訊息都建模為一條**帶有密碼學簽章、只能追加的事件**，不論作者是人還是 AI Agent，都擁有統一的資料結構、身分模型與稽核軌跡。

它解決了以下痛點：

- 🧰 **現有 Relay 太笨重**：主流實作相依資料庫與繁瑣設定，本機除錯 Nostr 用戶端 / Agent 時啟動成本高。
- 👀 **過程無法觀測**：開發者需要即時看到事件串、依條件過濾、快速驗證，而不是面對黑箱。
- 🔁 **測試難以重現**：缺乏方便的事件錄製與回放機制。
- 🔐 **身分與權限耦合**：希望像對待團隊成員一樣，用獨立金鑰來界定 Agent，而不是一堆權限開關。

**自研差異化亮點：**

- 🪶 **真正零相依**：只使用 Go 標準函式庫，手寫 **RFC 6455 WebSocket** 與 **secp256k1 / BIP-340 Schnorr**，整個程式約為一個 5MB 的靜態執行檔。
- 🧪 **為開發者與 Agent 設計**：內建**即時除錯台**、**事件錄製 / 回放**、明確的指令結果，可嵌入、可斷言。
- ⚡ **一條指令完成全平台建置**：原生交叉編譯至 Windows / macOS / Linux（amd64 + arm64），無執行時期相依。

💡 **靈感來源**：本專案參考了 GitHub Trending 上 `block/buzz` 所提倡的「**使用者自主掌握的簽章事件日誌 Relay**」此一產品理念（Nostr 為開放標準 NIP）。與 Buzz 那個 581MB 的完整協作工作區不同，Nostromo 專注於**輕量、零相依、可觀測、可重現**的 Relay 核心，所有程式碼皆獨立自研。詳見[預研簡報](docs/RESEARCH_BRIEF.md)。

---

## ✨ 核心特性

- 📨 **完整的 NIP-01 協定**：事件釋出（`EVENT`）、訂閱（`REQ`）、關閉（`CLOSE`），支援任意 `#e / #p / #a / #t …` 標籤查詢（**NIP-12**）。
- 🔁 **可取代與暫時性事件（NIP-16）**：自動處理詮釋資料等可取代事件與暫時性事件，暫時性事件**即時廣播但不寫入磁碟**。
- 🏷️ **參數化可取代事件（NIP-33）**：依 `d` 標籤維護同一作者、同一類型的多個獨立狀態。
- 🗑️ **事件刪除（NIP-09）**：處理刪除事件，只移除同一作者所引用的事件。
- ⏳ **逾期時間（NIP-40）**：逾期事件自動從查詢結果中消失。
- ✅ **指令結果（NIP-20）**：每個事件都回傳明確的 `OK / 失敗原因`。
- 🪪 **用戶端驗證（NIP-42）**：基於隨機挑戰的 Schnorr 驗證，可設定為訂閱的前置條件。
- 🔍 **全文搜尋（NIP-50）**：多關鍵詞「且（AND）」比對，大小寫不敏感。
- 🖥️ **內建即時除錯台**：瀏覽器開啟即可連線、過濾、觀測事件串，並可傳送任意原始訊息。
- 💾 **事件日誌持久化**：追加式 JSONL，啟動時**回放重建**，關閉時可**壓縮（compaction）**。
- 🧰 **配套 CLI**：`serve / keygen / validate / replay` 四個子指令，涵蓋執行、金鑰、校驗與回放。
- 🛡️ **安全的預設值**：訊息大小限制、寫影格互斥、無第三方相依，攻擊面小。

---

## 🚀 快速開始

### 環境需求

- 執行預編譯版本：**完全不需要執行時期環境**（靜態執行檔）。
- 從原始碼建置：**Go 1.23+**（僅用標準函式庫，`go mod tidy` 後無外部相依）。

### 方式一：直接下載（建議）

前往 [**Releases**](https://github.com/gitstq/nostromo-relay/releases)，下載對應平台的檔案，解壓縮後直接執行：

```bash
# 記憶體模式啟動（資料不持久化），預設監聽 :7777
./nostromo serve --db ""
```

### 方式二：從原始碼建置

```bash
git clone https://github.com/gitstq/nostromo-relay.git
cd nostromo-relay
make build           # 產物在 bin/nostromo
./bin/nostromo serve
```

### 方式三：Docker

```bash
docker run --rm -p 7777:7777 -v nostromo-data:/data nostromo:1.0.0
# 或使用 docker compose
docker compose up
```

啟動後：

| 入口 | 位址 |
| --- | --- |
| 🖥️ 除錯台 | <http://localhost:7777/> |
| 🔌 WebSocket | `ws://localhost:7777/` |
| 📋 Relay 資訊（NIP-11） | `curl -H "Accept: application/nostr+json" http://localhost:7777/` |
| ❤️ 健康檢查 | <http://localhost:7777/healthz> |
| 📊 指標 | <http://localhost:7777/metrics> |

### 30 秒體驗完整流程

```bash
# 1) 產生一組金鑰
./nostromo keygen

# 2) 產生一批已簽章的範例事件
go run ./examples/seed -out seed.jsonl

# 3) 回放至正在執行的 Relay
./nostromo replay --to ws://127.0.0.1:7777 seed.jsonl

# 4) 校驗事件檔案
./nostromo validate seed.jsonl
```

---

## 📖 詳細使用指南

### `serve` —— 啟動 Relay

```bash
nostromo serve [flags]
```

| 參數 | 預設值 | 說明 |
| --- | --- | --- |
| `--addr` | `:7777` | 監聽位址 |
| `--db` | `data/events.jsonl` | 事件日誌檔案；傳 `""` 為純記憶體模式 |
| `--name` | `Nostromo` | Relay 名稱 |
| `--desc` | 內建描述 | Relay 描述 |
| `--contact` | 空 | 聯絡方式（Email 或 URL） |
| `--pubkey` | 空 | 營運者公鑰 |
| `--require-auth` | `false` | 訂閱前必須完成 NIP-42 驗證 |
| `--compact` | `true` | 優雅關閉時壓縮事件日誌 |

### `keygen` —— 產生金鑰

```bash
nostromo keygen
# private key (keep secret): 7a256d3d...
# public key (x-only)      : e45823fd...
```

### `validate` —— 校驗事件

支援單一事件 JSON 或多行 JSONL，亦可從標準輸入讀取：

```bash
nostromo validate event.json
cat events.jsonl | nostromo validate -
```

### `replay` —— 回放事件

把檔案中的事件依序發布至目標 Relay，並印出每則的 `OK` 結果：

```bash
nostromo replay --to ws://127.0.0.1:7777 events.jsonl
```

### 典型使用情境

- 🧑‍💻 **本機開發 Nostr 用戶端 / Agent**：用 `--db ""` 啟動一個零狀態、可重置的本機 Relay。
- 🤖 **給 AI Agent 一條可觀測的事件匯流排**：在除錯台即時觀察 Agent 發出的事件，用回放重現問題。
- 🏠 **個人 / 社群自架**：以檔案持久化搭配 Docker / systemd 長期執行。

### 用戶端連線範例（JavaScript）

```javascript
const ws = new WebSocket("ws://localhost:7777/");
ws.onopen = () => {
  // 訂閱所有 kind 1 文字事件
  ws.send(JSON.stringify(["REQ", "my-sub", { kinds: [1], limit: 100 }]));
};
ws.onmessage = (ev) => {
  const [type, ...rest] = JSON.parse(ev.data);
  console.log(type, rest); // EVENT / EOSE ...
};
```

> 📸 **展示截圖 / 動畫占位**：除錯台截圖可置於 `docs/assets/`，並於此引用。

---

## 💡 設計思路與迭代規劃

**設計理念：**

- 🧱 **事件即日誌**：所有動作都是同一種「簽章事件」，自然帶來統一身分與可稽核性。
- 🧅 **極簡核心 + 可觀測**：先確保核心小而正確，再把除錯台、回放等開發者體驗做扎實。
- 🔓 **零相依即是安全**：不引入第三方函式庫，WebSocket 與曲線運算都在儲存庫內，便於稽核與重現。

**技術選型理由：** 選擇 Go，是因為其標準函式庫強大、靜態執行檔天生跨平台、交叉編譯只需設定 `GOOS / GOARCH`，非常契合「單一檔案、零執行時期相依」的目標。

**後續迭代計畫（Roadmap）：**

- 📈 更豐富的儲存後端與索引（在維持核心零相依的前提下提供選用配接器）。
- 🧩 更多 NIP（如 NIP-13 工作量證明反垃圾、NIP-59 封裝訊息的工具支援）。
- 🧪 官方多語言 SDK 範例與負載測試工具。
- 📡 多 Relay 鏡像 / 叢集部署的參考方案。

**社群貢獻方向：** 新 NIP 支援、測試案例、文件翻譯、效能優化都相當歡迎，詳見[貢獻指南](CONTRIBUTING.md)。

---

## 📦 打包與部署指南

### 全平台打包

```bash
make release          # 或 ./scripts/build.sh 1.0.0
```

產物位於 `bin/release/`，包含：

```
nostromo_linux_amd64 / nostromo_linux_arm64
nostromo_darwin_amd64 / nostromo_darwin_arm64
nostromo_windows_amd64.exe / nostromo_windows_arm64.exe
checksums.txt
```

### 部署

- **Docker / docker compose**：見根目錄 [`Dockerfile`](Dockerfile) 與 [`docker-compose.yml`](docker-compose.yml)。
- **systemd（Linux）**：服務單元見 [`deploy/nostromo.service`](deploy/nostromo.service)：

  ```bash
  sudo cp bin/nostromo /usr/local/bin/nostromo
  sudo cp deploy/nostromo.service /etc/systemd/system/
  sudo useradd -r -s /usr/sbin/nologin nostromo
  sudo mkdir -p /var/lib/nostromo && sudo chown nostromo:nostromo /var/lib/nostromo
  sudo systemctl enable --now nostromo
  ```

- **反向代理 / TLS**：Relay 信任 `X-Forwarded-Proto`，可置於 Caddy / Nginx 之後啟用 `wss`。

架構細節詳見 [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)。

---

## 🤝 貢獻指南

歡迎 Issue、PR 與文件翻譯！提交訊息遵循 Conventional Commits：

- `feat: 新增……` / `fix: 修復……` / `docs: 文件……` / `refactor: 重構……` / `test: 測試……`

提交前請執行品質守門：

```bash
make check   # gofmt + go vet + go test
```

完整規範詳見 [**CONTRIBUTING.md**](CONTRIBUTING.md)。

---

## 📄 開源授權說明

本專案以 **[MIT License](LICENSE)** 開源，可自由使用、修改與散布。Nostr 為開放協定，相關 NIP 標準版權歸其各自作者所有。

<div align="center">

**Made with 🛰️ by [gitstq](https://github.com/gitstq)**

</div>
