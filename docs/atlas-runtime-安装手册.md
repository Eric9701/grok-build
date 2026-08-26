# atlas-runtime 安装与使用手册

**atlas-runtime** 是企业侧 AI 编码运行时：终端里的 **Atlas CLI**（命令名 `atlas`）、编辑器里的 **Atlas 扩展**，以及从企业 marketplace 安装的技能/角色插件。它们共用 `~/.atlas`（Windows：`%USERPROFILE%\.atlas`）里的配置和登录态。


| 组件    | 你看到的名字                               | 作用                                     |
| ----- | ------------------------------------ | -------------------------------------- |
| CLI   | `atlas` / `atlas.exe`                | 对话、工具调用、自动更新、插件                        |
| 编辑器扩展 | Atlas for VS Code（包名 `atlas-vscode`） | 侧边栏聊天；通过 ACP 拉起同一套 CLI                 |
| 插件    | `atlas-skills`、`atlas-sdd`           | Main Flow 技能（`/ask-atlas` 等）与企业 SDD 角色 |


下文示例基址为 `http://10.218.220.237:22255/atlas`，请换成你司 atlas-server 的实际地址。

更完整的功能说明见 [atlas-用户指南.html](./atlas-用户指南.html)（含 ask-atlas / SDD、企业运行时、配置分层）。

---

## 1. 环境要求

- **Windows 10/11**（PowerShell 5.1+）或 macOS / Linux
- 日常使用需能访问企业 atlas-server（安装、登录、托管模型、更新都走该地址）
- **连不上云端时**仍可用本地/BYOK 模型启动，见 [§9](#9-连不上-atlas-server-时如何启动)
- 编辑器：**VS Code** 或 **Cursor**（同一份 VSIX）
- 企业安装脚本会尽量装上 `atlas-sdd` + `atlas-skills`；需能访问插件 marketplace（GitLab）

---

## 2. 安装 CLI

脚本会按本机选择产物：`windows-x86_64` / `linux-x86_64` / `linux-aarch64` / `macos-x86_64` / `macos-aarch64`。企业 atlas-server 上必须已发布对应文件，否则会提示该平台尚不可用。

### Windows

```powershell
irm http://10.218.220.237:22255/atlas/cli/install.ps1 | iex
```

企业通道：

```powershell
irm http://10.218.220.237:22255/atlas/cli/install-enterprise.ps1 | iex
```

### macOS / Linux

```bash
curl -fsSL http://10.218.220.237:22255/atlas/cli/install.sh | bash
```

企业通道：

```bash
curl -fsSL http://10.218.220.237:22255/atlas/cli/install-enterprise.sh | bash
```

新开一个终端，确认：

```bash
atlas --version          # macOS / Linux
atlas.exe --version      # Windows
```

---

## 3. 登录

企业账号走 **设备码**。先在浏览器登录 atlas-server，再让 CLI 拿令牌。

```powershell
atlas login --device-auth
```



VS Code / Cursor 侧边栏也会引导同一套登录；扩展启动用的是 `atlas login --device-auth`，不要用没有 `--device-auth` 的 `atlas login`。

退出：

```powershell
atlas logout
```

连不上 atlas-server、或启动报 *loading your account settings* 超时：见 [§9](#9-连不上-atlas-server-时如何启动)（`require_session_at_startup` + `remote_fetch`）。

---



## 4. 安装 VS Code / Cursor 扩展

扩展是 CLI 的图形前端，**不能单独工作**：必须先装好 `atlas` 并能登录。

### 用现成 VSIX（企业分发）

下载地址：<http://10.218.220.237:8888/atlas/atlas-vscode-3.1.0.vsix>

```powershell
# 下载
Invoke-WebRequest -Uri http://10.218.220.237:8888/atlas/atlas-vscode-3.1.0.vsix -OutFile atlas-vscode-3.1.0.vsix

# VS Code
code --install-extension .\atlas-vscode-3.1.0.vsix --force

# Cursor
cursor --install-extension .\atlas-vscode-3.1.0.vsix --force
```



## 5. 插件（技能与 SDD 角色）

企业安装脚本一般已经装好。手工补装或更新：

```powershell
atlas plugin marketplace add https://gitlab.imyai.cn/zhangyufeng/atlas-plugins.git
atlas plugin install atlas-sdd --trust
atlas plugin install atlas-skills --trust

atlas plugin marketplace update
atlas plugin update atlas-sdd
atlas plugin update atlas-skills
atlas plugin list
```

SSH marketplace：

```powershell
atlas plugin marketplace add git@gitlab.imyai.cn:zhangyufeng/atlas-plugins.git
```

**新开一个 CLI / 编辑器会话** 后，斜杠补全才会带上新技能正文。

---



## 6. 基础命令



### 6.1 终端里的 atlas-runtime

```powershell
atlas                  # 进入当前目录的交互会话
atlas --version
atlas login --device-auth
atlas logout
atlas models
atlas models refresh
atlas plugin list
atlas plugin update atlas-skills
```

在项目根目录启动 `atlas`，工作目录就是该仓库；`CONTEXT.md`、改文件、斜杠技能都相对这个目录。

### 6.2 会话内斜杠（CLI 与 VS Code 输入框相同）

输入 `/` 看当前安装实际提供的列表。常用：


| 命令                 | 用途                                 |
| ------------------ | ---------------------------------- |
| `/ask-atlas`       | 路由器：按你的意图选技能或企业阶段                  |
| `/grill-with-docs` | 在工作目录里拷问并留下 `CONTEXT.md` / ADR     |
| `/tdd`             | 先红后绿实现一个行为                         |
| `/implement`       | 按说明/票据实现，内部走 tdd，收尾 `/code-review` |
| `/diagnosing-bugs` | 难复现 / 回归类缺陷：先有变红的复现命令再改            |
| `/code-review`     | 相对某基点做 Standards + Spec 双轴评审       |
| `/model`           | 选模型（托管目录由服务端按账号下发）                 |
| `/refresh-model`   | 立刻从远程刷新模型目录（别名 `/refresh-models`）     |
| `/compact`         | 压缩上下文                              |
| `/plugins`         | 列出 / 重载 / 信任插件                     |


`atlas-sdd` 装好后还有阶段命令，例如 `/1-requirement-analyst`、`/2-architect-design`、`/3-program-design`、`/4-software-engineer`、`/6-qa-engineer`。它们只映射**阶段**；未拷问时不要靠这些命令跳过 `/grill-with-docs`。

VS Code 侧边栏另有 UI，不是斜杠：`+` 新会话、底部 Agent / Plan / Auto accept。

### 6.3 三种典型用法

**修 bug**

```text
/ask-atlas 登录后偶发 401
→ /diagnosing-bugs（或直接 /tdd）
→ /code-review
```

不跑需求分析 / 架构角色。

**加一个小功能点**（一个会话能说清）

```text
必要时 /grill-with-docs（决策很少可跳过）
→ /implement 或 /tdd
→ /code-review
```

**要企业需求/设计文档再实现**（`atlas-sdd` + 你明确要文档，或仓库 enterprise-strict）

```text
/ask-atlas …出需求文档
→ 按提示先 /grill-with-docs（父会话多轮）
→ Role 1 写 documents/requirements-analyst/
→ 「设计文档」Role 3（可引用已有架构；没有则让用户指定或跳过，不强制 Role 2）
→ 「实现代码」Role 4（红绿）
→ 父会话 /code-review
```

`/ask-atlas` 里的「需求文档 / 设计文档 / 实现代码」表示**当前阶段**，不是直接召唤会写那种文件的单个角色。「设计文档」走详细设计（Role 3）；架构（Role 2）只有你明确说「架构设计」才跑。

---



## 7. 更新

**CLI：** 端点指向企业 `/atlas/cli` 时，启动会检查通道版本。也可再跑一遍安装脚本。

**插件：**

```powershell
atlas plugin marketplace update
atlas plugin update atlas-sdd
atlas plugin update atlas-skills
```

**扩展：** 安装更新后的 VSIX（`--force`），或由管理员在内网市场发布新版本。

---



## 8. 常见问题


| 现象 | 处理 |
| --- | --- |
| `atlas` 不是内部或外部命令 | 新开终端；确认 `~/.atlas/bin` 或 `%USERPROFILE%\.atlas\bin` 在 PATH，或设置 `atlas.cliPath` |
| 安装提示该平台尚不可用 | atlas-server 尚未发布 `grok-{ver}-{os}-{arch}`；见 [CLI-发布.md](../services/atlas-server/docs/CLI-发布.md) |
| 扩展侧边栏起不来 / 一直要登录 | 先在终端跑通 `atlas login --device-auth`；检查 `atlas.cliPath` |
| `/ask-atlas` 没有或行为像旧版 | `atlas plugin list` 是否有 `atlas-skills`；`plugin update` 后**新开会话** |
| 没有企业角色（需求分析等） | 安装并 trust `atlas-sdd` |
| 模型列表不对 / 没有托管模型 | 登录账号在 Admin 里要有模型分配；等最多约 5 分钟缓存，或重登 / `/refresh-model` |
| 设备码批准失败 | 先浏览器登录 atlas-server；`user_code` 与账号**机器码**都要填对；用户须已在 Admin 开户 |
| `startup timed out` / *loading your account settings* | 连不上代理拉 settings；见 [§9](#9-连不上-atlas-server-时如何启动)，设 `remote_fetch = false` |
| 只关了 `require_session_at_startup` 仍卡约 40s | 不够：还必须 `remote_fetch = false`（无环境变量可替代） |
| Linux 老 glibc 跑不起来 | 用 musl 官方包；见 [atlas-编译手册.md](./atlas-编译手册.md) |

---

## 9. 连不上 atlas-server 时如何启动

企业代理（如 `10.218.220.237:22255`）不可达时，**设备码登录、托管模型拉取、远程 settings、CLI 自动更新**都会失败。本地仍可对话，需同时关掉两道门，并准备本地/BYOK 模型。

### 9.1 两道门（缺一不可）

编辑 `~/.atlas/config.toml`（Windows：`%USERPROFILE%\.atlas\config.toml`）：

```toml
[auth]
# 启动不要强制企业 auth.json 会话（仅 BYOK / 本地模型不够绕过默认门控）
require_session_at_startup = false

[features]
# 跳过远程 settings（/settings）与 /models 拉取；否则会卡在 bootstrap ~40s 后报
# Couldn't start Grok: startup timed out … loading your account settings
remote_fetch = false
```

说明：

| 配置 | 作用 |
| --- | --- |
| `require_session_at_startup = false` | 新进程不要求 OAuth/OIDC 会话；也可用 `GROK_REQUIRE_SESSION_AT_STARTUP=0` |
| `remote_fetch = false` | 不拉账号 settings / 远程模型目录；**没有**环境变量，`GROK_CONFIG` 覆盖也改不了，只能写 toml |

报错特征：`Mode: local agent`，`Steps: … bootstrap≈43s …`，提示 *The longest step was loading your account settings*。日志：`~/.atlas/logs/unified.jsonl`。

### 9.2 配置本地或 BYOK 模型

手写段**不要**加 `managed = true`（避免被托管同步清掉）：

```toml
[model.local]
model = "qwen2.5-coder"                 # 或你的上游模型名
base_url = "http://127.0.0.1:11434/v1" # Ollama / OpenAI 兼容网关
api_key = "ollama"
api_backend = "chat_completions"
context_window = 128000
```

```bash
atlas -m local
# 或会话内 /model local
```

也可用 `XAI_API_KEY` 指向能直连的公网/内网网关（会话令牌不存在时作为回退）。

### 9.3 可选：少打云端

```toml
[features]
telemetry = false
remote_fetch = false

[telemetry]
trace_upload = false
```

```bash
export GROK_DISABLE_TASK_REPORT=1
```

### 9.4 恢复内网后

1. 删掉或改回 `remote_fetch = true`（或不写，默认 true）
2. 视需要恢复 `require_session_at_startup`（或不写，默认开）
3. `atlas login --device-auth`
4. `/refresh-model` 或 `atlas models refresh` 拉托管目录

已装在本机的 `atlas-skills` / `atlas-sdd` 可离线用斜杠；**模型上游**仍须本地/BYOK。

---

