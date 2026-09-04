# atlas-plugin 变更记录

两包、轻耦合（ADR 0001）：

| 包 | 角色 | 源 |
|---|---|---|
| **atlas-skills** | Main Flow（入口 `/ask-atlas`，曾用名 ask-matt） | `services/skills/engineering` |
| **atlas-sdd** | Role Executor + `spec/` 企业基线 | 发布仓 `atlas-plugins`，本仓不再放 `services/sdd` |

发布：`https://gitlab.imyai.cn/zhangyufeng/atlas-plugins.git`。用户级安装在 `~/.atlas/installed-plugins`，须 `[plugins].enabled`。记法见 [README](./README.md)。

## 2026-09

### 2026-09-02 — 建立本变更记录

- **状态**：文档
- **会话**：[整理变更记录](54b536e1-6151-41a6-bccc-39bad21556c3)

### 2026-08-26 → 09-02 — 工作画像 skill

- **状态**：已落地
- **会话**：[工作画像 skill](075a1108-c3dd-4976-850b-286b649c24ab)
- 新增 `user-work-profile`：用 Task Report 做单人/多人工作画像（五维 + L1–L4）。
- 默认取数基址 `http://10.218.220.237:22255`。并行任务按「同一时段并发」计。
- 人员技能评定口径对齐 `docs/ppt/企业级系统开发-Atlas素材.md`。

## 2026-08

### 2026-08-31 — 给 Claude Code / Cursor 用？

- **状态**：分析未改
- **会话**：[插件给外部 IDE](e490fc43-4c50-4252-989e-c72cae42380c)
- 包格式是 Atlas marketplace（`plugin.json` + skills/commands），不是 Claude Code 的 `.claude/skills` 或 Cursor skill 包。
- 外部工具不能直接 `plugin install`。可手工把单个 `SKILL.md` 拷到对方扫描目录；Role Agent / `spec/` 不会自动生效。
- Atlas 反过来能兼容扫描 `~/.claude/skills`。

### 2026-08-16 → 08-23 — 流程打磨

- **状态**：已落地
- **会话**：[流程与排障](dee2989c-ad51-4479-8490-3abcafe1c69d)、[能力与运维](73fbb1a7-df21-4ab0-9c9f-e5d058a2840b)
- **Grill 只在父会话跑**。Role 1 不做第二轮 `ask_user_question`。
- Role 2（架构）两边 Compliance Mode 都可选；「设计文档」→ Role 3，不是 Role 2。
- Role 4 可跑 UT，并出 HTML 测试报告（不强制）。
- 只升其中一个包时，以 `installed-plugins/registry.json` 版本为准，不要只看源码 `plugin.json`。
- 多源未扫全会拒绝解析：显式 `atlas-sdd@<qualifier>`，或保证 marketplace 可扫。

### 2026-08-12 前后 — Light Coupling 与可发现性

- **状态**：已落地
- **会话**：[主线建设](7b869ee4-cf94-48f3-9801-068610925442)
- ADR 0001：skills 与 SDD 分包装，ask-atlas 是唯一编排入口。
- ADR 0002：角色按阶段 **调用** 映射 skill，不把 skill 正文抄进 agent。
- ADR 0003：企业安装尽力同时装 `atlas-sdd` + `atlas-skills`；失败不阻断 CLI 安装。
- ADR 0004：架构文档不是详细设计前置。
- 入口斜杠 `ask-matt` → **`/ask-atlas`**。

## 2026-07

### 2026-07 下旬 — 把 SDD 角色送进 Atlas

- **状态**：已落地
- **会话**：[企业化起点](ba632f70-aedc-4426-b76e-b57346e56fc7)、[主线建设](7b869ee4-cf94-48f3-9801-068610925442)
- 从 `agents` / `.claude/agents` + spec 收成 8 个 Role Executor，经 marketplace 安装。
- 斜杠选 `atlas-sdd:` 角色，而不是每次手写 agent 名。
