# grok-build-vscode 变更记录

`services/grok-build-vscode`：VS Code 扩展 + Desktop。自己不扫 plugins/skills，只 spawn `atlas agent stdio`；斜杠来自 ACP `availableCommands`。上游独立 remote，不要往 `Eric9701/grok-build` 推。记法见 [README](./README.md)。

上游版本记在该目录自己的 `CHANGELOG.md`。这里只记 **Atlas overlay** 与合并动作。

## 2026-10

### 2026-10-28 — 聊天里选中文字可以复制

- **状态**：已落地
- **会话**：[聊天复制选中文字](5dee06aa-76a9-40a7-9fbd-1b2c8736ea28)
- 4.15.4 只在窗口重新获得焦点时避开输入框。VS Code 的 Ctrl+C 会先把焦点交回网页，选区经常已经被清掉，复制读到的是输入框里的光标，剪贴板是空的。
- `media/chat.js` 在选区还在时记住聊天里的文字，`copy` 时写入剪贴板。输入框里另有一段选中文字、或用户已经在输入框里打字时，不再拿聊天选区覆盖。
- 覆盖测试：`test/transcript-copy-focus.dom.test.ts`。

### 2026-10-28 — 合并上游 4.15.4，用户可见 Grok 文案改为 Atlas

- **状态**：已落地
- **会话**：[合并 vscode 4.15.4](5dee06aa-76a9-40a7-9fbd-1b2c8736ea28)
- 上游 `main` 从 4.15.3 合到 4.15.4：聊天里复制选中文字、Muse Allow all 覆盖子代理审批、自定义模型 401 不再要求登录、工具行保留长文件名末尾。
- 异常、提示和帮助里仍写给用户看的 Grok 改为 Atlas（含 README / 设置说明 / 自定义模型 401）。`GROK_*` 环境变量、协议 id、模型 id、`SuperGrok`、安装包文件名 `Grok-Build-Desktop-*` 未改。
- 保留 Atlas overlay：扩展名、`~/.atlas`、`atlas.*`、企业更新通道。
- `npm test`：339 files / 8337 passed。

### 2026-10-26 — 合并上游 4.15.3

- **状态**：已落地
- **会话**：[合并 vscode 4.15.3](5dee06aa-76a9-40a7-9fbd-1b2c8736ea28)
- 上游 `Eric9701/grok-build-vscode` `main`（与 `phuryn` 同一尖端）从 4.6.0 合到 4.15.3：Muse Code、实验性 Antigravity、上下文窗口、工作流卡片、Projects 只保留自选文件夹、Disconnect 不登出 CLI、Windows 缺 CLI 时不再卡启动。
- 保留 Atlas overlay：扩展名 Atlas for VS Code、`~/.atlas`、`atlas` CLI、`atlas.*` 设置与命令、本地 `[model.*]`、企业更新通道 `http://10.218.220.237:22255/atlas/cli`。AFK Pilot 只作为手机/浏览器伴侣。
- 测试对准 Atlas 文案与 `atlas.*` 存储键（`npm test`：338 files / 8307 passed）。

## 2026-09

### 2026-09-17 — 用户可见 Grok 文案改为 Atlas

- **状态**：已落地
- **会话**：[合并上游 vscode](f4655f99-7132-406e-a676-0a06ba292453)
- 4.6.0 合入后仍露出的界面文案改为 Atlas：欢迎页 tip、语音设置说明、Settings 配置路径 `~/.atlas/config.toml`、CLI 已是最新提示、命令面板说明。
- `brandUserFacingText` 同时改写 `grok login` / `~/.grok` 展示路径；安装脚本提示点 Atlas 图标。SuperGrok、协议 id、`GROK_*`、安装包文件名 `Grok-Build-Desktop-*` 未改。

### 2026-09-17 — 合并上游 4.6.0

- **状态**：已落地
- **会话**：[合并上游 vscode](f4655f99-7132-406e-a676-0a06ba292453)
- **提交**：`62d74bba`
- 上游 `phuryn/grok-build-vscode` `main` 从 4.1.0 合到 4.6.0：会话中途登录过期卡、OpenAI 语音后端、订阅用量 donut、Changes 视图 / Changed-files 卡、composer model chip + effort strip、Provider config files、Codex steer。
- 保留 Atlas overlay：品牌、`~/.atlas`、`atlas` CLI、`atlas.*` 设置、本地 `[model.*]`、企业更新通道 `10.218.220.237:22255/atlas`。
- 测试对准 Atlas 文案与 `atlas.*` 存储键（`npm test`：256 files / 6164 passed）。

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
