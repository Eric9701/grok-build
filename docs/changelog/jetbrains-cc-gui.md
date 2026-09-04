# jetbrains-cc-gui 变更记录

`services/jetbrains-cc-gui`：独立 git（`origin` = Eric9701 fork，`upstream` = 原作者）。IDEA 插件，Claude / Codex / Grok 并列；Atlas 走 **Grok 族 Provider**（家目录 `~/.atlas`，二进制 `atlas`）。记法见 [README](./README.md)。

上游版本记在该目录 `CHANGELOG.md`。这里只记 **Atlas overlay** 与合并动作。

## 2026-09

### 2026-09-03 — 默认模式 PowerShell 不再误报「用户拒绝」

- **状态**：已落地
- **会话**：[Case001 权限误拒](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f)
- 默认 Ask 下超时 / 切会话 / IPC 失败回 ACP `cancelled`，只有点了 No 才 `reject-once`（不再出现假的 `User rejected the execution`）。
- `session/request_permission` 已批准后，`terminal/create` 不再弹第二次。
- Windows 宿主按 CLI 同样方式调 `powershell.exe -NoProfile -NonInteractive -Command`，不再用 Unix 的 `-l -c`；Windows 上不再 `detached` spawn，否则 PowerShell 标准输出是空的。

### 2026-09-02 — 建立本变更记录

- **状态**：文档
- **会话**：[整理变更记录](54b536e1-6151-41a6-bccc-39bad21556c3)

### 2026-09-02 — `/` 命令不要默认 Claude 目录

- **状态**：已落地
- **会话**：[IDEA 斜杠默认](acbbd656-b088-4270-9e84-fbd48ae62ca0)
- Provider 为 `grok` / `atlas` 时不再灌 Claude 内置斜杠（`/compact`、`/init`、`/claude-api`）。
- 斜杠等 ACP `availableCommands`（即 atlas-cli 发现结果）。`SlashCommandRegistry.isGrokFamily`。

### 2026-09-01 — 合并上游 v0.5.5

- **状态**：已落地
- **会话**：[合并上游](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f)
- 上游：隐藏 CLI Provider、设置页社区区、webview 事件队列、NVM 下 CLI 发现等。
- 保留 Atlas Home / ENC model id / Grok 族斜杠行为。

## 2026-08

### 2026-08-26 — 合并上游 v0.5.4

- **状态**：已落地
- **会话**：[合并上游](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f)
- 上游含 Grok 持久 ACP runtime（`GrokSDKBridge`）。叠加层继续解析 `~/.atlas`。

### 2026-08-21 → 08-23 — 增加 Atlas 后端

- **状态**：已落地
- **会话**：[IDEA 扩展](f8a5f924-225c-4836-8505-2a728e8f851a)
- `AtlasHome`：`GROK_HOME` > 已存在的 `~/.atlas` > 遗留 `~/.grok` > 默认 `~/.atlas`。
- `GrokLocalAuthResolver` 读 Atlas Home 的 `auth.json` / `config.toml`。托管 `ENC(...)` 的 model id **不得**当作 `session/set_model`。
- 设置页 / 403 文案：走企业登录时引导 `atlas login --device-auth`，不要 SuperGrok / `grok login`。
- 不把环境里的 `XAI_API_KEY` 塞进 OAuth 路径（会把 SuperGrok 打成 `xai.api_key` → 403）。
