# Task Report 取数口径

权威实现：`services/atlas-server/internal/telemetry/taskreport.go`、`internal/store/task_report.go`。Admin 页：`/atlas/admin/task-reports`。人员评定如何用这些字段见 [rubric.md](rubric.md)。

## 一条报告是什么

CLI 在 **子代理任务结束** 时 `POST /atlas/v1/task-reports`。`userId` / `email` / `clientVersion` **只来自 JSON 正文**（ADR 0002），不是 JWT。缺用户记为 `anonymous`。`team_id` 来自请求头 `x-teamid`。

`status`：`completed` | `cancelled` | `error`。汇总里失败 = `success=0` 且 `status <> 'cancelled'`。

`model` 是 Catalog ID；`modelRouting` 是明文 Routing Name。Admin 排行按 Catalog ID。

## Admin API

默认 origin：`http://10.218.220.237:22255/`。规范化后的 API 根：`http://10.218.220.237:22255/atlas`。完整路径：`{API根}/admin/api/task-reports`。`--base` 或环境变量 `ATLAS_BASE` 可覆盖 origin；无 path 时自动补 `/atlas`。

| 调用 | 查询 | 返回 |
|---|---|---|
| 整体 | `from` `to`（可选 `limit` 限制用户条数，默认 50、上限 500） | `{from,to,summary,agents,models,users}` |
| 个人汇总 | `user_id`（或 `email`）+ `aggregate=1` + `from` `to` | `{from,to,userId,agents,models}` |
| 个人明细 | `user_id` + `limit` + `from` `to` | `{from,to,userId,count,reports}` |

`from`/`to` 为闭区间 `YYYY-MM-DD`，按服务端本地 `DATE(created_at)` 过滤。两者都省略 = **当天**。`from=all` / `to=all` / `date=all` = 不限日期。旧参 `date=` 等于 `from=to`。

整体 `summary`：`totalTasks` `successCount` `failedCount` `cancelledCount` `totalArtifacts` `totalTokens` `uniqueUsers` `uniqueModels`。

`users[]`：`userId` `email` `count` `successCount` `artifactCount` `tokensUsed`。

`agents[]`：`subagentType` `count` `artifactCount` `tokensUsed`。

`models[]`：`model` `count` `artifactCount` `tokensUsed`。空 model 显示为 `(unknown)`。

明细常用字段：`description` `subagentType` `model` `modelRouting` `status` `success` `durationMs` `toolCalls` `turns` `tokensUsed` `artifacts[]` `{path,kind}` `artifactCount` `cwd` `worktreePath` `startedAt` `completedAt` `clientVersion` `createdAt`。`kind` ∈ `code` | `doc` | `other`。

个人明细默认 `limit=50`，合法范围 1–500。定量用 aggregate，定性用 description 样本。

当前 Admin API 无登录中间件（内网页）。若返回 401/403，向用户要可用的 Cookie 或内网入口，不要猜口令。

## 脚本

`scripts/fetch_reports.py`（标准库）。`--all-users` 会先拉整体，再对 `users` 前 `--max-users`（默认 15）人各拉 aggregate + 明细。默认删除每条 `prompt`/`error`。

## MySQL 回退

表 `task_reports`。DSN：`ATLAS_MYSQL_DSN` 或 `[mysql].dsn`。日期谓词必须用 `DATE(created_at)` + 日字符串，不要把 `time.Time` 当 UTC 塞进去。

整体（把 `:from` `:to` 换成日，或去掉两行日期条件）：

```sql
SELECT COUNT(*) AS total_tasks,
       IFNULL(SUM(success), 0) AS success_count,
       IFNULL(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0) AS cancelled_count,
       IFNULL(SUM(CASE WHEN success = 0 AND status <> 'cancelled' THEN 1 ELSE 0 END), 0) AS failed_count,
       IFNULL(SUM(artifact_count), 0) AS total_artifacts,
       IFNULL(SUM(tokens_used), 0) AS total_tokens,
       COUNT(DISTINCT user_id) AS unique_users,
       COUNT(DISTINCT NULLIF(TRIM(model), '')) AS unique_models
FROM task_reports
WHERE DATE(created_at) >= :from AND DATE(created_at) <= :to;
```

按用户：

```sql
SELECT IFNULL(user_id, '') AS user_id,
       IFNULL(MAX(NULLIF(email, '')), '') AS email,
       COUNT(*) AS count,
       IFNULL(SUM(success), 0) AS success_count,
       IFNULL(SUM(artifact_count), 0) AS artifact_count,
       IFNULL(SUM(tokens_used), 0) AS tokens_used
FROM task_reports
WHERE DATE(created_at) >= :from AND DATE(created_at) <= :to
GROUP BY user_id
ORDER BY count DESC
LIMIT 50;
```

单人 agent / model：`WHERE user_id = ?` 后 `GROUP BY subagent_type` 或 `GROUP BY IFNULL(NULLIF(TRIM(model), ''), '(unknown)')`。

## 易错

- 不带日期的 API = 当天，不是全量。
- 一条报告 ≠ 一次用户发送。父会话多子代理会放大任务数。
- Catalog ID 与 Routing Name 不要混着排行。
- `user_id` 无外键；删用户后报告仍在，字符串照旧展示。
- 上报可被 `GROK_DISABLE_TASK_REPORT=1` 关掉；没报告不等于没工作。
