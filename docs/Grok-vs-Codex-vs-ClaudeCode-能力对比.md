# Grok Build vs Codex CLI vs Claude Code 能力对比

> 本文对比 **Grok Build（`grok`）**、**OpenAI Codex CLI（`codex`）** 与 **Anthropic Claude Code（`claude`）** 三款终端 AI 编码代理，重点回答"Grok 在 agent 定义、skill 调用等扩展能力上与另外两家的差距"。
>
> Grok 侧结论基于本仓库源码与 `crates/codegen/xai-grok-pager/docs/user-guide/` 用户指南；Codex / Claude Code 侧基于公开资料。三款产品迭代很快，个别细节可能已变化，请以各自官方文档为准。
>
> 图例：✅ 支持 ｜ ⚠️ 部分支持/较弱 ｜ ➖ 无对应概念 ｜ ❌ 不支持

---

## 目录

- [一、总体结论](#一总体结论)
- [二、扩展能力对比（你关心的重点）](#二扩展能力对比你关心的重点)
- [三、核心 agent 运行时对比](#三核心-agent-运行时对比)
- [四、集成与分发形态对比](#四集成与分发形态对比)
- [五、Grok 独有 / 领先的能力](#五grok-独有--领先的能力)
- [六、Grok 的真实差距（按优先级）](#六grok-的真实差距按优先级)
- [七、选型建议](#七选型建议)

---

## 一、总体结论

**在"agent 定义 / skill / hooks / plugins / subagent"这些扩展体系上，Grok Build 已经补齐甚至反超**——它的 persona/role 的输入输出契约链、subagent 的 worktree 隔离，比另外两家更细。

Grok 的真实差距集中在**产品化 / 生态 / 集成面**，而非核心 agent 能力面：

1. 第一方 IDE 扩展深度（无原生 VS Code 扩展，JetBrains 未上线）
2. 云端异步任务形态（无对标 Codex Cloud / Claude Code on web 的托管产品）
3. GitHub 原生协作闭环（无第一方 PR 审查机器人）
4. 生态存量（机制齐全但社区内容远少）
5. 多模型路由与 Windows 官方支持

---

## 二、扩展能力对比（你关心的重点）

| 能力 | Grok Build | Claude Code | Codex CLI |
|------|:----------:|:-----------:|:---------:|
| **Agent 定义**（`.md` + frontmatter，含 model/tools/prompt） | ✅ `.grok/agents/*.md`（+ 内置 `general-purpose`/`explore`/`plan`） | ✅ `.claude/agents/*.md` | ⚠️ 弱/后期才有 |
| **Persona / Role**（行为叠加层、能力模式、I/O 契约） | ✅ personas + roles + `inputs/outputs` 契约链 | ➖ 无独立 persona 概念 | ❌ |
| **Subagent**（并行子会话、独立上下文窗口） | ✅ + `capability_mode` / `isolation:worktree` / `resume_from` | ✅ | ⚠️ 有限 |
| **Skill 调用**（SKILL.md、自动/斜杠触发、参数模板） | ✅ 支持 `$ARGUMENTS`/`$ARGUMENTS[N]`/`$N`/`${SKILL_DIR}`；兼容 `~/.claude`、`~/.cursor` skills | ✅（原生） | ⚠️ 较新 |
| **Slash 命令** | ✅ 大量内置 + skill/plugin 命令 | ✅（`.claude/commands`） | ✅（prompts） |
| **命令内联能力**（`!bash` 注入输出、`@file` 内联文件） | ⚠️ 支持参数/路径 token 替换，未见 `!bash`/`@file` 内联 | ✅ | ⚠️ |
| **Hooks**（生命周期事件，可 deny 工具调用） | ✅ 14 种事件，兼容 Claude/Cursor hook 格式 | ✅ | ⚠️ 较弱 |
| **Plugins + Marketplace** | ✅ 安装/信任/`require_sha`/CLI/TUI | ✅ | ❌ |
| **MCP（客户端）** | ✅ OAuth + `search_tool`/`use_tool` 按需调用 | ✅ | ✅ |
| **项目规则**（AGENTS.md/CLAUDE.md 累积、目录作用域优先级） | ✅ 兼容 Claude/Cursor 文件名与目录 | ✅ | ✅（AGENTS.md） |
| **Memory（跨会话记忆）** | ✅ experimental：`/flush` `/dream` `/remember` | ✅ | ➖ |
| **Output styles（全局输出风格切换）** | ⚠️ persona 仅作用于 subagent，主会话靠 agents/rules 近似 | ✅ | ➖ |

小结：你直接问的**「agent 定义、skill 调用」两项，Grok 完全对齐**，不存在功能缺口。

---

## 三、核心 agent 运行时对比

| 能力 | Grok Build | Claude Code | Codex CLI |
|------|:----------:|:-----------:|:---------:|
| Agentic loop（工具循环） | ✅ | ✅ | ✅ |
| 流式响应（SSE） | ✅ | ✅ | ✅ |
| 上下文压缩 / auto-compact | ✅ 多点触发 + two-pass | ✅ | ✅ |
| Plan mode（规划模式） | ✅ | ✅ | ⚠️ |
| Checkpoint / Rewind（回退） | ✅ FS+hunk+git 快照 | ✅ | ⚠️ |
| 后台任务 / 并行 | ✅ 后台 task + subagent | ✅ | ⚠️ |
| 权限 / 审批模式 | ✅ ask / auto(分类器) / always-approve | ✅ | ✅ approval modes |
| 沙箱隔离 | ✅ nono(Landlock/Seatbelt) + bwrap + 子进程 seccomp 断网 | ✅ bash sandbox | ✅ seatbelt/landlock |
| Headless / 脚本模式 | ✅ `grok -p` | ✅ `claude -p` | ✅ `codex exec` |
| 会话恢复 / fork | ✅ `/resume` `/fork` `resume_from` | ✅ | ✅ |
| 配置 profile | ✅ agent profile / config | ✅ | ✅ profiles |

---

## 四、集成与分发形态对比

| 能力 | Grok Build | Claude Code | Codex CLI |
|------|:----------:|:-----------:|:---------:|
| 官方 VS Code 扩展 | ❌（走 ACP） | ✅ | ✅ |
| JetBrains 插件 | ⚠️ coming soon | ✅ | ✅ |
| 编辑器协议 | ✅ **ACP**（Zed/Neovim/Emacs/marimo） | ➖ 自有扩展 | ➖ 自有扩展 |
| 云端异步任务（托管） | ❌（有 serve/relay/headless 积木，无托管产品） | ✅ Claude Code on web | ✅ Codex Cloud |
| GitHub PR 机器人 / Action | ❌ | ✅ `@claude` / Action | ✅ code review |
| SDK | ✅ ACP SDK（TS/Rust/Python/Go/Kotlin） | ✅ Claude Agent SDK（TS/Python） | ✅ SDK（TS） |
| 多 provider 模型路由 | ❌（xAI Grok + 有限 BYOK） | ❌（Claude 系） | ❌（OpenAI 系） |
| Windows 支持 | ⚠️ best-effort、官方未测试 | ⚠️ 历史需 WSL | ✅ 原生较好 |
| 企业治理（策略下发/审计/SSO） | ⚠️ folder-trust / managed workspace / `require_sha` | ✅ 商业版 managed settings | ✅ org 功能 |

---

## 五、Grok 独有 / 领先的能力

- **多媒体生成**：`image_gen` / `image_edit` / `image_to_video` / `reference_to_video`（`/imagine`、`/imagine-video`）——Codex / Claude Code 均无。
- **自主目标模式**：`/goal`（`update_goal` 工具），跨轮次朝目标推进并汇报进度。
- **定时 / 循环任务**：`/loop` + `scheduler_*` 工具（`Ns/Nm/Nh/Nd` 间隔，7 天自动过期）。
- **Persona/Role 的 I/O 契约链**：persona 可声明 `inputs`/`outputs`，一个 persona 的输出文件成为下一个的输入，链式编排——比 Claude subagent 更细。
- **Leader 共享进程架构**：单机唯一 leader 托管共享 agent，多个 client（TUI/IDE/headless）复用会话与 MCP。
- **子进程级网络隔离**：主进程联网（需连 LLM），子 shell 用 seccomp 单独断网。
- **Voice 输入**（`xai-grok-voice` crate）、内置 **Dashboard**、`/btw` 旁路提示、Claude/Cursor 生态兼容（skills/hooks/rules/settings 直接读取）。

---

## 六、Grok 的真实差距（按优先级）

### 1. IDE 集成深度 —— 最明显短板
Grok 依赖 **ACP** 接入编辑器（Zed / Neovim / Emacs），**无第一方深度 VS Code 扩展**，JetBrains 仍 "coming soon"。Claude Code 与 Codex 都提供官方 VS Code 扩展（内联 diff、审批 UI、侧边栏），Codex 另有成熟 JetBrains 插件。对多数 VS Code / JetBrains 用户是实打实的体验差距。

### 2. 云端 / 异步任务形态
Grok 有 `serve` / `headless` / relay 等积木，但**没有对标 Codex Cloud / Claude Code on web 的托管产品面**。"提交任务、关机、回来看结果/PR"的工作流需自建。

### 3. GitHub 原生协作
缺少第一方 **GitHub App / Action / PR 审查机器人**（Claude `@claude`、Codex code review）。团队"PR 里 @机器人自动改/审"的闭环缺位。

### 4. 生态存量
Plugin / Marketplace / Skill 机制齐全，但**社区内容存量**远小于 Claude Code（后者有大量现成 subagents/skills/commands）。机制到位，生态尚薄。

### 5. 模型与平台
- **模型锁定** xAI Grok（+ 有限 BYOK），无多 provider 路由。
- **Windows 官方 best-effort、未测试**（README 明示仅 macOS/Linux 为受支持构建平台）；Codex 原生 Windows 支持更好。

### 6. 细粒度特性
- **Output styles**：Claude 有全局输出风格/人格一键切换；Grok 的 persona 仅作用于 subagent，主会话只能用 agents/rules 近似。
- **命令内联**：Grok 命令/skill 支持参数与路径 token（`$ARGUMENTS`/`$N`/`${SKILL_DIR}`…），但未见 Claude 那样在命令体内联执行 `!bash` 注入输出、`@file` 内联文件内容（此点未在代码中完全证实，供参考）。
- **企业治理**：有 folder-trust、managed workspaces、`require_sha`，但集中策略下发/审计/SSO/SCIM 等成熟度不及两家商业版。

---

## 七、选型建议

| 场景 | 推荐 |
|------|------|
| 重度使用 VS Code / JetBrains，要内联 diff 与图形审批 | Claude Code 或 Codex |
| 需要云端异步跑任务、GitHub PR 自动化闭环 | Codex（Cloud）/ Claude Code（web + Action） |
| 已有 Claude/Cursor 的 skills/hooks/rules 想直接复用 | **Grok Build**（原生兼容读取） |
| 需要多媒体生成、自主目标、定时任务、精细 subagent 编排 | **Grok Build**（独有/领先） |
| 在 Zed / Neovim / Emacs 中通过 ACP 接入 | **Grok Build**（ACP 原生） |
| 深度定制 agent/persona/role + I/O 契约链 | **Grok Build** |

**一句话**：Grok 在核心 agent 能力与扩展体系上不输甚至领先；短板在 IDE 扩展、云端/协作产品面与生态存量。
