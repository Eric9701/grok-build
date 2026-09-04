# Atlas Relay Demo

独立出站中继：浏览器聊天页与一台或多台本机 `atlas agent headless` 对话。不改 atlas-server，也不改 CLI。

## Language

**Relay Agent**:
出站连 `/ws`（或 `/ws/agent`）的 `atlas agent headless` 进程。
_Avoid_: 客户端、Client（口语里常把「那台机器」叫客户端，但本上下文里 Client 是浏览器）

**Relay Client**:
连 `/ws/client` 的浏览器聊天页，扮演 ACP Client。
_Avoid_: 把聊天页叫 Agent；用「客户端」同时指 CLI 和浏览器

**Agent Identity**:
一个 Relay Agent 的稳定登记名。优先取握手 URL 的 `agent_id`；缺省用 `x-userid`；再缺省为 `anonymous-<短号>`。
_Avoid_: 用 JWT 当身份（本 demo 不验 JWT）；用连接序号当长期身份

**Agent Replacement**:
同一 Agent Identity 再次连上时，新连接顶掉旧连接。
_Avoid_: 同 id 并存多条活连接

**Bound Agent**:
一个 Relay Client 当前唯一对接的那个 Relay Agent。同时只绑一个；切换即改 Bound Agent。
_Avoid_: 一页同时绑多个 Agent

**Bind**:
Relay Client 选定一个 Agent Identity 并与之对接。页面 URL 的 `agent` 可作为初始 Bind。
_Avoid_: 靠重连 WebSocket 来换人

**Client Replacement**:
同一个 Bound Agent 上只留一个 Relay Client，后来者顶掉前者。
_Avoid_: 多浏览器同时对本 Agent 发 prompt

**Agent Transcript**:
聊天页按 Agent Identity 分开保存的对话记录（仅本页内存）。切换不清屏；刷新丢。
_Avoid_: 多 Agent 混在一条时间线；切换就清空

**Agent Cwd**:
某个 Agent Identity 上次使用的工作目录，按身份记在浏览器里。
_Avoid_: 全页共用一个 cwd

**Dispatch Job**:
一次云端派工：三份文档的 HTTP(S) URL + jobId，目标是指定 Bound Agent 的仓库 cwd。
_Avoid_: 把文档正文塞进 prompt；用 ACP 文件 RPC 下发

**Documents Drop**:
派工第一轮：Agent 按 URL 拉取并写入 Documents Contract（需求 / 设计 / 验收）。
_Avoid_: 这一轮就开始写业务代码

**Role 4 Run**:
派工第二轮：在文档已在盘上之后，只拉起 `atlas-sdd:4-software-engineer-agent` 做实现。
_Avoid_: 再跑 Role 1/2/3；用完整 `/implement` 代替 Role 4

**Execution Report**:
派工第三轮写出的机器可读回执（`documents/execution-report-<jobId>.json`），是完成与否的依据。
_Avoid_: 把 Task Report 当验收回执
