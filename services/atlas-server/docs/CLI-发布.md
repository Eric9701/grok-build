# Atlas CLI 发布指南

将 CLI 二进制发布到 **atlas-server**，供客户端通过 `GET /cli/*` 检查更新并下载。不依赖公网 x.ai CDN。

## 流程概览

```text
编译 CLI → publish-release.ps1 → releases/ → atlas-server 静态托管 → CLI 自动更新
```

| 步骤 | 动作 |
|------|------|
| 1 | 编译得到可执行文件（如 `atlas.exe` / `xai-grok-pager.exe`） |
| 2 | 运行发布脚本，写入 `releases/` |
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
  install.ps1                    # 可选，脚本自动从 pager scripts 拷贝
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
  -Arch x86_64 `
  -ReleasesDir .\releases
```

参数说明：

| 参数 | 必填 | 说明 |
|------|------|------|
| `-Binary` | 是 | 源二进制路径 |
| `-Version` | 是 | semver，如 `0.2.109` |
| `-Channel` | 否 | `stable` / `alpha` / `enterprise`，默认 `stable` |
| `-Arch` | 否 | `x86_64` / `aarch64`，默认跟本机 |
| `-ReleasesDir` | 否 | 默认 `../releases`（相对脚本目录） |

产物命名固定为：

```text
grok-{version}-windows-{arch}.exe
```

例如：`grok-0.2.109-windows-x86_64.exe`。

## HTTP 接口

基址：`http://10.218.220.237:22255/atlas/cli`（与 `[server] public_base_url` + `/cli` 一致）。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/atlas/cli/stable` | 返回最新 stable 版本号（`text/plain`） |
| GET | `/atlas/cli/alpha` | alpha 通道指针 |
| GET | `/atlas/cli/enterprise` | enterprise 通道指针 |
| GET / HEAD | `/atlas/cli/grok-{ver}-windows-{arch}.exe` | 下载二进制 |
| GET | `/atlas/cli/install.ps1` | 安装脚本（已发布时） |

冒烟检查：

```powershell
# 应输出版本号，例如 0.2.109
(Invoke-WebRequest -UseBasicParsing http://10.218.220.237:22255/atlas/cli/stable).Content

# 应返回 200
Invoke-WebRequest -UseBasicParsing -Method Head `
  http://10.218.220.237:22255/atlas/cli/grok-0.2.109-windows-x86_64.exe
```

未发布通道时返回 404，正文提示先运行 `publish-release.ps1`。

**注意：** 发布后立刻生效，一般不需要重启 `atlas-server.exe`。若改过 `releases_dir` 配置，才需要重启。

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
3. 需要升级时再下载 `GET /cli/grok-{ver}-windows-{arch}.exe`
4. 安装完成后本地缓存版本信息（含 stable 指针，用于显示 `[stable]` / `[alpha]`）

## 推荐发布 checklist

- [ ] 确认二进制 `--version` 与 `-Version` 一致
- [ ] 对目标通道执行 `publish-release.ps1`
- [ ] `GET /cli/{channel}` 返回正确版本号
- [ ] HEAD/GET 对应 `grok-*-windows-*.exe` 为 200
- [ ] 客户端已配置 `cli_update_base_url` / `GROK_CLI_BASE_URL`
- [ ] 用另一台或干净目录的 CLI 验证能检出更新（可选）

## 常见问题

**`/cli/stable` 404**  
`releases/stable` 不存在。先跑发布脚本。

**版本号对了但仍提示无更新**  
检查客户端通道、本机版本是否已 ≥ 通道指针，以及 `cli_update_base_url` 是否指向本机 atlas-server。

**下载 404**  
通道指针版本与磁盘上的 `grok-{ver}-windows-{arch}.exe` 文件名不一致（版本或 arch 写错）。

**大文件是否入库**  
`releases/*.exe` 已被 `.gitignore` 忽略，勿把二进制提交进 git；只在部署机本地或制品库保留。
