# 架构设计不强制；需求分析后默认进入详细设计

需求分析完成后，默认下一阶段是 **Role 3 详细设计**，不是 Role 2 架构设计。「设计文档 / 详细设计」映射到 Role 3。架构文档是可选输入：有则引用，没有则让用户指定路径或确认跳过；**不得**因此自动拉起 Role 2。Role 2 仅在用户明确要「架构 / 架构设计」时作为 Side Ramp。

## Status

accepted

## Considered Options

- **强制 1→2→3**（「设计文档」先 Role 2 再 Role 3）— rejected：多数需求已有系统架构或只需模块级详细设计；每条需求再产一份架构会拖慢门闩，并与 Role 3 的架构概述重复。上一场评测会话正是因此在「做详细设计」时先开了 Role 2。
- **无架构则阻塞详细设计** — rejected：把可选产物变成硬门闩。
- **无架构则自动 spawn Role 2** — rejected：用户没点名架构阶段。
- **详细设计默认下一阶段；架构可选引用** — accepted：父会话查找已有架构（`documents/architecture-design/` 或用户指定路径）；找不到则询问路径或确认「没有」；然后 spawn Role 3。

## Consequences

- ask-atlas / sdd-workflow 默认链改为 Grill → Role 1 → Role 3 → Role 4 → Close。
- **enterprise-strict** 为 1→3→4→6；Role 2 仍不强制。
- CONTEXT **Compliance Mode** 不再把「设计文档直达 Role 3」列为 Avoid。
- Role 3 prompt 把架构文档标为可选输入；不得把缺失架构当成失败。
