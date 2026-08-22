# atlas-admin 管理系统 — 需求与设计共识

> 本文档是 2026-08-20 grill 会话的产物，记录 atlas-admin 独立管理工程的全部已定决策。
> 词典以 `services/atlas-server/CONTEXT.md` 为准；组织树合并决策见 `docs/adr/0005-user-groups-org-tree.md`。

## 1. 项目定位

- **全新独立 git 仓库**：`E:\work\mygit\architechure\atlas-admin`，Go module `github.com/atlas-build/atlas-admin`（远程仓库待用户创建）。
- **技术栈**：Go + 服务端渲染 HTML（无前端框架），与 atlas-server 风格一致。
- **独立进程**，监听 `:22256`（atlas-server 为 `:22255`），TOML 配置（`atlas-admin.toml`），`ATLAS_ADMIN_*` 环境变量覆盖。
- **atlas-server 一行代码不改**：现有 `/admin/*` 页面保留，与新 admin 并存，互不导航。

## 2. 身份与登录

- 身份复用 atlas-server `user` 表（实时直读，无镜像同步）。
- 登录校验直接比对 `PasswordHash`（bcrypt / `bcrypt.DefaultCost`，与 atlas-server 相同算法）。
- admin 独立登录页；会话为 **JWT Cookie（单 token，24h）**；服务端维护 `admin_token_blacklist` 支持吊销。
- 登录时查询角色；**未分配任何角色的用户拒绝登录**（提示联系管理员）。
- 首个管理员：部署时手工 SQL 往 `admin_user_roles` 插入。

## 3. 角色（固定五个，不可增删）

| 角色 key | 名称 | 权限矩阵 |
|---|---|---|
| admin | 管理员 | 用户管理全部、组织全部、模型授权全部、任务报告全部 |
| line_lead | 条线负责人 | 用户管理=本条线成员归属、组织=本条线及以下分组管理、模型授权=本条线、任务报告=本条线 |
| developer | 开发人员 | 用户管理=仅自己、组织只读、模型授权只读、任务报告=仅自己 |
| tester | 测试人员 | 组织只读、任务报告只读 |
| ba | BA | 组织只读、任务报告只读 |

- 角色全局生效（按用户）；仅管理员可分配；允许一人多角色。
- 条线负责人 → 业务条线范围由独立映射表 `admin_line_leads` 决定（与成员关系解耦，一人可负责多条线）。
- 首期不做系统设置页（当前无真实可管理配置项）。

## 4. 组织架构

- `user_groups` 增量加列：`parent_id`（可空自引用）、`node_type`（dept/line/squad），见 ADR-0005。
- 层级：业务部(dept) → 业务条线(line) → 条线分组(squad)；成员与模型授权**只挂 squad 层**，无跨层继承。
- 一个用户可属多个条线分组。
- 存量分组不迁移：`parent_id=NULL` 显示为"待归类"。
- 有子节点/成员/授权的组禁止删除；删除用户级联清理其组织关系与角色映射。

## 5. 数据

- 与 atlas-server 同一 MySQL 实例同一 schema（`atlas` 库），直连读写。
- 新表（`admin_` 前缀）：
  - `admin_roles`：预置 5 角色记录（仅为外键引用与展示）
  - `admin_user_roles`（user_id → role）
  - `admin_line_leads`（user_id → line org_node）
  - `admin_token_blacklist`（jti/payload + 过期时间）
- 建表/加列走**手工 SQL 脚本**（atlas-admin 仓库 `scripts/migrate.sql`），服务启动不自动迁移。
- 模型授权沿用现有 `user_models` / `group_models` 语义（直挂用户 + 分组授权），仅重做页面交互。

## 6. 页面（全量重做，首期清单）

1. 登录页
2. 用户管理（增删改查、角色分配入口、组织归属维护）
3. 组织架构（树形展示：业务部→条线→分组；待归类区）
4. 角色分配（用户↔角色；条线负责人↔条线范围）
5. 模型授权（用户直挂 / 分组授权，含 Effective Model Set 预览）
6. 任务报告（列表 + 条线过滤）

- 交互首期：统一壳（左侧导航 + 顶栏含用户/登出）、列表搜索/筛选/分页/排序、弹窗编辑 + 行内校验。
- 批量操作与树拖拽为二期。
- 视觉：中文界面、蓝色系（#1976d2 家族）、卡片+表格，延续现有管理页风格。

## 7. 任务报告条线过滤

- 条线负责人可见范围按上报人**当前**组织归属实时推导（上报人挂在某条线下任一分组即算该条线的人；跨多线用户的报告在多条线均可见）。
- 历史报告不做条线快照。

## 8. 明确不做（首期）

- 不改 atlas-server 任何代码/表结构（除 `user_groups` 加列由 atlas-admin 迁移脚本执行）。
- 不做 SSO、不做角色增删、不做组织树继承授权、不做批量/拖拽、不做系统设置页、不做普通用户自助门户。
