---
name: user-work-profile
description: >-
  Analyzes Atlas task-report telemetry into a work portrait and skill rating
  (工作画像 / 人员技能评定): five dimensions (流程纪律、并行效率、领域深度、工程效率、工具熟练),
  scorecard, and L1–L4. Supports single-user and multi-user analysis. Use when
  the user asks for 工作画像, 用户画像, 人员技能评定, 五维能力, 评分卡, L1 L2 L3 L4,
  task report 分析, 单人分析, or 多人分析.
disable-model-invocation: true
---

# User Work Profile

用 **Task Report** 做 **过程证据画像**（企业技能评定框架，见 `docs/ppt/企业级系统开发-Atlas素材.md` §23–25）。一行一条 **子代理任务**，不是一轮对话。

这是画像框架，**不是**已上线的考试或绩效考核产品。阈值是示例，用团队中位数校准。没数据不要编。

取数口径：[reference.md](reference.md)。五维与评分卡：[rubric.md](rubric.md)。成稿：[portrait.md](portrait.md)。

## Process

### 1. Scope

缺的只问缺口：

| 项 | 默认 | 完成标准 |
|---|---|---|
| 支路 | 点了具体人 → **单人**；团队/对比/排行/`all` → **多人** | 支路已定 |
| 对象 | 单人：`userId` 或 email；多人：名单或 Top N | 标识已定 |
| 岗位 | 未说则 **全栈/骨干**（五维均衡）。可切：需求 / 开发 / 架构 / QA | 切片已定 |
| 区间 | **近 14 天**（含今天）；禁止不带 `from`/`to` 打 API | `YYYY-MM-DD` 或显式 `all` |
| 交付 | Markdown + 自包含 HTML；Cursor 再加 Canvas | 路径已定 |

身份是报文体 Report User，可 `anonymous`。画像里不要写成登录铁证。

**Done when:** 支路、对象、岗位、`from`/`to` 已写下。

### 2. Acquire

1. 用户已给的 JSON / 导出。
2. Admin API（脚本只依赖标准库）：

```bash
python scripts/fetch_reports.py --from FROM --to TO
python scripts/fetch_reports.py --from FROM --to TO --user-id USER
python scripts/fetch_reports.py --from FROM --to TO --user-id U1 --user-id U2
python scripts/fetch_reports.py --from FROM --to TO --all-users --max-users 15
```

`ATLAS_BASE` 默认 `http://10.218.220.237:22255/`。无 path 时补 `/atlas`。默认剥 `prompt`/`error`。领域深度或失败校准需要原文时，用户明确后再加 `--include-sensitive`。

3. MySQL 仅当用户给了 DSN 且 API 不可用。SQL 见 [reference.md](reference.md)。

单人评定也要拉 **overall**（算空转中位数、组织对照）。`anonymous` 单独成组，不进排名。

**Done when:** 有 overall；单人还有该用户 aggregate + 明细。空结果要说空。

### 3. Measure

先硬指标，再按 [rubric.md](rubric.md) 映到 **五维** 和 **评分卡**。缺维写「数据不足」，不要凑 100 分。

必算：

- 量 / 成 / 产 / 模 / 时 / 域（任务数、成功率、kind、Catalog ID、时段、cwd）
- **A 流程纪律**：`subagentType` 角色覆盖；用 `parentSessionId` 近似「需求包」漏斗 1→3→4（strict 再看 6）
- **B 并行效率**：同一时刻重叠的子代理任务（峰值并行度、并行占比、时间加权平均并行度；算法见 rubric）
- **C 领域深度**：Role 1/3 任务与 doc 产物；无敏感字段则不编「是否引用 CONTEXT」
- **D 工程效率**：`durationMs` / `turns` / `toolCalls`、空转指数 = `tokensUsed / max(artifactCount,1)` 相对团队中位数
- **E 工具熟练**：`clientVersion`、主会话 plan vs explore、Catalog 是否集中；无分配表则不编「模型合规率」

质量误读（Token 高≠低效、成功率低≠能力差、峰值并行高≠一定高效）见 rubric。先流程后快慢：该走企业链却只有 plan/explore 的高产，企业场景记负分；standard 小改动可以只有 plan。

**Done when:** 每一维都有证据或明确不足；`anonymous` / 空 model / 空产物已降权。

### 4. Portray

按 [portrait.md](portrait.md)：一句话人设 + 五维表 + 评分卡 + 建议等级（L1–L4）。多人对照五维与漏斗，不要并列 N 份互不引用的个人摘要。

落盘 `.scratch/work-profiles/<from>_<to>-<scope>.md` 与同名 `.html`。Cursor 再写 Canvas（数据内联，禁止 canvas 内 `fetch`）。标明 **「基于现网过程数据的画像框架」**。

**Done when:** Markdown 与 HTML 已落盘；未泄漏 prompt/error/IP；报告头有区间、数据源、岗位切片、样本上限。

## Guardrails

- 只基于 Task Report（及用户批准的敏感字段 / `documents/` 抽检）。补 session signals 之前先问。
- 多证据，禁止用单一成功率或 Token 排名。
- 对岗位加权（rubric 岗位切片）；不因没出 Role 2 扣架构分。
- 默认不贴 `prompt`、`error`、`clientIp`。指标筛人，专家读文档是校准，不是本技能默认步骤。
- 不当作 HR 终局或绩效考核文。
