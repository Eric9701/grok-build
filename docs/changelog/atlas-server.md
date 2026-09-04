# atlas-server 变更记录

`services/atlas-server`：Go 企业代理。登录、ListModels、settings、遥测、Task Report、托管模型。基址本环境 `http://10.218.220.237:22255/atlas`。记法见 [README](./README.md)。

## 2026-09

### 2026-09-02 — 建立本变更记录

- **状态**：文档
- **会话**：[整理变更记录](54b536e1-6151-41a6-bccc-39bad21556c3)

## 2026-08

### 2026-08-31 — 刷新 xAI probe bundle

- **状态**：已落地
- **会话**：[创建 skill 路径](954e513d-9243-4b31-9bd9-95f584df3127)
- `scripts/xai_login.py`：官方 Device Login 拿 token，写 `scripts/auth.json`。**不要**拿 `~/.atlas/auth.json` 打 `cli-chat-proxy.grok.com`。
- `probe_xai_proxy.py` 继续拉 probe；`pack_bundle_archive.py` 把 `probe_bundle_archive/bundle` 打回 gzip。
- bundle 内 `.grok` 文案替换为 `.atlas`（与 CLI 创建 skill 路径对齐）。

### 2026-08-24 — refresh_tokens 落 MySQL

- **状态**：已落地
- **会话**：[Token 续期](2f91e8e7-8182-4f37-a5d8-d69e2686c5a2)
- Device code 仍内存（15 分钟登录窗）。refresh 落库，重启不丢已登录 CLI。
- `invalid_grant` 会导致 CLI 清 `auth.json`，Task Report 变 `anonymous`。

### 2026-08-24 — Task Report agent 去掉 grok- 前缀

- **状态**：已落地
- **会话**：[Agent 展示](3750f595-6a0e-43e3-8f7d-41f1be702957)
- Admin 页：`grok-build-plan` 显示 `build-plan`；库内仍存原文。悬停看原值。只改 `web`。

### 2026-08-20 — User Groups 组织树

- **状态**：已落地（表结构约定；迁移脚本在 atlas-admin）
- **ADR**：[0005](../adr/0005-user-groups-org-tree.md)
- `user_groups` 加 `parent_id` / `node_type`（业务部 / 条线 / 条线分组）。只有分组挂成员和模型。无跨级继承。

### 2026-08-15 前后 — 开户、排行、Catalog 字段

- **状态**：已落地
- **会话**：[能力与运维](73fbb1a7-df21-4ab0-9c9f-e5d058a2840b)
- Seed Account：用户名 + 邮箱 + 机器码；UserId = 邮箱 `@` 前前缀；默认密码 `atlas123`。
- Task Report 明细可弹详情；按人排行可按 token / 任务数。只改 `web`。
- 报文体增加 Routing Name（`modelRouting`）；排行按 Catalog ID。
- 改用户模型分配后 **不推送** 客户端；对方要 `atlas models refresh` / `/refresh-model`，或等预取 / 鉴权 / ETag / 缓存过期。

### 2026-08-12 前后 — 遥测门控与 ENC

- **状态**：已落地
- **会话**：[能力与运维](73fbb1a7-df21-4ab0-9c9f-e5d058a2840b)、[主线建设](7b869ee4-cf94-48f3-9801-068610925442)
- `trace_upload_enabled=false` **只关** trace 产物上传。signals / events 走 `telemetry_enabled`。Task Report 默认开，仅 `GROK_DISABLE_TASK_REPORT=1` 可关。
- ListModels：有用户分配 → 只返回托管条目；`id` / `model` / `api_key` 下发前 `ENC(...)`。DB 仍存明文 id/model。
- ADR：Managed Catalog Mode 回退；models cache At-Rest ENC；Task Report 身份从报文体读。

### 2026-08-01 → 08-12 — 托管模型与 Admin

- **状态**：已落地
- **会话**：[主线建设](7b869ee4-cf94-48f3-9801-068610925442)
- 表 `managed_models` / `user_models`（collation 对齐 `users.user_id`，防 FK 1215）。
- Admin：`/atlas/admin/models`，分配到人；同一模型可按组复用（cmsgroup-kimi2.7 vs imsgroup-kimi2.7）。
- 明文入库自动 `ENC(...)`。密钥默认 `atlas-managed-model-secret-v1`，`ATLAS_MODEL_SECRET_KEY` 可覆盖。
- settings 从 `download/probe_settings.json` 出 `GET /atlas/v1/settings`。文档 `docs/settings.md`。
- Linux 构建：`scripts/build-linux.sh`。`CGO_ENABLED=0`。
- Admin task-reports 默认进 `/atlas/admin/task-reports`（注意前缀，裸 `/admin/...` 会 404）。

## 2026-07

### 2026-07-22 → 07-29 — 从 mock 登录到 Task Report

- **状态**：已落地
- **会话**：[企业化起点](ba632f70-aedc-4426-b76e-b57346e56fc7)
- Go 服务模拟 xAI Device Login；用户名 + 机器码；用户落 MySQL。
- probe 默认从仓库 `download/` 读（后续脚本目录演进为 `scripts/download-new`）。
- Task Report 入库 + Admin 页初版。未登录用户记 `anonymous`。
- 遥测：`/events`、`/traces`、`/sessions/{id}/signals`；trace 默认关的讨论在此开始。
- 默认上报/OAuth 基址落到企业 `…:22255/atlas`。
