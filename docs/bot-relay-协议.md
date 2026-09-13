# Bot-Relay 交互协议

面向 **Grok Bot 客户端（`bot_client`）↔ Computer Hub / 服务** 的稳定线协议。  
权威实现与类型：`crates/common/xai-tool-protocol`（`bot_relay.rs`、`methods.rs`、fixtures）。

> **与 Atlas CLI 的区分**  
> Atlas CLI 的 `agent/relay`（`--grok-ws-url`）透传的是 **ACP JSON-RPC**，不是本协议。  
> Bot-relay 的连接角色是 `ConnectionKind::BotClient`（`"bot_client"`），走 Computer Hub 的 WebSocket + JSON-RPC 方法目录。

---

## 1. 参与方与职责

| 角色 | 说明 |
|------|------|
| **bot_client** | Grok 主端 / X-chat 等客户端；握手 `kind: "bot_client"` |
| **Computer Hub（service）** | 鉴权、账号链接、冷路径缓存、热路径转发、事件扇出 |
| **Box / in-box gateway** | 用户机器上的 Bot 运行时；`bot.command` 的 `name`/`args` 原样透传至此 |

Hub **不解析** gateway 命令体：`bot.command` 的 `name` 与 `args` 为 upstream-verbatim；`agentId` 仅作 Hub 路由元数据。

```text
  bot_client  ──JSON-RPC / bot.*──►  Computer Hub  ──passthrough──►  Box gateway
       ▲                                 │
       └──────── bot.event (notify) ─────┘
```

---

## 2. 传输与握手

- 传输：Computer Hub WebSocket（与 harness / tool_server 共用协议族）。
- 协议版本：`PROTOCOL_VERSION = "1.0.0"`（`handshake.rs`）；不兼容变更才升版本，能力用 `hello_ack.capabilities` 协商。

**Hello（客户端 → Hub）** 要点：

```json
{
  "protocol_version": "1.0.0",
  "kind": "bot_client"
}
```

**HelloAck（Hub → 客户端）** 要点：`connection_id`、`user_id`、`capabilities`。  
Bot-relay 面在 `capabilities` 中应出现的方法字符串见下节 `BOT_RELAY_CAPABILITIES`。

---

## 3. 方法目录（客户端稳定边界）

`BOT_RELAY_CAPABILITIES`（顺序与代码一致）：

| Method | 方向 | 冷/热 | 含义 |
|--------|------|-------|------|
| `bot.command` | 请求/响应 | **热**（可能唤醒 box） | 透传 in-box gateway 命令 |
| `bot.vncDescriptor` | 请求/响应 | **热**（可能唤醒） | 短时 noVNC 描述符 |
| `bot.roster` | 请求/响应 | **冷** | 缓存 agent 花名册，不唤醒 |
| `bot.status` | 请求/响应 | **冷** | Off-box 运行态，不唤醒 |
| `bot.transcript.offbox` | 请求/响应 | **冷** | Off-box transcript 分页，不唤醒 |
| `bot.subscribe` | 请求/响应 | — | 订阅指定 agents 的 `bot.event` |
| `bot.unsubscribe` | 请求/响应 | — | 取消订阅 |
| `bot.bindConversation` | 请求/响应 | — | 记录 conversation → agents 索引（**不**按 conversation 路由） |
| `bot.event` | **Hub → 客户端通知** | — | 事件信封；不是客户端可调 verb |

字段命名：**线协议 camelCase**（如 `agentId`）；Rust 类型为 snake_case + `serde(rename_all = "camelCase")`。

不存在的方法示例：`bot.ensureBox` **不在** Method 枚举与 capabilities 中。

---

## 4. 各方法载荷

### 4.1 `bot.command`

```json
{
  "agentId": "agt_1",
  "name": "sendPrompt",
  "args": { "prompt": "hello" }
}
```

- `name` / `args`：gateway 原样；本 crate **不**为它们建模。
- 结果：任意 JSON（`BotCommandResult = Value`）。
- 信封 `agentId` 与 `args.agentId`（若存在）不一致 → `command_rejected` / `agent_id_mismatch`。

Gateway 命令名（生成物 `BotRelayCommands`，编译默认集；运行时可放宽/收窄）包括但不限于：

`createAgent`、`listAgents`、`sendPrompt`、`updateAgent`、`deleteAgents`、  
`getAgentTranscript`、`getAgentTranscriptPage`、`getAgentTranscriptTail`、`getAgentTranscriptWindow`、  
`attachUpload`、`uploadAttachment`、`readAttachment*`、`interruptAgentRun`、`promptAcceptanceStatus`、  
`createGroup`、`setGroupMembers`、`connectChannel` / `disconnectChannel` / `refreshChannel`、  
以及 avatar / automation / widget / teach-recording 等相关命令。

完整列表与参数类型见：

- Kotlin：`crates/common/xai-tool-protocol/generated/kotlin/BotRelayCommands.kt`
- Swift：`generated/swift/BotRelayProtocol.swift`
- 源注释指向 upstream：`xai-grok-bot-upstream`（生成输入）

### 4.2 `bot.vncDescriptor`

**Params：** `{ "agentId": "..." }`  
**Result：**

```json
{
  "vncUrl": "https://example.invalid/vnc",
  "expiresHint": null
}
```

- `expiresHint`：Unix **毫秒**；`null` 表示遗留 network-token（至 pod 迁移前有效）；有值时应在到期前刷新。

### 4.3 `bot.roster`（冷）

**Params：** `{}`  
**Result：**

```json
{
  "agents": [
    {
      "agentId": "agt_1",
      "name": "Watcher",
      "status": "idle",
      "lastTurnAt": 1700000123000
    }
  ]
}
```

- `status`：`running` | `idle` | `unknown`。冷读 off-box 行恒为 `unknown`（耐久注册表无活动态）。
- `lastTurnAt`：可选；冷行可缺省。

### 4.4 `bot.status`（冷）

**Params：** `{}`  
**Result：** `{ "runState": "hibernated" }`

| `runState` | 含义 |
|------------|------|
| `absent` | 无 box / 未部署 |
| `hibernated` | 休眠 |
| `running` | 运行中 |
| `unknown` | 未知线值降级 |

发送端只发命名变体；接收端对未知字符串降级为 `unknown`。

### 4.5 `bot.transcript.offbox`（冷）

**Params：** `{ "agentId": "...", "cursor": "..."? }`  
**Result：** `{ "entries": <upstream page>, "nextCursor": "..."? }`  
`entries` 为 upstream 原样页。

### 4.6 `bot.subscribe` / `bot.unsubscribe`

```json
{
  "agentIds": ["agt_a", "agt_b"],
  "fullFidelity": false
}
```

- `fullFidelity`：可选；为 true 时订阅全保真 SSE（含 inline avatars）；默认 slim。
- 结果：`{}`。

### 4.7 `bot.bindConversation`

```json
{
  "conversationId": "conv_1",
  "agentIds": ["agt_a"],
  "primary": "agt_a"
}
```

仅写索引；**Hub 不会**根据 `conversationId` 推断 `bot.command` 目标。结果：`{}`。

---

## 5. `bot.event` 事件信封

信封版本：`v == 1`（`BOT_EVENT_ENVELOPE_V`）。

```json
{
  "v": 1,
  "agentId": "agt_1",
  "seq": 2,
  "channel": "hub:turn_finished",
  "event": { ... },
  "eventId": "optional-content-id"
}
```

### 5.1 `seq` 语义（接收端必须遵守）

- 按 **(connection, agent)** 单调，每次 `bot.subscribe` 后从 **1** 起。
- **排序参考，不是去重键**：重投递带新 `seq` 视为新事件；同 `seq` 两帧也是两个事件。
- **不可跨连接比较**。
- **不得**用 seq gap 推断 resync；resync 仅由显式 `hub:resync_required` 或重连触发。

### 5.2 `channel`

| 形态 | 示例 | `event` 体 |
|------|------|------------|
| Hub 已知 | `hub:turn_finished` | `{ agentId, conversationIds, preview }` |
| Hub 已知 | `hub:resync_required` | `{ agentId }` |
| Hub 未知未来 | `hub:...` | 当作 HubUnknown；保留字符串 |
| Upstream | 无 `hub:` 前缀，如 `transcript` | upstream 原样 |

`hub:turn_finished` 的 `preview` 在 schema 上是必填 string（可空串）。Hub 发射端常发 `""`（客户端应读 transcript tail）；非空在线也合法。

`eventId`：预留给内容身份去重；缺省时不出现在线上。

### 5.3 序列符合性（fixtures）

目录：`crates/common/xai-tool-protocol/fixtures/bot_relay/`。  
序列文件形如：

```json
{
  "must": "说明",
  "expectedResyncCount": 1,
  "expectedDistinctEvents": 2,
  "frames": [ /* BotEventEnvelope[] */ ]
}
```

- `expectedResyncCount`：仅统计显式 `hub:resync_required`。
- `expectedDistinctEvents`：送达事件数（按帧计，不按 seq 去重）。

可执行参考消费者：`tests/bot_relay_conformance.rs`（`ReferenceConsumer`）。

---

## 6. 错误模型

JSON-RPC `error`：

- `error.message`：snake_case 的 `BotRelayErrorCode` 字符串。
- `error.data`：Hub 错误对象：

```json
{
  "code": "command_rejected",
  "retryable": false,
  "detail": {},
  "reason": "not_yet_enabled"
}
```

- `detail` 始终存在（无内容时为 `{}`）；`detail.upstream` 仅调试，客户端勿解析。
- `reason`：`command_rejected` 与 link-state 类错误携带机器可读 token。
- 未知 `code` 字符串 → 接收端降级为 `upstream_error`，保留 `retryable` / `detail`。

### 6.1 `BotRelayErrorCode`（闭集）

| code | 典型 JSON-RPC 分类 | 说明 |
|------|-------------------|------|
| `identity_unavailable` | tool_unavailable | 身份不可用 |
| `link_required` | forbidden | 需链接 Cursor 账号 |
| `link_removed` | forbidden | 链接已解除，勿静默重试 |
| `consent_required` | forbidden | 需用户同意 |
| `enterprise_unsupported` | forbidden | 企业侧不支持；`reason` 指名规则 |
| `legacy_pricing_unsupported` | forbidden | 旧计费账号 |
| `email_unverified` | forbidden | xAI 邮箱未验证 |
| `link_conflict` | forbidden | 链接冲突；`reason` 区分 |
| `cursor_account_unavailable` | forbidden | 已链但 Cursor 账号不可用 |
| `link_unsupported` | forbidden | 更细拒绝 token 未知；`reason` 原样 |
| `no_plan` | forbidden | 无套餐 |
| `usage_exhausted` | rate_limited | 用量耗尽 |
| `box_migrating` / `box_recreating` / `box_unavailable` | tool_unavailable | Box 不可达类 |
| `computer_unavailable` | tool_unavailable | 计算机不可用 |
| `command_rejected` | invalid_request | 命令被拒；看 `reason` |
| `upstream_error` | internal_error | 上游/未知 |

Link-state 子集（`is_link_state`）：一律 **不可重试**，且带 `reason`。

### 6.2 `command_rejected` 的 `reason`（闭集）

| reason | 含义 |
|--------|------|
| `not_yet_enabled` | 已编译但未放行 |
| `agent_id_mismatch` | 信封与 args 的 agentId 不一致 |
| `args_too_large` | 如 uploadAttachment 参数 JSON > 3 MiB |
| `args_invalid` | 缺参/空参 |
| `attachments_not_supported_in_live` | Live 模式拒附件 |
| `not_supported_in_live` | Live 不支持 interrupt / acceptance 等 |
| `attachment_credential_unavailable` | 无可用凭证取附件 |
| `harness_refused` | harness 拒收；可再发同消息 |
| `attachment_not_found` | 附件不可见 |
| `attachment_wrong_source` | 非 BOT_CHAT 上传 |
| `attachment_too_large` | 对象 > 25 MiB |
| `attachment_not_ready` | 未 PostProcessDone |
| `gateway/unknown-method` | Box 拒收已成形 catalog 方法（能力偏斜） |

仅当 `code=command_rejected` 且 `reason=gateway/unknown-method` 时，`is_gateway_method_unsupported` 为 true（与 catalog-miss / `not_yet_enabled` 区分）。

---

## 7. 与 Hub 合成工具（模型侧）的关系

Computer Hub 还可向 **agent harness** 注册一组模型可见工具（`GROK_BOT_TOOL_IDS`），例如：

`bot_create_agent`、`bot_list_agents`、`bot_send_prompt`、`bot_get_agent_transcript*`、`bot_transcript_offbox`、`bot_await_turn`。

这是 **Hub → 模型 tool 面**，不是 bot_client 的 JSON-RPC verb。  
实现上多映射到 box gateway / 冷路径；工具 id 与描述见 `crates/common/xai-computer-hub-core/src/bot_tools.rs`。

Bot-relay 客户端应用 **第 3–5 节** 的 `bot.*` 方法；模型侧用 **本节** 的 `bot_*` 工具名——二者勿混为一谈。

---

## 8. 典型交互顺序

```text
1. WS upgrade + hello(kind=bot_client) → hello_ack(capabilities 含 bot.*)
2. bot.status / bot.roster          （冷：探活、列 agent，不唤醒）
3. bot.subscribe { agentIds }       （开始收 bot.event）
4. bot.command { name: sendPrompt, … }  （热：可能唤醒 box）
5. 收 bot.event：upstream transcript… → hub:turn_finished
6. 若收 hub:resync_required：按客户端策略重拉 transcript / 状态（勿靠 seq gap）
7. bot.unsubscribe / 断连
```

Cold-first：能 `bot.transcript.offbox` / `bot.roster` 解决的，不要先 `bot.command` 唤醒 box。

---

## 9. 产物与测试入口

| 产物 | 路径 |
|------|------|
| Rust 线类型 | `crates/common/xai-tool-protocol/src/bot_relay.rs` |
| Method 枚举 | `…/src/methods.rs` |
| 连接角色 | `…/src/connection.rs`（`BotClient`） |
| Wire fixtures | `…/fixtures/bot_relay/` |
| 符合性测试 | `…/tests/bot_relay_conformance.rs` |
| Kotlin / Swift 生成 | `…/generated/kotlin|swift/` |
| Hub 工具常量 | `crates/common/xai-computer-hub-core/src/bot_tools.rs` |

修改协议时：改 Rust 源类型与 fixtures，跑 conformance；多语言生成物按仓库 `just generate-gateway-types`（或等价）流程再生。

---

## 10. 文档维护

- **线协议行为以 Rust 类型 + fixtures + conformance 为准**；本文是人读摘要。
- 新增 `command_rejected` reason 或 `BotRelayErrorCode` 必须同步闭集常量与 metrics 标签，否则 codegen/测试会失败。
- 新增 `hub:*` channel：在 `HubChannel` 落地前，客户端须能当 `HubUnknown` 解码并忽略/降级。
