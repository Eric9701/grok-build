# Atlas Remote Settings（`GET /atlas/v1/settings`）

CLI 启动时通过 `cli_chat_proxy_base_url` 拉取远程 settings，反序列化为 `RemoteSettings`
（定义见 `crates/codegen/xai-grok-config-types`）。

## 数据来源（atlas-server）

| 优先级 | 来源 |
|--------|------|
| 1 | `download/probe_settings.json` |
| 2 | 上游 `GET /settings`（`[upstream] enabled=true` 时）并写回 probe |
| 3 | 内置兜底 JSON（`allow_access=true` 等） |

atlas-server 返回前会强制：

- `allow_access = true`（避免本地被 Free 门禁拦住）
- 若缺少 `default_model`，补成 `"grok-4.5"`

路径：`GET /atlas/v1/settings`  
实现：[`internal/settings/handler.go`](../internal/settings/handler.go)

## 优先级（CLI）

一般规则：

**CLI 参数 / 环境变量 / 本地 `config.toml` > remote settings > 内置默认**

个别字段另有说明（见下表）。

---

## 一、准入与订阅

| 字段 | 类型 | 作用 | CLI 是否消费 |
|------|------|------|----------------|
| `allow_access` | bool | 产品门禁；仅 `true` 放行进主界面 | ✅ |
| `gate_message` | string | 拦截页文案 | ✅ |
| `gate_url` | string | 拦截页 CTA 链接 | ✅ |
| `gate_label` | string | 拦截页按钮文案 | ✅ |
| `subscription_tier_display` | string | 档位展示名（如 `"Free"`），影响 UI/计费提示 | ✅ |
| `on_demand_enabled` | bool | 是否允许按需积分；`false` 禁止改 on-demand 上限 | ✅ |
| `usage_billing_redirect_url` | string | `/usage` 外链；空则走后端 | ✅ |
| `subscription_watch_interval_secs` | number | Free→付费轮询间隔（秒）；`0` 关闭 | ✅ |
| `zdr_access_enabled` | bool | ZDR 用户是否放行；默认偏拦 | ✅ |
| `privacy_notice_rollout` | — | — | ❌ 不进结构体 |

---

## 二、模型与会话

| 字段 | 类型 | 作用 | CLI 是否消费 |
|------|------|------|----------------|
| `default_model` | string | 新会话推荐默认模型 | ✅ |
| `system_prompt_label` | string | 系统提示身份标签 | ✅ |
| `file_toolset` | string | 文件工具集：`standard` / `hashline` | ✅ |
| `inference_idle_timeout_secs` | number | 推理流空闲超时（秒） | ✅ |
| `ab_turn_timeout_secs` | number | — | ❌ 未读取 |
| `auto_compact_threshold_percent` | number | 上下文占用到该 % 触发自动压缩 | ✅ |
| `compaction_mode` | string | `summary` / `transcript` / `segments` | ✅ |
| `two_pass_compaction_enabled` | bool | 双通道预压缩 | ✅ |
| `worktree_type` | string | worktree 创建策略（如 `standalone`） | ✅ |
| `restore_code` | bool | resume 时是否恢复代码改动 | ✅ |
| `show_resolved_model` | bool | `/session-info` 是否显示解析后模型 | ✅ |
| `leader_mode` | bool | 是否建议开启 leader 模式 | ✅ |
| `subagent_worktree_snapshot_enabled` | bool | 子代理 worktree 是否快照后清理 | ✅ |

---

## 三、功能开关（工具 / UI）

| 字段 | 类型 | 作用 | CLI 是否消费 |
|------|------|------|----------------|
| `web_fetch_enabled` | bool | 是否注册 `web_fetch` | ✅ |
| `web_fetch_proxy` | string | fetch 代理 | ✅（本地优先） |
| `web_fetch_allowed_domains` | array | 域名白名单 | ✅（本地优先） |
| `ask_user_question_enabled` | bool | 「向用户提问」工具；默认偏开 | ✅ |
| `path_not_found_hints` | bool | 路径不存在时增强提示 | ✅ |
| `contextual_hints` | object | 分 tip 上下文提示 | ✅ |
| `show_thinking_blocks` | bool | 是否展示思考块 | ✅ |
| `group_tool_verbs` | bool | 折叠连续只读工具行 | ✅ |
| `collapsed_edit_blocks` | bool | Edit 折叠为 +N/-M | ✅ |
| `voice_mode_enabled` | bool | 语音输入 | ✅ |
| `sharing_enabled` | bool | 会话分享 | ✅ |
| `plugin_cta` | bool | 插件安装 CTA | ✅ |
| `official_marketplace_auto_register` | bool | 首启自动注册官方 marketplace | ✅ |
| `image_description_model` | string | 图片描述辅模型 | ✅ |
| `auto_background_on_timeout` | bool | bash 超时转后台而非杀掉 | ✅ |
| `allow_background_operator` | bool | 是否允许命令中 `&` | ✅ |
| `workspace_command_enabled` | bool | 仅 `true` 启用 `workspace` 子命令 | ✅ |
| `image_gen_enabled` | bool | 图片生成（文档意图） | ⚠️ 进结构体，解析多半只看 env/本地 |
| `video_gen_enabled` | bool | 视频生成（文档意图） | ❌ 基本未读 |
| `auto_permission_mode_enabled` | bool | 扁平键 | ❌；实际用 `auto_mode` 对象 |
| `self_verification_mode_enabled` | — | — | ❌ |
| `strip_competitor_branding` | — | — | ❌ |
| `proactivity_reminder_cadence` | — | — | ❌ |

---

## 四、Goal 模式

| 字段 | 类型 | 作用 | CLI 是否消费 |
|------|------|------|----------------|
| `goal_enabled` | bool | `/goal` 总开关；`false` 强制关 | ✅ |
| `goal_classifier_enabled` | bool | 完成度分类器 | ✅ |
| `goal_planner_enabled` | bool | 目标规划器 | ✅ |
| `goal_verifier_count` | number | 对抗验证 skeptics 数（约 1–5） | ✅ |
| `goal_classifier_max_runs` | number | 分类器最大次数 | ✅ |

---

## 五、Memory / Pruning / Flush / Dream

本地 `[memory]` 等 TOML 优先；`--no-memory` 最高。

| 字段组 | 作用 | CLI 是否消费 |
|--------|------|----------------|
| `memory_enabled` | 跨会话记忆总开关 | ✅ |
| `memory_search_max_results` / `memory_search_min_score` | 检索条数与最低分 | ✅ |
| `memory_initial_injection_*` | 开场注入记忆 | ✅ |
| `memory_embedding_model` / `memory_embedding_dimensions` | embedding 模型与维数 | ✅ |
| `memory_temporal_decay_*` | 时间衰减 | ✅ |
| `memory_mmr_*` | MMR 多样性 | ✅ |
| `memory_watcher_enabled` | 记忆文件监视 | ✅ |
| `pruning_*` | 压缩前裁剪历史 | ✅ |
| `flush_*` | 记忆刷盘 | ✅ |
| `dream_*` | 后台巩固记忆 | ✅ |

---

## 六、遥测 / 反馈 / 存储

| 字段 | 类型 | 作用 | CLI 是否消费 |
|------|------|------|----------------|
| `telemetry_enabled` | bool | 遥测总开关 | ✅ |
| `telemetry_mode` | string | `"session-metrics"` / `"full"` / `"off"`（优先于 bool） | ✅ |
| `trace_upload_enabled` | bool | Trace/OTLP 上传（需遥测已开） | ✅ |
| `feedback_enabled` | bool | `/feedback`；`null` 默认开 | ✅ |
| `session_registry_enabled` | bool | 会话注册/上传钩子 | ✅ |
| `loc_tracking` | bool | 代码行变更归因 | ✅ |
| `writeback_enabled` | bool | 仅 `true`→写回远程存储，否则本地 | ✅ |

---

## 七、MCP / TodoGate / Doom-loop

| 字段 | 类型 | 作用 | CLI 是否消费 |
|------|------|------|----------------|
| `mcp_startup_timeout_secs` | number | MCP 握手超时（秒） | ✅ |
| `managed_mcps_enabled` | bool | 是否拉托管 MCP | ✅ |
| `managed_mcp_gateway_tools_enabled` | bool | 托管网关工具 | ✅ |
| `todo_gate_enabled` | bool | 回合末 Todo 提醒门 | ✅ |
| `todo_gate_max_fires_per_prompt` | number | 每 prompt 最多触发次数 | ✅ |
| `doom_loop_recovery` | object | 对象型恢复策略；`null`=关 | ✅ |
| `doom_loop_enabled` 等扁平数值 | number/bool | — | ❌ 多数不进结构体 |

---

## 八、UI 文案

| 字段 | 类型 | 作用 | CLI 是否消费 |
|------|------|------|----------------|
| `tips` | string[] | 启动 tip，按日轮转一条 | ✅ |
| `announcements` | object[] | 公告条（见下） | ✅ |

### `announcements[]` 结构

```json
{
  "id": "promo-supergrok-upsell",
  "message": "...",
  "severity": "promo",
  "cta": {
    "label": "Click here to Upgrade",
    "url": "https://...",
    "caption": "or use Ctrl+O"
  },
  "dismissible": false,
  "persistent": true
}
```

| 子字段 | 作用 |
|--------|------|
| `id` | 唯一 ID，用于隐藏/去重 |
| `message` | 公告正文 |
| `severity` | 样式分类（如 `promo`） |
| `cta` | 行动按钮（label / url / caption） |
| `dismissible` | 是否可点隐藏 |
| `persistent` | 是否跨会话持续展示 |

---

## 九、OAuth（结构体有，当前基本未读）

| 字段 | 说明 |
|------|------|
| `grok_oauth_enabled` | 文档意图：远程开默认 xAI OAuth；实际多靠 CLI/env |
| `oauth2_issuer` / `oauth2_client_id` | 自定义 OAuth2；当前解析后未接线 |

---

## 十、子代理相关

| 字段 | 说明 |
|------|------|
| `subagents_enabled` | **不进有效消费路径**；子代理由本地 `[subagents]` 控制 |
| `subagents_default_model` | 不进结构体 / 未使用 |

---

## 十一、未接线 / 透传（改了通常无效果）

以下键会出现在 `probe_settings.json` 中，但 CLI 当前不消费（不进 `RemoteSettings` 或进了不读）：

- `force_update`、`min_client_version`、`release_channel`
- `max_upload_file_bytes`、`max_upload_untracked_bytes`
- `disable_codebase_upload`、`base_tree_*`、`max_supplemental_files`
- `local_repo_as_private`、`ab_safety`、`ab_turn_timeout_secs`
- 扁平 `doom_loop_enabled` / `doom_loop_*_threshold` / `doom_loop_window_size` / `doom_loop_cycle_max_length`
- `strip_competitor_branding`、`self_verification_mode_enabled`、`proactivity_reminder_cadence`
- `non_git_workspace_capture`（`non_git_warning` 会消费）

---

## 十二、本地部署建议

针对 atlas-server + 内网 CLI：

1. **必须**：`allow_access: true`（server 已强制）
2. **模型**：设好 `default_model`
3. **上报**：按需打开 `telemetry_enabled` / `trace_upload_enabled`（对齐 atlas 的 `/atlas/v1/traces`、`/atlas/v1/task-reports`）
4. **能力**：`goal_*`、`web_fetch_enabled`、`voice_mode_enabled` 等按产品需要
5. **公告**：`announcements` / `tips` 中的公网升级链接建议改成你们自己的地址或清空
6. **子代理**：不要依赖 remote 的 `subagents_enabled`，改本地 `config.toml` 的 `[subagents]`

示例最小可用片段：

```json
{
  "allow_access": true,
  "default_model": "grok-4.5",
  "subscription_tier_display": "Atlas",
  "telemetry_enabled": true,
  "trace_upload_enabled": true,
  "subagents_enabled": true,
  "goal_enabled": true,
  "web_fetch_enabled": true,
  "announcements": [],
  "tips": []
}
```

---

## 相关文件

| 路径 | 说明 |
|------|------|
| [`download/probe_settings.json`](../download/probe_settings.json) | 当前服务端返回的 settings 缓存 |
| [`internal/settings/handler.go`](../internal/settings/handler.go) | 服务端 handler |
| `crates/codegen/xai-grok-config-types/src/lib.rs` | `RemoteSettings` 定义 |
| `crates/codegen/xai-grok-shell/src/remote/client.rs` | `fetch_settings_blocking` |
