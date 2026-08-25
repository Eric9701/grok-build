# Atlas Server Access

Enterprise atlas-server domain for accounts, User Groups, and which managed models a user may see.

## Language

**Org Node**:
A tree node in the `user_groups` table, typed by `node_type`: 业务部 (dept), 业务条线 (business line), or 条线分组 (squad). Only squads hold members and model assignments. Legacy rows with `parent_id = NULL` are unclassified nodes awaiting placement.
_Avoid_: Department unit, OU, flat group

**User Group**:
A squad-level Org Node — the leaf layer. The only Org Node level that holds Group Membership and Group Assignment. Formerly flat; nesting exists only above this level.
_Avoid_: Team, Org, Model Entitlement Group, business line (that is a non-leaf Org Node)

**Direct Assignment**:
Models attached to a single user via `user_models`. May mark one model as that user's default.
_Avoid_: Personal entitlement (unless contrasting with group), override

**Group Assignment**:
Models attached to a User Group (squad-level Org Node only). Contributes to availability only; does not set a user's default model. No inheritance across Org Node levels.
_Avoid_: Group default, inherited default, line-level grant

**Effective Model Set**:
The deduplicated union of a user's Direct Assignment and all Group Assignments from User Groups they belong to (enabled catalog entries only).
_Avoid_: Allowlist (ambiguous with upstream fallback), entitlements bundle

**Managed Catalog Mode**:
When the Effective Model Set is non-empty, `/v1/models` returns that managed set (encrypted). When empty, the server falls back to probe / upstream / builtin — same as today's "no user_models" behavior.
_Avoid_: Open mode, unrestricted (implies no gates at all)

**At-Rest ENC**:
The `ENC(...)` form of a managed model's routing name (`model`) and `api_key` on client disk — both the Managed Config Segment and the models cache (`api_key` + `info.model`). Catalog id stays the map/section key in plaintext.
_Avoid_: Whole-file cache encryption; treating the models cache as a plaintext dump of memory

**Managed Config Segment**:
A client `config.toml` `[model.<catalog-id>]` table with `managed = true`. Routing name and `api_key` stay in At-Rest ENC; catalog id is the section name.
_Avoid_: User-authored `[model.*]` without `managed` (not overwritten by catalog sync)

**Group Membership**:
The many-to-many link between a user and a squad-level Org Node. A user may belong to many squads; membership is identity-only (no per-membership model list).
_Avoid_: Role in group, group seat

**Catalog ID**:
The picker / `config.toml` key of a catalog entry (`[model.<id>]`). Task Report field `model` stores this.
_Avoid_: Routing Name, model slug, `info.model`

**Routing Name**:
The upstream sampling slug (`model.model` / `info.model`). Task Report field `modelRouting` stores this in plaintext.
_Avoid_: Catalog ID, ENC form of the slug, `id`

**Task Report**:
A per-task usage record the client posts after a main-session turn or subagent finishes. Attributed by Report User and Client Version carried in the report body. Identifies the model by Catalog ID (`model`) and Routing Name (`modelRouting`).
_Avoid_: Trace, session signal, telemetry event

**Report User**:
The user identity on a Task Report: `userId` and `email` from the report body. Not derived from the access token.
_Avoid_: JWT subject, Bearer user, x-userid

**Client Version**:
The compiled CLI version of the process that posted the Task Report (`GROK_VERSION` / `--version`).
_Avoid_: Protocol version, agent version, model version

**Seed Account**:
Admin 用显示名 + 邮箱 + 机器码即可建户。UserId 取邮箱 `@` 前前缀；默认密码为约定种子 `atlas123`。
_Avoid_: 手工指定 userId 作为必填；把「只有机器码」当成开户

**OAuth Refresh Token**:
CLI device login 签发的 refresh，落在 MySQL `refresh_tokens`。Device code 仍只在进程内存（15 分钟登录窗）。重启 atlas-server 不得丢掉已登录 CLI 的静默续期。
_Avoid_: 把 refresh 当成 MemoryStore 即可；用「access 过期」解释 Task Report 变 `anonymous`（那是 refresh 失败后 CLI 清了 `auth.json`）
