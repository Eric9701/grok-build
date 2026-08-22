# Atlas-Admin 管理端重设计 — 会话记录

- **日期**：2026-08-20 ～ 2026-08-21
- **工作区**：`E:\work\mygit\architechure\grok-build`（SDD 文档与决策）；`E:\work\mygit\architechure\atlas-admin`（新独立仓库，实现）
- **模式**：Atlas SDD 全流程（grill-with-docs → Role 1 需求 → Role 3 详细设计 → Role 4 实现 → 代码评审 Close gate）
- **状态**：✅ 已收尾（评审缺陷已修复并提交；遗留优化项已登记）

---

## 1. 原始需求

用户原始诉求（`/ask-atlas`）：

> 重新设计 atlas-server 的 admin 管理功能（独立开发工程），atlas-server 保持不变，新增：
> 1. 用户登录
> 2. 角色管理（管理员、条线负责人、开发人员、测试人员、BA）
> 3. 组织架构（业务部-业务条线-条线分组-开发人员）
>
> 其他保持不变，优化现有页面交互。

核心约束：**atlas-server 代码零改动**，admin 功能以独立工程交付。

## 2. SDD 流程执行情况

| 阶段 | 产物 | 状态 |
|------|------|------|
| grill-with-docs（8 轮） | `docs/atlas-admin-redesign.md`（共识文档） | ✅ |
| 领域建模 | `services/atlas-server/CONTEXT.md` 术语表更新 + `docs/adr/0005-user-groups-org-tree.md` | ✅ |
| Role 1 需求分析 | `documents/requirements-analyst/atlas-admin-requirements-analysis-2026-08-20.md`（v1.1 确认基线） | ✅ |
| 架构门（Role 2） | — | ⏭️ 用户选择跳过 |
| Role 3 详细设计 | `documents/detailed-design/atlas-admin-detailed-design-2026-08-20.md`（16 章） | ✅ |
| Role 4 实现 | 新仓库 `atlas-admin`，M0~M6 六个里程碑 + 2 个评审修复 commit | ✅ |
| Role 6 QA 测试用例 | — | ⏭️ 判定非企业严格模式，默认跳过 |
| 代码评审（Close gate） | Standards + Spec 双轴并行评审 | ✅ 2 个明确缺陷已修复 |

## 3. 关键决策记录（grill 轮次共识）

### 3.1 定位与技术选型

- **独立 git 仓库**（Round 7 Q6:B），不放入 grok-build 的 `services/` 下。
- Go 1.24 + 服务端渲染 HTML（无前端框架），依赖与 atlas-server 对齐：chi/v5、golang-jwt/jwt/v5、go-toml/v2、x/crypto（bcrypt，`DefaultCost`）、google/uuid、go-sql-driver/mysql。
- **同一 MySQL 实例、同一 `atlas` 库，直连**；atlas-server 既有表与 SQL 查询不动（Round 3 Q6:C：“目前的表结构不变，atlas-server 仍然通过 sql 查询模型权限”）。
- 迁移采用**手工幂等 SQL**（`scripts/migrate.sql`，information_schema + PREPARE 判列存在性），配 `rollback.sql` 回滚脚本与 `bootstrap-admin.sql` 引导脚本（Round 8 Q3:B）。

### 3.2 认证与授权

- bcrypt 校验 atlas-server `users` 表 `password_hash`；JWT 单 token 24h（含 jti），HttpOnly Cookie（SameSite=Lax），`admin_token_blacklist` 撤销。
- **无角色用户拒绝登录**；一期不做登录防爆破（Q6 决议）。
- 固定 5 角色（admin / line_lead / developer / tester / ba），用户全局唯一角色；条线负责关系独立映射表 `admin_line_leads`（一人可负责多条线）。
- **末位管理员保护**：不能摘除最后一个 admin 角色、不能删除自己或最后一个管理员账号（Q4 决议）。

### 3.3 组织树

- 复用 `user_groups` 表，**加列式改造**：新增可空 `parent_id` + `node_type`（dept/line/squad）——ADR-0005；回滚即删两列。
- 成员与模型分配**只挂在 squad（条线分组）层**，不继承；支持多部门根（Q8 决议）；存量旧行 `parent_id=NULL` 显示为“待归类”。
- 用户可属于**多个 squad**（Round 3 Q4:C，覆盖早期一对一建议）。
- 删除保护：有子节点 / 有成员 / 有分配 / 有负责人映射的节点禁止删除。
- 移动节点用**表单选父节点**方式（Q3 决议），不做拖拽。

### 3.4 数据与报表

- 任务报表保留（Q1）：列表 + 筛选 + **详情**（含 prompt/报错等敏感字段，FR-030）+ **稽核统计视图**（FR-031，按用户/条线/日期；跨条线用户在每个条线各计一次；“未归属”单列一行）。
- 条线过滤**实时**取自上报人当前 squad 归属，不做快照。
- 删除用户的报表保留（`user_id` 可空、无外键）。
- tester / ba 可看全部报表（Q9 决议）；developer 只读自己的（Q2 决议）。
- 用户 ID 生成：邮箱前缀，冲突加 `-2`/`-3` 后缀；创建用户必须设初始密码（≥8 位，无默认密码）。

### 3.5 明确范围外（out-of-scope）

登录防爆破、拖拽式组织树编辑、条线报表快照、审批流、消息通知等（详见 `docs/atlas-admin-redesign.md` 完整清单）。

## 4. 交付物清单

### 4.1 grok-build 仓库（本工作区）

| 文件 | 内容 |
|------|------|
| `services/atlas-server/CONTEXT.md` | 术语表：新增 **Org Node** 条目；**User Group** 重定义为 squad 层叶子；Group Assignment / Membership 标注 squad-only、不继承 |
| `docs/adr/0005-user-groups-org-tree.md` | 组织树并入 `user_groups` 的加列式改造决策与回滚方案 |
| `docs/atlas-admin-redesign.md` | 8 轮 grill 全量共识 |
| `documents/requirements-analyst/atlas-admin-requirements-analysis-2026-08-20.md` | 31 条 FR + 9 条 NFR + 权限矩阵 + §12.4 九项决议（v1.1 确认基线） |
| `documents/detailed-design/atlas-admin-detailed-design-2026-08-20.md` | 完整 DDL、迁移/回滚 SQL、7 个模块设计、35 条路由表、6 个页面设计、FR/NFR 追溯矩阵、M0~M6 里程碑、§14 假设 A-1~A-12、§15 澄清 C-1~C-4（2026-08-21 全部确认） |
| `documents/session-notes/atlas-admin-sdd-session-2026-08-21.md` | 本文档（会话记录） |

**atlas-server 代码零改动**（仅 CONTEXT.md 术语表文档更新）。

### 4.2 新独立仓库 `E:\work\mygit\architechure\atlas-admin`

- 目录：`cmd/atlas-admin`、`internal/{api,auth,authz,config,server,service,store,web}`、`scripts/{migrate,rollback,bootstrap-admin}.sql`、`atlas-admin.toml.example`、`README.md`
- 里程碑提交：`8e028cb` M0+M1（骨架/配置/迁移/认证/授权）→ `1663c74` M2 组织树 → `288da08` M3 用户与角色 → `e27fae2` M4 模型目录/分配 → `86a0aab` M5 任务报表 → `e2a8116` M6 加固
- 评审修复提交：`8fb6de4`（gitignore 锚定 + 补提交入口 main.go）、`815f2a8`（条线负责人整组设置成员缺陷修复）
- 测试：7 个测试包、约 126 用例，`go build` / `go vet` / `go test -count=1 ./...` 全绿

## 5. 代码评审结果与修复（Close gate）

双轴并行评审（Standards：仓库规范符合度；Spec：与需求/设计符合度）。**3 个明确缺陷，已全部修复提交**：

| # | 轴 | 缺陷 | 位置 | 修复 |
|---|----|------|------|------|
| 1 | Standards | 构造了带 `ReadHeaderTimeout` 的 `httpSrv`，却调用包级 `http.ListenAndServe`，超时配置被丢弃 | `cmd/atlas-admin/main.go` | 改为 `httpSrv.ListenAndServe()` |
| 2 | Spec | 条线负责人走 `PUT /org/nodes/{id}/members` 整体设置成员时，逐个候选用户要求 `IsUserInLines`（已在其条线内），导致**无法向本条线新增成员**，违反 FR-010/FR-017，且与 `POST /users/{id}/memberships` 行为不一致 | `internal/service/membersvc.go` | 删除逐用户前置拒绝；范围校验仅保留 `checkSquad`（squad 属于本人条线）；测试改为断言“条线外用户可被添加” |
| 3 | 修复过程中新发现 | `.gitignore` 无斜杠模式 `atlas-admin`（本意匹配编译产物）同时匹配 `cmd/atlas-admin/` 目录，**入口 main.go 从未被提交** | `.gitignore` | 模式锚定根目录（`/atlas-admin`、`/atlas-admin.exe`），补提交 `cmd/atlas-admin/main.go` |

### 遗留优化项（判定为风格/一致性判断题，未改，留待迭代）

- 重复小工具函数：`containsID`/`contains`、`User`/`UserRow`、squad+line 校验散落 3 处、`CanSeeUser` 两份、`itoa`
- 服务层重复的 scope 分支切换
- 列表查询参数“数据团”（data clumps）
- 导航项 `navAllowed` 硬编码，未走权限矩阵
- `usersmgmt.go` 中 `GROUP_CONCAT` 结果解析
- 用户列表缺“条线”筛选维度
- 建用户时重名内联预检（应下沉 repo 层）
- `/api/v1/__ping` 属设计范围外新增（scope creep）
- `go.mod` 声明 go 1.24，设计文档写 1.25

## 6. 验证情况

- ✅ 构建/vet/测试全绿（注意：本机 `go env GOOS=linux` 为交叉编译默认值，跑测试需 `$env:GOOS="windows"`，非代码问题）
- ✅ grok-build 侧 `services/atlas-server` 代码零改动确认
- ✅ atlas-admin 工作区干净、8 commits
- ⚠️ **未做浏览器端到端走查**：管理端 UI 需真实 MySQL 并执行 `scripts/migrate.sql` + `bootstrap-admin.sql` 后方可运行（README 已写明集成步骤），当前环境不具备。如需可用测试库人工走查一遍。

## 7. 待办（需用户操作 / 可选）

1. 为 `atlas-admin` 创建远端 git 仓库，提供地址后补 remote 并推送。
2. （可选）企业级测试用例文档（Role 6 QA）——当前判定非严格模式已跳过，需要可再生成。
3. （可选）真实库环境下的浏览器端到端验证。
4. （迭代）第 5 节遗留优化项。

## 8. 用户交互时间线（决议速查）

| 时点 | 用户输入 | 效果 |
|------|----------|------|
| 初始 | 原始需求（见 §1） | 启动 SDD 流程 |
| grill 启动 | “全部推荐，后面的提问使用中文” | 后续提问改中文、缺省选项走推荐 |
| Round 3 | “Q6:C 目前的表结构不变，atlas-server 仍然通过 sql 查询模型权限” | admin 侧新表管理、存量表不动 |
| Round 3 | Q4:C | 用户可属多个 squad |
| Round 4 | Q7: bcrypt `DefaultCost` | 密码哈希选型定案 |
| Round 7 | Q6:B | 独立 git 仓库 |
| Round 8 | Q3:B | 手工 SQL 迁移 |
| Role 1 澄清 | Q1-Q5:推荐 Q6:A Q7:B+稽核统计视图 Q8:A Q9:A | FR-030/031 入档、多根、tester/ba 全报表等 |
| 架构门 | “跳过” | 不产出架构文档，直接进详细设计 |
| 设计澄清 | “全推荐” | C-1~C-4 确认，进入 Role 4 实现 |
