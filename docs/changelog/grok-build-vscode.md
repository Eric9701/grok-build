# grok-build-vscode 变更记录

`services/grok-build-vscode`：VS Code 扩展 + Desktop。自己不扫 plugins/skills，只 spawn `atlas agent stdio`；斜杠来自 ACP `availableCommands`。上游独立 remote，不要往 `Eric9701/grok-build` 推。记法见 [README](./README.md)。

上游版本记在该目录自己的 `CHANGELOG.md`。这里只记 **Atlas overlay** 与合并动作。

## 2026-09

### 2026-09-02 — 建立本变更记录

- **状态**：文档
- **会话**：[整理变更记录](54b536e1-6151-41a6-bccc-39bad21556c3)

### 2026-09-02 — 合并上游 4.1.0

- **状态**：已落地
- **会话**：[合并上游](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f)
- **提交**：`c2911ca4`
- 浏览器 rewind/Claude 连接/GitHub clone、会话 tab 抢占等上游能力合入，保留 Atlas 品牌与 `.atlas` 配置叠加。

## 2026-08

### 2026-08-26 — model 刷新后 picker 要跟着变

- **状态**：已落地
- **会话**：[刷新 model](0076d771-ca2e-4b30-9f24-533fea368fe4)
- 订阅 CLI 的 `x.ai/models/update`，刷新 Settings / 模型选择器。不自己打 `/models`。

### 2026-08-25 — 合并上游 3.17.0

- **状态**：已落地
- **提交**：`9ff6c8f2`

### 2026-08-24 → 08-26 — 文案 Grok → Atlas；短品牌一并改

- **状态**：已落地
- **会话**：[规则下发与 overlay](61031787-835f-4e05-ade6-0a8f5c68e17a)
- 欢迎页、提示、错误信息、短品牌「Grok / Grok Build」改为 Atlas。
- 配置读写走 `.atlas`（兼容遗留 `.grok`）。ACP `_meta.rules` 要求新配置写 `.atlas`，不要去搜 `~/.grok`。

### 2026-08-16 — 合并上游 3.10.0

- **状态**：已落地
- **提交**：`c6290be5`

### 2026-08-12 → 08-31 — Overlay 能力与回归

- **状态**：已落地
- **会话**：[能力与运维](73fbb1a7-df21-4ab0-9c9f-e5d058a2840b)
- 未登录引导：`atlas login --device-auth`（`shellPath` 为解析到的 `atlas.exe`）。
- Settings 可读写用户 `[model.*]`，**跳过 `managed`**；`x.ai/models/update` 刷新 picker。
- Desktop 更新地址与 CLI 同一企业通道（`cli_update_base_url` / `22255/atlas`），不是独立 8888。
- 欢迎页、MCP 引导、user-guide 路径去掉 Grok / `~/.grok`。
- VS Code **没有**独立 MCP 设置页：MCP 在 CLI/`config.toml`；工程级写项目 `.atlas/config.toml`。
- 扩展不扫 `~/.atlas/installed-plugins`。斜杠里有 `/ask-atlas`、`/datachain-diagnosis` 说明 CLI 用户级发现是通的。

### 2026-08-05 前后 — 第一版 Atlas 适配

- **状态**：已落地
- **会话**：[主线建设](7b869ee4-cf94-48f3-9801-068610925442)
- 在 grok-build-vscode 上叠 Atlas 后端：定位 `atlas` 二进制、Device Auth、Atlas 文案。
- 去掉/不接企业侧不需要的上游遥测上报。
- 出 VSIX（环境示例 `http://10.218.220.237:8888/atlas/atlas-vscode-3.1.0.vsix`）。
