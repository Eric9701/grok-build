# Atlas Relay Demo

独立的 **出站 WebSocket 中继 + 聊天页**，用来验证一台或多台 `atlas agent headless` 与浏览器对话。

**不改 atlas-server，不改 CLI。** 只做 ACP 透传：CLI 当 Relay Agent，本页当 Relay Client。术语见 [CONTEXT.md](./CONTEXT.md)。

```
浏览器  http://127.0.0.1:2420/build?agent=laptop-a
   │  /ws/client   +  atlas.relay/bind
   ▼
relay-demo  （按 Agent Identity 分槽）
   ▲  /ws?agent_id=laptop-a
   │
atlas agent headless   ← 本机出站，工具仍在本机执行
```

## 启动

需要本机已安装 Go，且 CLI 已登录（`atlas login --device-auth`）。

终端 1：

```powershell
cd services/atlas-relay-demo
go run .
```

终端 2（`--grok-ws-origin` 必须与浏览器地址同源）。**建议带上 `agent_id`**，否则回退 `x-userid`，再没有则是 `anonymous-<序号>`：

```powershell
atlas agent --always-approve headless `
  --grok-ws-url "ws://127.0.0.1:2420/ws?agent_id=laptop-a" `
  --grok-ws-origin http://127.0.0.1:2420
```

再开一台机器或第二个进程时换一个 id：

```powershell
atlas agent --always-approve headless `
  --grok-ws-url "ws://127.0.0.1:2420/ws?agent_id=laptop-b" `
  --grok-ws-origin http://127.0.0.1:2420
```

浏览器打开 <http://127.0.0.1:2420/build>（或 `/build?agent=laptop-a`）。

1. 只有一个 Agent 在线时自动绑上；多个时用顶栏下拉选择  
2. 填写 **该 Agent 机器上的绝对路径** 作为 cwd（按身份分别记住）  
3. 圆点变绿、状态为「就绪」后发消息，或用「云端派工」三轮执行  

## 云端派工（URL → Role 4 → 回执）

做法说明见 [docs/云端派工推荐做法.md](./docs/云端派工推荐做法.md)。

聊天页上方填写 **jobId** 和三份文档的 **http(s) URL**，点「开始派工」。页面会连续发三轮 `session/prompt`：

1. **落盘**：Agent 下载 URL，写入  
   `documents/requirements-analyst/`、`documents/detailed-design/`、`documents/test-cases/`  
2. **实现**：拉起 `atlas-sdd:4-software-engineer-agent`（不要 Role 1/2/3，不要整段 `/implement`）  
3. **回执**：写出 `documents/execution-report-<jobId>.json`，页面解析 JSON  

本机试跑可点「填入示例 URL」，对应：

- http://127.0.0.1:2420/sample-docs/requirements.md  
- http://127.0.0.1:2420/sample-docs/design.md  
- http://127.0.0.1:2420/sample-docs/acceptance.md  

Agent 机器必须能访问这些 URL（示例挂在本 demo 上时，CLI 与浏览器要同机或能路由到 2420）。cwd 必须是**业务仓根**，且已装 `atlas-sdd` 插件，否则 Role 4 拉不起来。

`POST /dispatch/prompts` 只生成三轮文案，不替 Agent 拉网。

默认监听 `127.0.0.1:2420`。换端口：`go run . -addr 127.0.0.1:9000`，两边 URL 一起改。

## 行为

| 路径 | 谁连 |
|---|---|
| `/build` 或 `/` | 聊天页 |
| `/ws`、`/ws/agent` | CLI headless（用 `?agent_id=` 登记身份） |
| `/ws/client` | 聊天页 |
| `/status`、`/healthz` | 探活；`/status` 列出在线 Agent |
| `/dispatch/prompts` | POST，生成三轮派工 prompt |
| `/sample-docs/…` | 示例需求 / 设计 / 验收 |

- **Agent Identity**：`agent_id` → `x-userid` → `anonymous-<序号>`  
- 同一身份再连：**新连接顶掉旧连接**  
- 聊天页同时只绑一个 Agent；`atlas.relay/bind` 换人，URL `?agent=` 作初始选中  
- 每个 Agent 只留一个浏览器；后来者抢走 Bind（先到的页面不断线，但要重新选择）  
- 当前 Agent 掉线：**保持 Bind**，同身份重连后再建 ACP 会话  
- 对话记录按 Agent 分栏存在本页（刷新丢）  
- 权限请求在页面里自动 Allow once（仍建议 CLI 加 `--always-approve`）  
- Demo **不校验 JWT**，只打日志。不要暴露到公网  

## 若 CLI 立刻退出

文案是 `Headless mode requires a grok.com session`：当前登录不是第一方会话（issuer 对不上），relay 门控不会连。先 `atlas login --device-auth`，确认 `auth.json` 里 `oidc_issuer` 与编译进 CLI 的 issuer 一致。

连的是 `wss://code.grok.com/...` 而不是本 demo：漏了 `--grok-ws-url`。

## 和 atlas-server 的关系

本进程与 atlas-server 无关。模型鉴权、device login 仍走你现有的 atlas-server；这里只把网页和本机 agent 接在一起。
