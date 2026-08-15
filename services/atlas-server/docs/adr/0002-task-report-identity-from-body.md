# Task Report 身份与版本从报文体读取

`POST /v1/task-reports` 的 **Report User**（`userId` + `email`）和 **Client Version** 只来自 JSON 正文，不解析 JWT、也不用 `x-userid` 头。缺用户时记为 `anonymous`，上报仍是尽力而为。企业侧 JWT 与客户端会话经常对不上（托管模型、仅 API Key、代理剥 token），从报文取身份比从 Bearer 更稳；这是有意信任客户端字段的内部遥测，不是鉴权。
