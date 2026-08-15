# Atlas CLI 发布指南

将 CLI 二进制发布到 **atlas-server**，供客户端通过 `GET /cli/*` 检查更新并下载。不依赖公网 x.ai CDN。

终端用户安装 CLI、VS Code 扩展与基础命令见 **[atlas-runtime 安装与使用手册](../../../docs/atlas-runtime-安装手册.md)**。

## 流程概览

```text
编译 CLI → publish-release → releases/ → atlas-server 静态托管 → CLI 自动更新
```

| 步骤 | 动作 |
|------|------|
| 1 | 在目标平台（或交叉编译）得到 CLI：`atlas.exe` / `xai-grok-pager` |
| 2 | 按平台运行发布脚本，写入 `releases/`（同一版本可连续发 Windows / Linux / macOS） |
| 3 | 确认 `GET /cli/stable` 返回版本号（无需重启 server） |
| 4 | 客户端把更新基址指到本机 `/cli` |

## 目录布局

默认目录：`services/atlas-server/releases/`（可用 `ATLAS_RELEASES_DIR` 或配置 `[data] releases_dir` 覆盖）。

```text
releases/
  stable                         # 通道指针：纯文本 semver，如 0.2.109
  alpha                          # 可选
  enterprise                     # 可选
  grok-0.2.109-windows-x86_64.exe
  grok-0.2.109-linux-x86_64
  grok-0.2.109-linux-aarch64
  grok-0.2.109-macos-x86_64
  grok-0.2.109-macos-aarch64
  install.ps1 / install-enterprise.ps1
  install.sh / install-enterprise.sh
```

通道文件内容必须是合法 semver，且无多余换行以外的杂质（脚本会写成无换行 ASCII）。

## 发布命令

在 `services/atlas-server` 下执行：

```powershell
# 发布到 stable（默认）
.\scripts\publish-release.ps1 `
  -Binary D:\atlas\atlas.exe `
  -Version 0.2.109

# 发布到 alpha
.\scripts\publish-release.ps1 `
  -Binary D:\atlas\atlas.exe `
  -Version 0.2.110 `
  -Channel alpha

# 指定架构 / 输出目录
.\scripts\publish-release.ps1 `
    -Binary ..\..\target\release\xai-grok-pager.exe `
    -Version 0.2.110 `
    -Os windows `
    -Arch x86_64 `
    -ReleasesDir .\releases

# 同一版本追加 Linux / macOS 产物（通道指针仍是该 semver）
.\scripts\publish-release.ps1 `
    -Binary D:\dist\xai-grok-pager-linux `
    -Version 0.2.109 `
    -Os linux `
    -Arch x86_64
```

Linux / macOS 本机发布：

```bash
# 编译（仓库根目录；protoc 见 README / PROTOC）
cargo build -p xai-grok-pager-bin --release
# 产物：target/release/xai-grok-pager

cd services/atlas-server
chmod +x scripts/publish-release.sh
./scripts/publish-release.sh \
    --binary ../../target/release/xai-grok-pager \
    --version 0.2.109 \
    --channel enterprise
# 交叉或拷来的二进制请显式传 --os / --arch
./scripts/publish-release.sh \
    --binary ./xai-grok-pager \
    --version 0.2.109 \
    --os macos \
    --arch aarch64 \
    --channel enterprise
```

参数说明：

| 参数 | 必填 | 说明 |
|------|------|------|
| `-Binary` / `--binary` | 是 | 源二进制路径 |
| `-Version` / `--version` | 是 | semver，如 `0.2.109` |
| `-Channel` / `--channel` | 否 | `stable` / `alpha` / `enterprise`，默认 `stable` |
| `-Os` / `--os` | 否 | `windows` / `linux` / `macos`；未指定时：`.exe` 视为 windows，否则跟本机 |
| `-Arch` / `--arch` | 否 | `x86_64` / `aarch64`，默认跟本机 |
| `-ReleasesDir` / `--releases-dir` | 否 | 默认 `../releases`（相对脚本目录） |

产物命名：

```text
grok-{version}-windows-{arch}.exe
grok-{version}-linux-{arch}
grok-{version}-macos-{arch}
```

例如：`grok-0.2.109-windows-x86_64.exe`、`grok-0.2.109-linux-x86_64`、`grok-0.2.109-macos-aarch64`。

同一通道指针对所有平台共用一个版本号。应先凑齐该版本的各平台文件，再让客户端升级；缺某个 `grok-{ver}-{os}-{arch}` 时，该平台安装会报尚未提供。

## HTTP 接口

基址：`http://10.218.220.237:22255/atlas/cli`（与 `[server] public_base_url` + `/cli` 一致）。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/atlas/cli/stable` | 返回最新 stable 版本号（`text/plain`） |
| GET | `/atlas/cli/alpha` | alpha 通道指针 |
| GET | `/atlas/cli/enterprise` | enterprise 通道指针 |
| GET / HEAD | `/atlas/cli/grok-{ver}-{os}-{arch}[.exe]` | 下载二进制（Windows 带 `.exe`） |
| GET | `/atlas/cli/install.ps1` / `install-enterprise.ps1` | Windows 安装脚本（已发布时） |
| GET | `/atlas/cli/install.sh` / `install-enterprise.sh` | Linux / macOS 安装脚本（已发布时） |

冒烟检查：

```powershell
# 应输出版本号，例如 0.2.109
(Invoke-WebRequest -UseBasicParsing http://10.218.220.237:22255/atlas/cli/stable).Content

# 应返回 200
Invoke-WebRequest -UseBasicParsing -Method Head `
  http://10.218.220.237:22255/atlas/cli/grok-0.2.109-windows-x86_64.exe

# Linux / macOS 产物
curl -fsSI http://10.218.220.237:22255/atlas/cli/grok-0.2.109-linux-x86_64
curl -fsSI http://10.218.220.237:22255/atlas/cli/grok-0.2.109-macos-aarch64
```

未发布通道时返回 404，正文提示先运行发布脚本。

**注意：** 发布后立刻生效，一般不需要重启 `atlas-server.exe`。若改过 `releases_dir` 配置，才需要重启。

## 客户端安装

```powershell
# Windows：从企业 atlas-server 安装（默认拉取 /atlas/cli，并安装 atlas-sdd）
irm http://10.218.220.237:22255/atlas/cli/install.ps1 | iex
```

```bash
# macOS / Linux（企业通道）
curl -fsSL http://10.218.220.237:22255/atlas/cli/install-enterprise.sh | bash
# 或稳定通道
curl -fsSL http://10.218.220.237:22255/atlas/cli/install.sh | bash
```

安装脚本会：
1. 从 `GROK_CLI_BASE_URL`（默认 `http://10.218.220.237:22255/atlas/cli`）下载通道二进制
2. 写入 `~/.atlas/config.toml` 的 `[endpoints]`（update + chat proxy）
3. 添加 marketplace 并 best-effort 安装 `atlas-sdd` + `atlas-skills`（`GROK_SKIP_ATLAS_SDD=1` / `GROK_SKIP_ATLAS_SKILLS=1` 可跳过）

Role-Scoped Skill Invocation 依赖 `atlas-skills` 可发现；详见 monorepo `docs/adr/0003-enterprise-skill-discoverability.md`。

发布新版后请重新执行发布脚本，以便 `releases/` 下的 `install.ps1` / `install.sh`（及 enterprise 变体）与源码脚本同步。

## 客户端配置

让 CLI 从 atlas-server 拉更新，而不是公网 CDN。

`~/.atlas/config.toml`：

```toml
[endpoints]
cli_chat_proxy_base_url = "http://10.218.220.237:22255/atlas/v1"
cli_update_base_url = "http://10.218.220.237:22255/atlas/cli"
```

或环境变量：

```powershell
$env:GROK_CLI_CHAT_PROXY_BASE_URL = "http://10.218.220.237:22255/atlas/v1"
$env:GROK_CLI_BASE_URL = "http://10.218.220.237:22255/atlas/cli"
```

服务端配置（可选，`atlas-server.toml`）：

```toml
[data]
releases_dir = "./releases"
```

## CLI 侧行为

1. 启动 / 检查更新时请求 `GET {cli_update_base_url}/{channel}`（如 `/cli/stable`）
2. 解析纯文本版本号，与本机版本比较
3. 需要升级时再下载 `GET /cli/grok-{ver}-{os}-{arch}[.exe]`
4. 安装完成后本地缓存版本信息（含 stable 指针，用于显示 `[stable]` / `[alpha]`）

## 推荐发布 checklist

- [ ] 确认二进制 `--version` 与 `-Version` 一致
- [ ] 对目标通道执行 `publish-release.ps1` 或 `publish-release.sh`（每个平台各一次）
- [ ] `GET /cli/{channel}` 返回正确版本号
- [ ] HEAD/GET 对应平台的 `grok-{ver}-{os}-{arch}[.exe]` 为 200
- [ ] 客户端已配置 `cli_update_base_url` / `GROK_CLI_BASE_URL`
- [ ] 用另一台或干净目录的 CLI 验证能检出更新（可选）

## 常见问题

**`/cli/stable` 404**  
`releases/stable` 不存在。先跑发布脚本。

**版本号对了但仍提示无更新**  
检查客户端通道、本机版本是否已 ≥ 通道指针，以及 `cli_update_base_url` 是否指向本机 atlas-server。

**下载 404**  
通道指针版本与磁盘上的 `grok-{ver}-{os}-{arch}[.exe]` 文件名不一致（版本、os 或 arch 写错）。Linux/macOS 没有对应产物时，安装脚本会报该平台尚不可用。

**大文件是否入库**  
`releases/*` 已被 `.gitignore` 忽略，勿把二进制提交进 git；只在部署机本地或制品库保留。
