# atlas-cli 变更记录

可执行文件 `atlas` / `atlas.exe`（crate 仍是 `xai-grok-pager-bin`）。家目录 **Atlas Home**（`~/.atlas`）。记法见 [README](./README.md)。

## 2026-09

### 2026-09-18 — CLI 提示/帮助/升级文案 Grok 改为 Atlas

- **状态**：已落地
- **会话**：[CLI 展示文案](f4655f99-7132-406e-a676-0a06ba292453)
- 用户可见异常、帮助、提示、升级信息改为 Atlas / `atlas`（含 `atlas update`、`atlas login`、doctor、inspect、MCP、插件）。家目录展示 `~/.atlas`。
- 未改 `GROK_*`、`--grok-ws-url`、模型 id、`grok.com`、SuperGrok、crate / 配置键名。

### 2026-09-18 — 内置用户指南 Grok 文案改为 Atlas

- **状态**：已落地
- **会话**：[Pager 文档品牌](f4655f99-7132-406e-a676-0a06ba292453)
- `/docs` 抽出的 user-guide / tutorial：家目录与系统目录改为 `~/.atlas`、`.atlas/`、`/etc/atlas`；产品名与 CLI 示例改为 Atlas / `atlas`；章节 `27-atlas-clone.md`。
- 未改 `GROK_*` 环境变量、`grok_com_config` 配置键、模型 id、`grok.com` / crate 路径。

### 2026-09-15 — 合入 origin/main

- **状态**：已落地
- **会话**：[代码新增行](f4655f99-7132-406e-a676-0a06ba292453)
- 合入上游 `Synced from monorepo`。overlay 保留 `~/.atlas`、`atlas`、企业安装名、托管 `managed`、企业 `GROK_CLI_BASE_URL`；接上游 `reasoning_summary`、skill 正文 cap、`LeaderDoorCli`。

### 2026-09-15 — Task Report 复用 edit.lines

- **状态**：已落地
- **会话**：[代码新增行](f4655f99-7132-406e-a676-0a06ba292453)
- `write` / `edit` / `apply_patch` 把已有的 `edit.lines` Insert 计数写进工具输出；Task Report 只读该字段，不再二次 Myers。无计数时仍按 hunk/全文差分兜底。

### 2026-09-15 — Task Report 统计代码新增行

- **状态**：已落地
- **会话**：[代码新增行](f4655f99-7132-406e-a676-0a06ba292453)
- 成功的 `write` / `edit` / `apply_patch` 上报每路径 Insert 行（`artifactLinesAdded`）。同文件多次编辑累加。bash 改文件仍不计。
- 与 [atlas-server](./atlas-server.md) 的 `codeLinesAdded` 配套。

### 2026-09-13 — 会话记忆回填

- **状态**：文档
- **会话**：[会话记忆](57e9ab26-cafd-4fb7-bd67-a823e1b5cd61)
- 把 09-03 起未入库会话折进 `docs/project-memory.md`：429 归类、两条 relay、Leader / agent-host、合入 overlay 要点。

### 2026-09-11 — Leader 与 agent-host

- **状态**：分析未改
- **会话**：[合并上游](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f)
- agent-host 是上游进程拆分（UI 与 Agent 不在同一进程），不是企业产品名。日常仍用 TUI / stdio / headless。
- Leader 可走 ACP `--grok-ws-url`，不能走 bot-relay。给 relay-demo 当盒子时不要加 `--relay-on-demand`。

### 2026-09-05 — Bot-relay 不是 CLI relay

- **状态**：文档
- **会话**：[Bot 协议](6ccf75f6-0232-4bd8-b8a1-05191470efbb)
- 整理 [docs/bot-relay-协议.md](../bot-relay-协议.md)。ACP-over-WS 可独立的是 Hub + stdio↔WS，不是 `headless` + grok.com 会话。

### 2026-09-03 — 429 与 overloaded 不是同一条路

- **状态**：分析未改
- **会话**：[429 策略](1abaf4ee-4818-4837-a401-a134ebc6c693)
- HTTP 429 当限流：采样器最多再打 1 次；主会话不排队；子 agent 可等最多 8 次 / 150s。正文写 overloaded 也不会改走 529 过载重试。

### 2026-09-10 — 合并上游 origin/main

- **状态**：已落地
- **会话**：[合并上游](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f)
- 合入上游 Synced from monorepo：auth 抽到 `xai-grok-login`、folder trust、TUI 等。
- 保留 Atlas overlay：家目录 `~/.atlas`、`atlas` 品牌、企业 Device Auth 默认、Startup Session Gate、托管模型 ENC（`ring`）、Task Report 默认开。

### 2026-09-02 — 建立本变更记录

- **状态**：文档
- **会话**：[整理变更记录](54b536e1-6151-41a6-bccc-39bad21556c3)

### 2026-09-01 — 出站中继不改 CLI

- **状态**：分析未改
- **会话**：[工作台与中继](8fe4e3ad-2535-415c-8d7b-68a472203fb6)
- CLI 没有独立「工作台」产品面；多会话靠 TUI/`/resume`，多任务靠子 agent。
- 浏览器同时管多 Agent 走 [atlas-relay-demo](../../services/atlas-relay-demo/CONTEXT.md)，继续用现有 `atlas agent headless --grok-ws-url`，身份靠 URL `agent_id`。

## 2026-08

### 2026-08-31 — 创建 skill 仍写 `.grok`

- **状态**：已落地
- **会话**：[创建 skill 路径](954e513d-9243-4b31-9bd9-95f584df3127)
- 新建 Native Skill 的默认路径从 `.grok/skills` 改到 Atlas Home / `.atlas`。
- 上游 bundle 文案里残留的 `.grok` 另在 atlas-server probe 侧替换。

### 2026-08-31 — 子 agent Task Report 缺产出物

- **状态**：已落地
- **会话**：[子 agent 产出物](674eaaa2-f6bb-4a51-a8be-9c5531608150)
- 主会话会带 artifacts；子 agent 结束上报时漏了产出物字段。补上报，与主会话同一报文体。

### 2026-08-26 — `atlas models refresh` / `/refresh-model`

- **状态**：已落地
- **会话**：[刷新 model](0076d771-ca2e-4b30-9f24-533fea368fe4)
- 立刻作废 `models_cache.json`，`GET /atlas/v1/models`，把托管段落到 `config.toml`，并广播 `x.ai/models/update`。
- 会话内别名 `/refresh-models`。`/model` 选择器仍只读内存 catalog，不因此打网。
- VS Code picker 靠同一广播刷新，见 [grok-build-vscode](./grok-build-vscode.md)。

### 2026-08-26 — 合并上游后 tool call id 重复

- **状态**：已落地
- **会话**：[规则下发与 overlay](61031787-835f-4e05-ade6-0a8f5c68e17a)
- Kimi / GLM：`tool call id :1 is duplicated`。ACP 子 agent 复用同一 tool call id 时上游 400。
- 会话内保证 tool call id 唯一（与 08-20 的 uniqueness 工作衔接）。
- GLM5 另报 `messages.content.type` 只接受 `text`：按该后端收窄内容类型，不把图片/其它 type 原样前传。

### 2026-08-24 — 通用规则与 CLI 一起下发

- **状态**：分析未改
- **会话**：[规则下发与 overlay](61031787-835f-4e05-ade6-0a8f5c68e17a)
- 方案：企业通用规则跟安装包 / 插件走，不另开运行时通道。未改发现优先级。

### 2026-08-24 — OAuth 一天后 Task Report 变 anonymous

- **状态**：已落地
- **会话**：[Token 续期](2f91e8e7-8182-4f37-a5d8-d69e2686c5a2)
- access 默认 1h；refresh 失败后 CLI 清 `auth.json`，之后上报用户变 `anonymous`。
- 服务端 refresh 落 MySQL（见 [atlas-server](./atlas-server.md)）。重启 atlas-server 不得丢掉已登录 CLI 的静默续期。

### 2026-08-24 — 多 MCP + npx 内存暴涨

- **状态**：分析未改
- **会话**：[MCP 资源](699a63c3-636c-49d9-91fb-8e19ea8e323d)
- stdio MCP 启动即拉起；`command = "npx"` 每个 server 一个 Node。工程级 MCP 写项目配置，不要写进用户全局。运行时不共享进程。

### 2026-08-23 — Ubuntu 编的二进制在 CentOS 7 跑不了

- **状态**：已落地
- **会话**：[老 glibc 构建](396a5ad5-3ab6-4b8b-a056-a8e75e086652)
- 官方 x86_64 Linux 包改 musl。`sqlite-vec` 缺 `u_int8_t`：用仓库 `.cargo/config.toml` 的 `CFLAGS_*`。
- vendor 必须完整（缺 `vendor/nucleo` 编不过）。手册见 `docs/atlas-编译手册.md`。

### 2026-08-20 — tool call id 与会话历史

- **状态**：已落地
- **提交**：`b7b56b2e`
- 对话历史与 tool call 唯一性，避免 ACP/上游 400 duplicated。

### 2026-08-17 前后 — 登录门控、ENC 缓存、Catalog 上报

- **状态**：已落地
- **会话**：[能力与运维](73fbb1a7-df21-4ab0-9c9f-e5d058a2840b)
- **Startup Session Gate** 默认开：登出或凭证被清后，下一次新进程才再要登录。`require_session_at_startup = false` 可关。
- Task Report：用户与客户端版本只读报文体；缺用户记 `anonymous`。主会话、plan 模式、子 agent 都报。
- `models_cache.json`：Managed Catalog Mode 时 `api_key` 与 `info.model` 保持 At-Rest ENC；catalog id 明文。非 ENC 整文件作废。
- 主会话与子 agent 都报 **Catalog ID**；Routing Name 明文另报 `modelRouting`，解析不到就省略。
- 升级后 `bin` 里仍是 `grok.exe` 算缺陷，须为 `atlas.exe`。
- 终端 tab、user-guide 路径用 Atlas Home，不要搜 `~/.grok/docs/user-guide`。
- 去掉 TUI「Click here to upgrade」。
- 安装脚本从 `22255/atlas` 拉包，尽量同时装 `atlas-sdd`。

### 2026-08-15 — 安装脚本与托管模型客户端

- **状态**：已落地
- **提交**：`e5ed9384`
- 企业安装脚本、托管模型解密（`util/model_secret.rs`，`ring` 必须在 `[dependencies]`）、`config.toml` 托管段同步。
- 用户手写、无 `managed` 的 `[model.*]` 不被覆盖。取消分配后删托管段。
- 合并优先级：本地 `[model.*]` > prefetch > builtin。

### 2026-08-06 — 品牌与配置目录

- **状态**：已落地
- **会话**：[主线建设](7b869ee4-cf94-48f3-9801-068610925442)
- 用户可见文案、TUI、家目录 `.grok` → `.atlas`。crate / 环境变量名保持 grok-*。
- 默认 OAuth issuer 走企业 atlas-server，不是 `auth.x.ai`。
- 企业登录：`atlas login --device-auth`。

## 2026-07

### 2026-07-22 → 07-29 — 企业化起点

- **状态**：已落地
- **会话**：[企业化起点](ba632f70-aedc-4426-b76e-b57346e56fc7)
- Device Auth 打自建后端；用户名 + 机器码（须 Admin 先开户）。
- Task Report 客户端上报初版；plan / 指定 agent / 主会话路径逐步补齐。
- 兼容扫描 `~/.claude/skills`；插件 skill **不会**出现在 `~/.atlas/skills`。
