# Atlas 产品变更记录

按产品拆开记 **本仓库企业化叠加层** 改了什么。上游 grok-build 的 `Synced from monorepo` 不在这里展开。

| 产品 | 记录 | 代码位置 |
|---|---|---|
| atlas-cli | [atlas-cli.md](./atlas-cli.md) | `crates/codegen/xai-grok-*`，安装产物改名为 `atlas` |
| atlas-plugin | [atlas-plugin.md](./atlas-plugin.md) | 源：`services/skills/`；发布仓：`https://gitlab.imyai.cn/zhangyufeng/atlas-plugins.git`（`atlas-sdd` + `atlas-skills`） |
| grok-build-vscode | [grok-build-vscode.md](./grok-build-vscode.md) | `services/grok-build-vscode`（VS Code / Desktop 叠加层） |
| jetbrains-cc-gui | [jetbrains-cc-gui.md](./jetbrains-cc-gui.md) | `services/jetbrains-cc-gui`（独立 git，IDEA 插件） |
| atlas-server | [atlas-server.md](./atlas-server.md) | `services/atlas-server` |

旁路（不进五件套主表）：[Atlas Relay Demo](../../services/atlas-relay-demo/CONTEXT.md) 用现有 `atlas agent headless --grok-ws-url`，不改 CLI、不进 atlas-server。

约定与易错点仍写 [project-memory.md](../project-memory.md)。术语写 CONTEXT。决策写 ADR。这里只记「哪次会话改了哪个产品的哪条行为」。

---

## 以后怎么记

改完五件套之一就记账，不必等用户再说「记一下」。分析了但没改代码也写一行，状态标 **分析未改**。

1. 打开对应 `docs/changelog/<产品>.md`。
2. 在该文件**最上面的日期节**插入一条（最新在上）。跨月则新建 `## YYYY-MM`。
3. 一条一事；用户可感知或后会话会踩的才写。不记纯 merge、格式化、看日志、重跑编译。
4. 跨产品同一需求：每个被改到的产品各写一条，互相链一下。

模板：

```md
### YYYY-MM-DD — 一句话标题

- **状态**：已落地 | 部分落地 | 分析未改 | 文档
- **会话**：[短标题](uuid-without-jsonl)
- 改了什么、为什么（1–4 条）。关键路径用反引号。
```

状态含义：

| 状态 | 何时用 |
|---|---|
| 已落地 | 代码或已发布插件已改 |
| 部分落地 | 主路径改了，已知缺口未关 |
| 分析未改 | 查清了，有意不改或留给配置/文档 |
| 文档 | 只动手册 / ADR / 本变更记录 |

---

## 会话索引

| 时段 | 会话 | 主要落点 |
|---|---|---|
| 2026-07-22 → 07-29 | [企业化起点](ba632f70-aedc-4426-b76e-b57346e56fc7) | cli、server |
| 2026-07-22 → 08-12 | [主线建设](7b869ee4-cf94-48f3-9801-068610925442) | 五件套初版 |
| 2026-08-12 → 08-31 | [能力与运维](73fbb1a7-df21-4ab0-9c9f-e5d058a2840b) | cli、server、vscode、plugin |
| 2026-08-16 → 08-23 | [流程与排障](dee2989c-ad51-4479-8490-3abcafe1c69d) | plugin、cli |
| 2026-08-21 | [用户指南 HTML](b1660ce9-71e4-4b60-937a-e2bdb2ab04a0) | 文档（cli 文案回归） |
| 2026-08-21 → 08-23 | [IDEA 扩展](f8a5f924-225c-4836-8505-2a728e8f851a) | jetbrains-cc-gui |
| 2026-08-23 | [老 glibc 构建](396a5ad5-3ab6-4b8b-a056-a8e75e086652) | cli |
| 2026-08-24 | [Token 续期](2f91e8e7-8182-4f37-a5d8-d69e2686c5a2) | cli、server |
| 2026-08-24 | [Agent 展示](3750f595-6a0e-43e3-8f7d-41f1be702957) | server |
| 2026-08-24 | [MCP 资源](699a63c3-636c-49d9-91fb-8e19ea8e323d) | cli（分析未改） |
| 2026-08-24 → 08-26 | [规则下发与 overlay](61031787-835f-4e05-ade6-0a8f5c68e17a) | 方案 + vscode/cli |
| 2026-08-26 → 08-27 | [刷新 model](0076d771-ca2e-4b30-9f24-533fea368fe4) | cli、vscode |
| 2026-08-26 → 09-02 | [工作画像 skill](075a1108-c3dd-4976-850b-286b649c24ab) | plugin |
| 2026-08-27 → 09-01 | [工作台与中继](8fe4e3ad-2535-415c-8d7b-68a472203fb6) | 旁路 relay-demo |
| 2026-08-31 | [子 agent 产出物](674eaaa2-f6bb-4a51-a8be-9c5531608150) | cli |
| 2026-08-31 | [创建 skill 路径](954e513d-9243-4b31-9bd9-95f584df3127) | cli、server |
| 2026-08-31 | [插件给外部 IDE](e490fc43-4c50-4252-989e-c72cae42380c) | plugin（分析未改） |
| 2026-08-31 → 09-02 | [合并上游](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f) | vscode、jetbrains |
| 2026-09-02 | [IDEA 斜杠默认](acbbd656-b088-4270-9e84-fbd48ae62ca0) | jetbrains-cc-gui |
| 2026-09-02 | [整理变更记录](54b536e1-6151-41a6-bccc-39bad21556c3) | 本文档 |
| 2026-09-03 | [Case001 权限误拒](5952b17a-2178-4e04-8f5c-cd0fa0bfda8f) | jetbrains |
