# Context Map

## Contexts

- [Agent Engineering Process](./CONTEXT.md) — how Atlas routes R&D through skills and atlas-sdd role agents
- [Atlas Server Access](./services/atlas-server/CONTEXT.md) — accounts, User Groups, and managed model entitlement for the enterprise proxy
- [atlas-runtime 安装与使用手册](./docs/atlas-runtime-安装手册.md) — CLI、VS Code 扩展与基础命令

## Relationships

- **Agent Engineering → Atlas Server Access**: delivery process only; no shared domain terms required for User Groups
- **Atlas Server Access → CLI / VS Code**: clients call `/atlas/v1/models`; Effective Model Set is computed server-side
