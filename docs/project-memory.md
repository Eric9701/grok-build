# 项目记忆

2026-07-22 至 2026-09-02 本工程 Cursor 会话沉淀。**术语**以 CONTEXT 为准；**实现**以代码与手册为准。本文只留反复踩过的约定和易错点。**五件套改了什么**写 [docs/changelog](./changelog/README.md)。

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

---

## 遥测

- `trace_upload_enabled=false` **只关** trace 产物上传，不管 signals / events / Task Report。
- Task Report **默认开**，仅 `GROK_DISABLE_TASK_REPORT=1` 可关。主会话、plan 模式、子 agent 都报。
- 尽量全关：`telemetry_enabled=false` + `trace_upload_enabled=false` + `GROK_DISABLE_TASK_REPORT=1`。
- Remote settings：`download/probe_settings.json` → `GET /atlas/v1/settings`。
- Admin：`/atlas/admin/task-reports`；明细可弹详情；按人排行可按 token / 任务数排。只改 `web` 即可动该页。
- Agent 展示去掉上游 `grok-` 前缀：`grok-build-plan` 显示为 `build-plan`；库内仍存原文。鼠标悬停可看原值。

---

## 插件与技能

- 用户级插件在 **Installed Plugin Snapshot**，须写入 `[plugins].enabled`（User/Project 默认 disabled）。
- 斜杠有 `/ask-atlas` = 插件已加载；有 `/datachain-diagnosis` = Native Skill 已扫到。二者都在 VS Code 可用，说明用户级发现是通的。
- `~/.claude/skills` 默认兼容扫描，所以磁盘上会「只看到 Claude 用户目录」。插件 skill **不会**出现在 `~/.atlas/skills`。
- 项目级：`.atlas/agents` 会扫；**`.atlas/plugins` / `.atlas/skills` 尚未对齐**（项目插件/skill 仍看 `.grok` / `.claude`）。
- 企业安装尽量同时装 `atlas-sdd` + `atlas-skills`。只升其中一个，查 `installed-plugins/registry.json` 的版本，不要只看源码 `plugin.json`。
- 装插件时多源未扫全会拒绝解析；显式 `atlas-sdd@<qualifier>` 或保证 marketplace 源可扫。
- Grill 只在父会话跑。Role 1 不做第二轮 `ask_user_question`。

---

## 客户端与编辑器

- 文案、终端 tab、user-guide 路径用 **Atlas Home**，不要搜 `~/.grok/docs/user-guide`。
- Desktop 更新地址与 CLI 同一企业通道（`cli_update_base_url` / `22255/atlas`），不是独立 8888。
- VS Code Settings 可读写用户 `[model.*]`（跳过 `managed`）；`x.ai/models/update` 刷新 picker。
- MCP `command = "npx"` 每个 server 会拉 Node；多个 mysql MCP 会叠出许多 Node，内存暴涨。工程级 MCP 写项目配置，不要写进用户全局。
- ACP 子 agent 若复用同一 `tool call id`，上游会 `400 duplicated`（入口常是 VS Code）。
- 上游 [grok-build-vscode](https://github.com/Eric9701/grok-build-vscode) 用独立 remote 同步，不要往 `Eric9701/grok-build` 推（会 403）。
- IDEA 侧：在 grok-build-vscode 上增加 Atlas 后端，与 Claude/Codex 并列。
- IDEA 默认 Ask + Windows PowerShell：超时/清会话必须走 ACP `cancelled`，不能回 `reject-once`（否则会话里会写成 `User rejected the execution`）。终端宿主用 `powershell.exe -NoProfile -NonInteractive -Command`，不要 `$SHELL`/`-l -c`。

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
| 2026-08-26 | [刷新 model](0076d771-ca2e-4b30-9f24-533fea368fe4) | `atlas models refresh` / `/refresh-model` |
| 2026-08-26 → 09-02 | [工作画像](075a1108-c3dd-4976-850b-286b649c24ab) | `user-work-profile` skill |
| 2026-08-27 → 09-01 | [工作台与中继](8fe4e3ad-2535-415c-8d7b-68a472203fb6) | atlas-relay-demo |
| 2026-08-31 | [创建 skill 路径](954e513d-9243-4b31-9bd9-95f584df3127) | `.grok` → `.atlas`；probe bundle |
| 2026-08-31 → 09-02 | [合并上游](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f) | vscode 4.1.0、jetbrains 0.5.5 |
| 2026-09-02 | [IDEA 斜杠](acbbd656-b088-4270-9e84-fbd48ae62ca0) | grok/atlas 不用 Claude 内置 `/` |
| 2026-09-02 | [变更记录](54b536e1-6151-41a6-bccc-39bad21556c3) | `docs/changelog/` |
| 2026-09-03 | [Case001 权限误拒](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f) | IDEA 默认模式 PowerShell 误报拒绝 |

子 agent 记录不另建索引，结论已折进上表对应主题。产品变更明细见 [changelog](./changelog/README.md)。
