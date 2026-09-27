# 项目预研简报 / Research Brief

- **执行日期**：2026-09-27
- **数据源**：GitHub Trending（daily / weekly）及 Python / Go / Rust / TypeScript 分语言榜
- **目标账号**：gitstq（已有 1688 个 owner 仓库纳入去重）

## 候选与排除

| 候选 | 方向 | 结论 |
| --- | --- | --- |
| paperclipai/paperclip、stablyai/orca、Tencent/WeKnora、HKUDS/CLI-Anything、TencentCloud/Octop | 大型 Agent 工作区/平台 | 体量重、依赖多，无法单仓完整落地，排除 |
| vectorize-io/hindsight、rohitg00/agentmemory、engram、beads、deja-vu | Agent 记忆 | 账号已有 45 个记忆类项目，相似度高，排除 |
| ahg-* / blueeagle-* 等（222 个） | API gateway/router | 账号已高度饱和，排除 |
| alibaba/open-code-review、各类 code review | 代码评审 | 账号已有 41 个，排除 |
| NVIDIA/Model-Optimizer | 模型优化库 | 依赖 ML 框架、重型，排除 |
| openbao/openbao | Secrets 管理 | 账号已有 65 个 vault/secret 项目，排除 |
| tensorflow / vscode / llvm / pytorch / next.js / quiche | 成熟巨型项目 | 单仓无法复刻，排除 |
| cloudflare security-audit-skill、addyosmani agent-skills 等 | Agent skill 包 | 账号已有 303 个 skill 类，排除 |
| **block/buzz** | 基于 **Nostr 签名事件日志** 的协作 relay | 本体 581MB 属巨型，但其底座方向账号覆盖为 **0**，作为灵感来源 |

## 去重结论

对账号全部 owner 仓库检索 `nostr / schnorr / secp256k1 / event-sourcing /
activitypub` 等关键词：**Nostr、事件溯源、ActivityPub 命中均为 0**；仅有
3 个含 "relay" 的仓库，均为 AI 大模型 API 中转站，与 Nostr 事件日志协议
无关（相似度 0%）。

## 锁定目标：Nostromo

参考 block/buzz「**用户自主掌控的签名事件日志 relay**」这一产品逻辑
（Nostr 为开放标准 NIP），独立自研一个**零依赖、单二进制、面向开发者与
AI Agent 的轻量 Nostr 事件日志 Relay**。

- **核心功能**：NIP-01 事件发布/订阅、NIP-16 可替换/临时事件、NIP-33
  参数化可替换、NIP-09 删除、NIP-40 过期、NIP-20 命令结果、NIP-42 认证、
  NIP-50 搜索；内置实时调试台；事件录制（JSONL）/回放。
- **技术亮点**：仅用 Go 标准库，手写 RFC 6455 WebSocket 与 secp256k1 /
  BIP-340 Schnorr；append-only 事件日志 + 启动重放；一条命令交叉编译
  Windows / macOS / Linux（amd64 + arm64）。
- **差异化**：Buzz 是 581MB 的完整协作工作区；Nostromo 聚焦"轻量、零依赖、
  可观测、可复现"的开发者/Agent relay，开箱即用、可嵌入、易测试。
