# 项目记忆

2026-07-22 至 2026-09-13 本工程 Cursor 会话沉淀。**术语**以 CONTEXT 为准；**实现**以代码与手册为准。本文只留反复踩过的约定和易错点。**五件套改了什么**写 [docs/changelog](./changelog/README.md)。

| 要做什么 | 去哪 |
|---|---|
| 研发流程名词 | [CONTEXT.md](../CONTEXT.md) |
| 家目录 / 技能发现 / 登录门控 | [CONTEXT-runtime.md](./CONTEXT-runtime.md) |
| 账号、托管模型、Task Report 名词 | [services/atlas-server/CONTEXT.md](../services/atlas-server/CONTEXT.md) |
| 遥测开关、构建口令 | [.cursor/rules/atlas-telemetry-ops.mdc](../.cursor/rules/atlas-telemetry-ops.mdc) |
| ENC / ListModels / 落盘 | [.cursor/rules/atlas-managed-models.mdc](../.cursor/rules/atlas-managed-models.mdc) |
| 五件套变更记录 | [changelog/README.md](./changelog/README.md) |
| 安装与基础命令 | [atlas-runtime-安装手册.md](./atlas-runtime-安装手册.md)（§9：断网启动） |
| 中文用户指南 HTML | [atlas-用户指南.html](./atlas-用户指南.html) |
| 跨平台编译 | [atlas-编译手册.md](./atlas-编译手册.md) |
| Settings 字段 | [services/atlas-server/docs/settings.md](../services/atlas-server/docs/settings.md) |
| 决策记录 | [docs/adr](./adr/)、[atlas-server/docs/adr](../services/atlas-server/docs/adr/) |
| Bot-relay（`bot.*`） | [bot-relay-协议.md](./bot-relay-协议.md)（不是 CLI `--grok-ws-url`） |
| 出站 ACP 中继 | [atlas-relay-demo/CONTEXT.md](../services/atlas-relay-demo/CONTEXT.md)、[云端派工](../services/atlas-relay-demo/docs/云端派工推荐做法.md) |

企业代理基址（本环境）：`http://10.218.220.237:22255/atlas`。插件仓：`https://gitlab.imyai.cn/zhangyufeng/atlas-plugins.git`。VSIX 示例：`http://10.218.220.237:8888/atlas/atlas-vscode-3.1.0.vsix`。

---

## 产品形状

- 上游是 grok-build。企业侧对外品牌 **Atlas**，家目录 **Atlas Home**（`~/.atlas`），可执行文件 **`atlas` / `atlas.exe`**。升级后 `bin` 里仍是 `grok.exe` 算缺陷。
- 三件套：CLI（TUI/ACP）+ VS Code / Desktop + **atlas-server**（Go，登录、模型、遥测、更新）。
- 研发插件拆两包：**`atlas-skills`**（Main Flow，入口 `/ask-atlas`，曾用名 ask-matt）与 **`atlas-sdd`**（Role Executor）。Light Coupling，见 ADR 0001–0004。
- VS Code / Desktop **自己不扫** plugins/skills，只 spawn `atlas agent stdio`；斜杠来自 ACP `availableCommands`。

---

## 登录与身份

- 企业登录：`atlas login --device-auth`。默认 OAuth issuer 走企业 atlas-server，不是 `auth.x.ai`。
- 拉 xAI probe（`probe_xai_proxy.py`）必须用 **官方** Device Login：`python services/atlas-server/scripts/xai_login.py`，凭证写 `services/atlas-server/scripts/auth.json`。不要拿 `~/.atlas/auth.json` 打 `cli-chat-proxy.grok.com`。
- **Startup Session Gate** 默认开：登出或凭证被清后，**下一次新进程**才要求再登录。
- 机器码登录：用户须先在 Admin 开户。未开户不能只靠机器码。
- Admin 建用户：用户名 + 邮箱 + 机器码即可；**UserId = 邮箱 `@` 前前缀**；默认密码 `atlas123`。
- Task Report 的 **Report User / Client Version 只读报文体**，不解析 JWT。缺用户记 `anonymous`（ADR 0002）。
- `auth.json` 可以不在 Atlas Home：托管模型 + 本地 `[model.*]` 仍可能对话；那不表示已走企业登录。
- OAuth **refresh_tokens 落 MySQL**（device code 仍内存）。access 默认 1h、refresh 默认 30d。重启 atlas-server 不应清 CLI 会话；若 refresh 被当成 `invalid_grant`，CLI 会清 `auth.json`，之后 Task Report 变 `anonymous`。

---

## 模型与密钥

- **Catalog ID**（`[model.<id>]` / Task Report `model`）≠ **Routing Name**（`model.model` / Task Report `modelRouting`）。主会话与子 agent 都报 Catalog ID；Routing Name 明文另报；解析不到就省略，不编造。排行按 Catalog ID。
- 托管条目：ListModels 下发 `id`/`model`/`api_key` 均为 `ENC(...)`。客户端落盘：Catalog ID 明文，Routing Name 与 `api_key` 保持 At-Rest ENC。`models_cache.json` 同样；非 ENC 整文件作废。
- 用户手写、无 `managed` 的 `[model.*]` **不被同步覆盖**。取消分配后删除托管段。
- 合并优先级：本地 `[model.*]`（含托管落盘）> prefetch > builtin。
- `/model` 选择器通常只读内存 catalog，**不**因此打 `/models`。会打网的时机见遥测规则。
- 立刻拉网：终端 `atlas models refresh`，会话内 `/refresh-model`（别名 `/refresh-models`）。二者作废 `models_cache.json`、GET `/atlas/v1/models`、同步托管段到 `config.toml`，并广播 `x.ai/models/update`。
- `model_family` 管上下文压缩族；**不配不等于关闭压缩**，只是走默认族逻辑。
- 改用户模型分配后，客户端不会自动推送；跑 `atlas models refresh` / `/refresh-model`，或等下次预取 / 鉴权变化 / ETag / 缓存过期。
- GLM-5 的 Chat Completions **只接受** `content.type = text`。附图 / 读图的 `image_url` 会 400；发请求前对文本模型剥图，走现有 strip-retry。
- Kimi / GLM 流式 tool call 空 id 或每轮复用 `:1` → `400 tool call id :1 is duplicated`。空 id 补 `call_anonN`，跨 turn 重复改写并同步 tool result。

---

## 模型 API 异常

- **只认状态码**：正文写 *overloaded* 的 **429** 仍走限流，不走过载。过载只认 529、带 overloaded 的 5xx、或流式 `overloaded_error`。
- 采样器默认 `max_retries = 15`（`GROK_MAX_RETRIES` / `[model.*] max_retries`），但 **429 阈值 = 2**：再打 1 次就 Fatal。有 `Retry-After` 按秒等（封顶 120s）；没有则指数退避约 2s 起、封顶 30s。
- 立刻放弃：`x-should-retry: false`；上下文/token 溢出；429 正文像「Request too large」且没有 `Retry-After`。
- **主会话**对 429 不再排队（ACP `-32003 Rate limited`）。**子 agent** 可按预算等：默认最多 8 次 / 累计 150s（`GROK_SUBAGENT_RATE_LIMIT_MAX_ATTEMPTS`）。`turn_transient_retry` 只管 5xx/超时，**不管 429**。
- VS Code 见 `-32003` 当用量限制，不开登录页。401 才走刷新/登录。

---

## 遥测

- `trace_upload_enabled=false` **只关** trace 产物上传，不管 signals / events / Task Report。
- Task Report **默认开**，仅 `GROK_DISABLE_TASK_REPORT=1` 可关。主会话、plan 模式、子 agent 都报。
- 尽量全关：`telemetry_enabled=false` + `trace_upload_enabled=false` + `GROK_DISABLE_TASK_REPORT=1`。
- Remote settings：`download/probe_settings.json` → `GET /atlas/v1/settings`。
- Admin：`/atlas/admin/task-reports`；明细可弹详情；按人排行可按 token / 任务数排。只改 `web` 即可动该页。
- Agent 展示去掉上游 `grok-` 前缀：`grok-build-plan` 显示为 `build-plan`；库内仍存原文。鼠标悬停可看原值。
- 主会话与子 agent 都会报 artifacts；口径是成功的 `write` / `edit` / `apply_patch`。纯 bash 改文件两边都不记。
- Task Report **不是**派工验收回执。回执是仓库里的 `documents/execution-report-<jobId>.json`（或 ACP 对话里抄回的 JSON）。CLI **没有**上传业务文档/测试报告的命令。

---

## 插件与技能

- 用户级插件在 **Installed Plugin Snapshot**，须写入 `[plugins].enabled`（User/Project 默认 disabled）。
- 斜杠有 `/ask-atlas` = 插件已加载；有 `/datachain-diagnosis` = Native Skill 已扫到。二者都在 VS Code 可用，说明用户级发现是通的。
- `~/.claude/skills` 默认兼容扫描，所以磁盘上会「只看到 Claude 用户目录」。插件 skill **不会**出现在 `~/.atlas/skills`。
- 项目级：`.atlas/agents` 会扫；**`.atlas/plugins` / `.atlas/skills` 尚未对齐**（项目插件/skill 仍看 `.grok` / `.claude`）。
- 企业安装尽量同时装 `atlas-sdd` + `atlas-skills`。只升其中一个，查 `installed-plugins/registry.json` 的版本，不要只看源码 `plugin.json`。
- 装插件时多源未扫全会拒绝解析；显式 `atlas-sdd@<qualifier>` 或保证 marketplace 源可扫。
- Grill 只在父会话跑。Role 1 不做第二轮 `ask_user_question`。
- 兼容单向：Atlas 会扫 `~/.claude/skills`；Claude Code / Cursor **原生**不会扫 `~/.atlas/installed-plugins`。完整 `/ask-atlas` + 角色链只跑在 Atlas 运行时（含 Cursor 里的 Atlas 扩展）。把 `SKILL.md` 拷到对方 skills 目录只能当方法论；8 个 Role 不会自动变成子 agent。
- 企业通用规则跟安装包 / 插件走，不另开运行时下发通道（方案已定，未改发现优先级）。
- Relay Demo 聊天页自己不扫 skills：斜杠来自 ACP `available_commands_update` / `x.ai/commands/list`。`/skills` / `/skill` 只开本页面板；执行仍是 `session/prompt` 发 `/<已广告的名字>`。

---

## 客户端与编辑器

- 文案、终端 tab、user-guide 路径用 **Atlas Home**，不要搜 `~/.grok/docs/user-guide`。
- Desktop 更新地址与 CLI 同一企业通道（`cli_update_base_url` / `22255/atlas`），不是独立 8888。
- VS Code Settings 可读写用户 `[model.*]`（跳过 `managed`）；`x.ai/models/update` 刷新 picker。
- MCP `command = "npx"` 每个 server 会拉 Node；多个 mysql MCP 会叠出许多 Node，内存暴涨。工程级 MCP 写项目配置，不要写进用户全局。
- ACP 子 agent 若复用同一 `tool call id`，上游会 `400 duplicated`（入口常是 VS Code）。
- 上游 [grok-build-vscode](https://github.com/Eric9701/grok-build-vscode) 用独立 remote 同步，不要往 `Eric9701/grok-build` 推（会 403）。
- IDEA 侧：在 grok-build-vscode 上增加 Atlas 后端，与 Claude/Codex 并列。
- IDEA 默认 Ask + Windows PowerShell：超时/清会话必须走 ACP `cancelled`，不能回 `reject-once`（否则会话里会写成 `User rejected the execution`）。终端宿主用 `powershell.exe -NoProfile -NonInteractive -Command`，不要 `$SHELL`/`-l -c`。Windows 上不要 `detached` spawn，否则 PowerShell 标准输出是空的。`session/request_permission` 已批准后，`terminal/create` 不再弹第二次。

---

## 进程拓扑与中继

日常企业用法是 Embedded TUI、`atlas agent stdio`（VS Code / IDEA）、`atlas agent headless`（relay-demo）。另外两条是上游能力，不是 Atlas 对外产品名：

| 路径 | 谁跑 Agent | 注意 |
|---|---|---|
| Embedded | 当前 TUI 进程 | 直接开 `atlas` |
| Leader | 长驻本机共享进程 | 默认 `[cli] use_leader` **关**；按需拉起，不是开机服务 |
| Agent-host | 独立 daemon / worker | Desktop / Computer Hub / 远程工作区；UI 和 Agent 不在同一进程 |
| Headless | 本进程出站连 WS | relay-demo 文档路径 |

两条「relay」线协议不同：

| 协议 | 入口 | 谁用 |
|---|---|---|
| ACP-over-WS | `--grok-ws-url` | `headless` / 裸 `leader`、[atlas-relay-demo](../services/atlas-relay-demo/CONTEXT.md) |
| Bot-relay | `kind: bot_client` + `bot.*` | Computer Hub；见 [bot-relay-协议.md](./bot-relay-协议.md) |

Leader **能**走 ACP-over-WS，**不能**走 bot-relay。给 relay-demo 当盒子：裸 `atlas agent leader --grok-ws-url …`（不要加 `--relay-on-demand`）。TUI 自动拉起的 Leader 带 `--relay-on-demand`，只服务本机界面，**不会**出现在 demo 聊天页。

Relay Demo 运维：

- 默认监听本机 **非回环 IPv4:2420**，不要用 `127.0.0.1`（跨机 CLI 拉不了 `/sample-docs`）。`--grok-ws-origin` 必须与浏览器地址同源。
- 身份：`?agent_id=` → `x-userid` → `anonymous-<序号>`。同 id 新连接顶旧连接。聊天页同时只绑一个 Agent。
- 文案 `Headless mode requires a grok.com session`：当前登录不是第一方会话（issuer 对不上 / BYOK），relay 门控不连。先 `atlas login --device-auth`。
- Linux 交叉编：`services/atlas-relay-demo/scripts/build-linux.sh`（`CGO_ENABLED=0`）。
- 云端派工：ACP **没有**云→Agent 推文件。三轮 = URL 落 Documents Contract → 只跑 Role 4（不要 Role 1/2/3，不要整段 `/implement`）→ `execution-report-<jobId>.json`。`POST /dispatch/prompts` 只生成文案。
- 聊天气泡用本页 `md.js` 渲染；思考/系统/工具行纯文本。

要把 ACP-over-WS 对接 Claude Code / Cursor：独立的是 **Hub + stdio↔WS 适配器**，不是把 `headless + grok.com 会话` 整段抽出去。各家 Agent 用官方 stdio ACP 被 spawn。适配器尚未落地。

---

## 合入上游 overlay

`origin/main` 合进 `feature-atlas` 时：以上游签名/结构为底，叠 Atlas 文案与默认值。

- 家目录 `~/.atlas`、命令 `atlas`、Device Auth 默认 true、Startup Session Gate（`require_session_at_startup` 必须是 **`pub fn`**，不是 `pub(crate)`）。
- auth 在 `xai-grok-login`：用 `crate::oidc` / `crate::device_code`，不要 `crate::auth::`，不要不存在的 `devbox_login`。
- `flow.rs` 保留 `should_use_device_flow(login_override, config_device_flow, proxy_base_url)`，`.default(true)`。
- 托管模型 ENC、`ring` 在 `[dependencies]`、Task Report 默认开，都要留下。
- user-guide 冲突：origin/main 新章节（folder trust、Esc 不再取消 turn 等）保留，把 `.atlas` / `atlas login` / Atlas Home 叠上去。
- vscode 走独立 remote，不要往 `Eric9701/grok-build` 推。

---

## 构建与发布

- CLI：`cargo build -p xai-grok-pager-bin --release`，产物 `xai-grok-pager`，安装改名为 `atlas`。版本靠 `GROK_VERSION`。
- Windows Rust：`PROTOC=bin/protoc-win64/bin/protoc.exe`。`ring` 必须在 `[dependencies]`。
- Linux **官方 x86_64 包用 musl**。Ubuntu 上默认 gnu 链出来的二进制在 CentOS 7（glibc 2.17）会 `GLIBC_2.xx not found`。musl 下 `sqlite-vec` 缺 `u_int8_t`：用仓库 `.cargo/config.toml` 的 `CFLAGS_*`。
- vendor 必须完整（缺 `vendor/nucleo` 会编不过）。Release 拉 ripgrep 需要 GitHub 或 `GROK_TOOLS_BUNDLE_RG_PATH`。
- atlas-server：`CGO_ENABLED=0`；Windows `go build -o atlas-server.exe ./cmd/server`；Linux `scripts/build-linux.sh`。Go 1.25+。
- 新表 collation 对齐 `users.user_id`，否则 FK Error 1215。
- 稳定通道：`/cli/stable`；企业安装脚本从 `22255/atlas` 拉包并尽量装 `atlas-sdd`。

---

## 已知缺口（会话里确认、尚未当缺陷修）

- 项目级 `.atlas/plugins`、`.atlas/skills` 未纳入发现。
- 用户指南 / 部分 prompt 仍可能指向 `.grok` 路径（已按 case 改过，回归时再搜一遍）。
- MCP stdio + `npx` 多实例内存问题：配置层面规避，未改运行时共享。
- 通用 stdio↔WS 适配器未抽：relay-demo Hub 只接 `atlas agent headless`，不能直接 spawn Claude / Cursor ACP。
- CLI / atlas-server 没有业务文档或测试报告上传口；派工回执目前只经 ACP 抄回或 Agent 自己 POST。

---

## 会话索引

| 时段 | 会话 | 主题 |
|---|---|---|
| 2026-07-22 → 07-29 | [企业化起点](ba632f70-aedc-4426-b76e-b57346e56fc7) | 登录、Atlas 品牌、atlas-server、Task Report 初版 |
| 2026-07-22 → 08-12 | [主线建设](7b869ee4-cf94-48f3-9801-068610925442) | 托管模型、遥测门控、VS Code 适配、SDD×skills、marketplace |
| 2026-08-12 → 08-23 | [能力与运维](73fbb1a7-df21-4ab0-9c9f-e5d058a2840b) | 登录门控、ENC 缓存、流程打磨、编译手册、MCP、插件发现 |
| 2026-08-16 → 08-23 | [流程与排障](dee2989c-ad51-4479-8490-3abcafe1c69d) | 评测会话是否走 SDD、架构不强制、重复 tool id、误 commit |
| 2026-08-21 | [用户指南 HTML](b1660ce9-71e4-4b60-937a-e2bdb2ab04a0) | `docs/atlas-用户指南.html` |
| 2026-08-21 | [IDEA 扩展](f8a5f924-225c-4836-8505-2a728e8f851a) | jetbrains-cc-gui 增加 Atlas 后端 |
| 2026-08-23 | [老 glibc 构建](396a5ad5-3ab6-4b8b-a056-a8e75e086652) | CentOS 7 / musl / sqlite-vec |
| 2026-08-24 | [Token 续期](2f91e8e7-8182-4f37-a5d8-d69e2686c5a2) | refresh 落库，防 Task Report anonymous |
| 2026-08-24 | [Agent 展示](3750f595-6a0e-43e3-8f7d-41f1be702957) | Task Report 去 `grok-` 前缀 |
| 2026-08-24 | [MCP 资源](699a63c3-636c-49d9-91fb-8e19ea8e323d) | npx 多实例内存 |
| 2026-08-24 → 08-26 | [规则下发与 overlay](61031787-835f-4e05-ade6-0a8f5c68e17a) | 规则跟包装；tool id / GLM-5 |
| 2026-08-26 | [刷新 model](0076d771-ca2e-4b30-9f24-533fea368fe4) | `atlas models refresh` / `/refresh-model` |
| 2026-08-26 → 09-02 | [工作画像](075a1108-c3dd-4976-850b-286b649c24ab) | `user-work-profile` skill |
| 2026-08-27 → 09-01 | [工作台与中继](8fe4e3ad-2535-415c-8d7b-68a472203fb6) | atlas-relay-demo、派工三轮 |
| 2026-08-31 | [子 agent 产出物](674eaaa2-f6bb-4a51-a8be-9c5531608150) | Task Report artifacts |
| 2026-08-31 | [创建 skill 路径](954e513d-9243-4b31-9bd9-95f584df3127) | `.grok` → `.atlas`；probe bundle |
| 2026-08-31 | [插件给外部 IDE](e490fc43-4c50-4252-989e-c72cae42380c) | 兼容单向 |
| 2026-08-31 → 09-11 | [合并上游](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f) | vscode / jetbrains / CLI overlay；agent-host；Leader |
| 2026-09-02 | [IDEA 斜杠](acbbd656-b088-4270-9e84-fbd48ae62ca0) | grok/atlas 不用 Claude 内置 `/` |
| 2026-09-02 | [变更记录](54b536e1-6151-41a6-bccc-39bad21556c3) | `docs/changelog/` |
| 2026-09-03 | [Case001 权限误拒](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f) | IDEA 默认模式 PowerShell 误报拒绝 |
| 2026-09-03 | [429 策略](1abaf4ee-4818-4837-a401-a134ebc6c693) | 429 ≠ overloaded |
| 2026-09-05 | [Bot 协议](6ccf75f6-0232-4bd8-b8a1-05191470efbb) | bot-relay vs ACP-over-WS |
| 2026-09-09 | [指南冲突](51fdfd92-ad46-4103-b0a7-f7485d0c2265) | user-guide overlay |
| 2026-09-10 | [登录冲突](ad8b8766-e90d-4542-8a33-d5c8217d6910) | auth → xai-grok-login |
| 2026-09-11 | [Relay Demo Linux](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f) | musl 无关；`CGO_ENABLED=0`；LAN IP |
| 2026-09-12 | [Relay 斜杠与 MD](b34f81f3-7ffc-409c-b31d-96b84cf24a65) | Command Catalog + `md.js` |
| 2026-09-13 | [会话记忆](57e9ab26-cafd-4fb7-bd67-a823e1b5cd61) | 本文回填 |

子 agent 记录不另建索引，结论已折进上表对应主题。产品变更明细见 [changelog](./changelog/README.md)。
