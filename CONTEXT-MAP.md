# Context Map

## Contexts

- [Agent Engineering Process](./CONTEXT.md) — how Atlas routes R&D through skills and atlas-sdd role agents
- [Atlas Runtime](./docs/CONTEXT-runtime.md) — Atlas Home、Native/Plugin Skill、登录门控
- [Atlas Server Access](./services/atlas-server/CONTEXT.md) — accounts, User Groups, and managed model entitlement for the enterprise proxy
- [项目记忆](./docs/project-memory.md) — 会话沉淀的约定与易错点（不是术语表）
- [atlas-runtime 安装与使用手册](./docs/atlas-runtime-安装手册.md) — CLI、VS Code 扩展与基础命令
- [Atlas 编译手册](./docs/atlas-编译手册.md) — Windows / Linux musl / macOS

## Relationships

- **Agent Engineering → Atlas Server Access**: delivery process only; no shared domain terms required for User Groups
- **Atlas Runtime → Atlas Server Access**: Device Auth 与 ListModels / settings 由服务端计算；客户端只消费
- **Atlas Runtime → Agent Engineering**: enabled 插件把 Main Flow / Role Executor 以斜杠命令送进会话
- **Atlas Server Access → CLI / VS Code**: clients call `/atlas/v1/models`; Effective Model Set is computed server-side
