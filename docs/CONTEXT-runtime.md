# Atlas Runtime

客户端进程如何找到家目录、登录态、技能与插件。与研发流程术语（根 `CONTEXT.md`）和云端账号模型（`services/atlas-server/CONTEXT.md`）分开。

## Language

**Atlas Home**:
`GROK_HOME` 解析后的用户态根，默认 `~/.atlas`（Windows：`%USERPROFILE%\.atlas`）。`config.toml`、会话、安装登记都落在这里。
_Avoid_: 把 `~/.grok` 当当前家目录；把 `~/.claude` 当 Atlas Home

**Native Skill**:
松散目录里的 `SKILL.md`：`Atlas Home/skills`，以及默认兼容的 `~/.claude/skills`。
_Avoid_: Plugin Skill；以为插件会复制进 `~/.atlas/skills`

**Plugin Skill**:
marketplace 插件包内的 `skills/` 与 `commands/`。只在插件 **enabled** 时进入斜杠列表。
_Avoid_: 用用户 skill 文件夹判断插件是否加载

**Installed Plugin Snapshot**:
`Atlas Home/installed-plugins/` 下的完整目录拷贝，由 `registry.json` 登记。用户级 marketplace 安装落在这里，不是 `Atlas Home/plugins/`。
_Avoid_: `~/.atlas/plugins` 作为安装位置；把 Claude `known_marketplaces.json` 当成 Atlas 安装登记

**Startup Session Gate**:
只在**新进程启动**时检查登录态。默认开启（`require_session_at_startup`，不必写进 toml；写 `false` 可关）。不打断已在跑的任务。
_Avoid_: 每个 turn 验登录并中止现会话

**Device Auth Login**:
企业登录命令：`atlas login --device-auth`。VS Code 未登录引导也走这条。机器码登录要求该用户已在 atlas-server 开户。
_Avoid_: 裸 `atlas login` / `grok login` 作为企业引导；未开户仅凭机器码进入

**Remote Fetch**:
`[features] remote_fetch`（默认 true）。为 false 时跳过启动时远程 settings（`/settings`）与模型目录拉取。断网 / 代理不可达时必须关掉，否则 bootstrap 约 40s 超时（*loading your account settings*）。无环境变量；`GROK_CONFIG` 覆盖无效。
_Avoid_: 只关 `require_session_at_startup` 却仍卡在 settings；以为 `GROK_REMOTE_FETCH=0` 有效
