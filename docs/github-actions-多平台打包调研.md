# 用 GitHub Actions 打包 Atlas CLI（Windows / Linux / macOS ARM）

调查日期：2026-10-08。对象是本仓库 Rust 包 `xai-grok-pager-bin` 的发布二进制，不是 VS Code 扩展的 Electron 安装包。

仓库根已启用 [`.github/workflows/cli-release.yml`](../.github/workflows/cli-release.yml)，只编译 `xai-grok-pager-bin`。与下面草稿的唯一差别：`bin/protoc-win64/bin/protoc.exe` 被 `.gitignore` 的 `/bin/` 与 `*.exe` 排除，checkout 里没有它。workflow 下载与 `bin/protoc` 锁定的 protoc 29.3 win64，再设置 `PROTOC`。不要把 `PROTOC` 指到仓库内的 Windows 路径。

`-latest` 会迁移。下面「当前映射」以当天打开的文档为准，不能当成永久标签表。

## 1. 结论

三类产物各用一台**同架构的托管 runner 原生编译**，再用 matrix 并行。不要在一台 Linux runner 上交叉编出三个操作系统。

| 产物 | 建议 `runs-on` | 当天该 label 的镜像 | Rust target | 发布文件名 |
| --- | --- | --- | --- | --- |
| Windows x86_64 | `windows-latest` | Windows Server 2025，x64 | `x86_64-pc-windows-msvc` | `grok-{ver}-windows-x86_64.exe` |
| Linux x86_64（正式发布） | `ubuntu-latest` | Ubuntu 24.04，x64 | `x86_64-unknown-linux-musl` | `grok-{ver}-linux-x86_64` |
| macOS ARM | `macos-latest` | macOS 26，arm64（Apple Silicon） | `aarch64-apple-darwin` | `grok-{ver}-macos-aarch64` |

依据：

- runner 标签与架构来自 [GitHub-hosted runners reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners) 的标准 runner 表，以及 [actions/runner-images README 的 Available Images](https://github.com/actions/runner-images/blob/main/README.md)。两处都把 `windows-latest` 指到 Windows Server 2025 x64，把 `ubuntu-latest` 指到 Ubuntu 24.04 x64，把 `macos-latest` 指到 macOS 26 arm64。
- target 与文件名来自 [docs/atlas-编译手册.md](atlas-编译手册.md) 和 [services/atlas-server/docs/CLI-发布.md](../services/atlas-server/docs/CLI-发布.md)。Linux x86_64 正式发布用 musl，不是默认的 `linux-gnu`。
- 工具链版本以仓库根 [rust-toolchain.toml](../rust-toolchain.toml) 的 `channel = "1.94.0"` 为准。rustup 会读这个文件，优先级高于默认工具链（[rustup Overrides](https://rust-lang.github.io/rustup/overrides.html)）。

若发布流水线不希望构建机在没有提交的情况下被 `-latest` 换掉：

- Linux 钉 `ubuntu-24.04`。它和当天的 `ubuntu-latest` 是同一张 x64 镜像。Ubuntu 24.04 镜像页的 Announcements 写着：`ubuntu-latest` 将在 **2026 年 11 月**改用 Ubuntu 26.04。横幅没有写具体哪一天。见 [Ubuntu2404-Readme.md](https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md) 与 [ubuntu26/20260927 发行说明](https://github.com/actions/runner-images/releases/tag/ubuntu26/20260927.149) 里的同一句。`ubuntu-26.04` 已经可以显式选用，并且已经 GA（[runner-images#14747](https://github.com/actions/runner-images/issues/14747)）。
- macOS ARM 可以钉 `macos-26`。runner-images README 把 `macos-latest`、`macos-26` 都标成 macOS 26 arm64。不要为了「钉死」去抄 Electron 流水线的 `macos-15`，原因见第 5 节。
- Windows 可以钉 `windows-2025`。它和 `windows-latest` 在上述两份文档里都是 Windows Server 2025 x64。

`jobs.<job_id>.runs-on` 决定 job 跑在哪种机器上，可以是托管 runner、larger runner 或 self-hosted（[workflow 语法](https://docs.github.com/en/actions/learn-github-actions/workflow-syntax-for-github-actions#jobsjob_idruns-on)）。matrix 用一份 job 定义展开成多组配置（[Running variations of jobs in a workflow](https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/running-variations-of-jobs-in-a-workflow)）。`strategy.fail-fast` 默认是 `true`：矩阵里有一个 job 失败，其余进行中和排队的 job 会被取消。三个平台要独立出包，必须写成 `fail-fast: false`（[语法：fail-fast](https://docs.github.com/en/actions/learn-github-actions/workflow-syntax-for-github-actions#jobsjob_idstrategyfail-fast)）。

## 2. 托管 runner 怎么选

### 2.1 当天的 `-latest`

[reference 页](https://docs.github.com/en/actions/reference/runners/github-hosted-runners) 写明：`-latest` 是 GitHub 提供的最新稳定镜像，不一定是操作系统厂商最新的版本。

公开仓库与私有仓库的标准机规格不同。下面是该页表格里的 CPU / 内存 / 架构（存储都是 14 GB SSD）。私有仓库会占用账号的免费分钟，用完后按分钟计费；该页只指向 Actions runner pricing，**本文没有打开计费页，不写单价**。

| Label | 公开仓库 | 私有仓库 | 架构 | 镜像（两份文档一致的部分） |
| --- | --- | --- | --- | --- |
| `ubuntu-latest` | 4 vCPU，16 GB | 2 vCPU，8 GB | x64 | Ubuntu 24.04 |
| `windows-latest` | 4 vCPU，16 GB | 2 vCPU，8 GB | x64 | Windows Server 2025 |
| `macos-latest` | 3 vCPU（M1），7 GB | 同左 | arm64 | macOS 26 |

同一页还列出、且 [runner-images README](https://github.com/actions/runner-images/blob/main/README.md) 也列出的相关 label：

- Linux x64：`ubuntu-24.04`、`ubuntu-22.04`、`ubuntu-26.04`，另有 1 vCPU 的 `ubuntu-slim`（不适合本仓库这种编译）。
- Linux arm64：`ubuntu-24.04-arm`、`ubuntu-22.04-arm`、`ubuntu-26.04-arm`。**没有** `ubuntu-latest-arm` 这种 label。
- Windows x64：`windows-2025`、`windows-2022`，以及 `windows-2025-vs2026`。
- Windows arm64：`windows-11-arm`、`windows-11-vs2026-arm`。本任务要的是 Windows x86_64，不用这些。
- macOS arm64 标准机：`macos-14`、`macos-15`、`macos-26`，另有 public preview 的 `xcode-27`。
- macOS Intel 标准机：`macos-15-intel`、`macos-26-intel`。

[larger runners 参考](https://docs.github.com/en/actions/reference/runners/larger-runners) 里的 macOS 大机：

| 规格 | 架构 | 资源 | Label |
| --- | --- | --- | --- |
| Large | Intel | 12 CPU，30 GB | `macos-latest-large`、`macos-14-large`、`macos-15-large`（表内标 latest）、`macos-26-large` |
| XLarge | arm64（M2） | 5 CPU，另有 8 核 GPU 硬件加速，14 GB | `macos-latest-xlarge`、`macos-14-xlarge`、`macos-15-xlarge`（表内标 latest）、`macos-26-xlarge`，以及 preview 的 `xcode-27-xlarge` |

larger runner 要 GitHub Team 或 GitHub Enterprise Cloud，并且该页写明必须有有效信用卡、消费限额大于 0。默认的 CLI 打包用标准机即可。macOS 标准 arm64 只有 7 GB 内存；如果原生编译内存不够，再考虑 `macos-26-xlarge` 这类 **arm64** 大机，而不是 Intel 的 `*-large`。

两份官方页面关于 macOS 机房的说法不一致，本文不择一：

- reference 页写：Windows 和 Ubuntu 在 Azure；macOS 在 GitHub 自己的 macOS cloud。
- [concepts 页](https://docs.github.com/en/actions/concepts/runners/github-hosted-runners) 写：Linux 和 Windows 在 Azure；macOS 在 Azure 数据中心。

当天打开的 [about 页](https://docs.github.com/en/actions/using-github-hosted-runners/using-github-hosted-runners/about-github-hosted-runners) 与 concepts 页正文相同。

`windows-latest` 的软件清单链接也不完全同一：

- reference 页把 `windows-latest` / `windows-2025` 链到 [Windows2025-Readme.md](https://github.com/actions/runner-images/blob/main/images/windows/Windows2025-Readme.md)，把 `windows-2025-vs2026` 另链一份。
- runner-images README 把 `windows-latest`、`windows-2025`、`windows-2025-vs2026` 写在同一行，Included Software 指向 windows-2025-vs2026。

本次实际读过的是前一份 Windows2025-Readme.md（Image Version 20260719.202.1）。其中有 Visual Studio Enterprise 2022，以及 `Microsoft.VisualStudio.Component.VC.Tools.x86.x64`。这满足编译手册对 MSVC /「使用 C++ 的桌面开发」的要求。job 日志 `Set up job` → `Runner Image` → `Included Software` 才是那一次运行的清单（concepts 页）。

### 2.2 Windows x86_64

在 `windows-latest`（x64）上原生编 `x86_64-pc-windows-msvc`。

编译手册第 3 节：Windows 使用 MSVC 工具链，并把 `PROTOC` 指到仓库里的 `bin/protoc-win64/bin/protoc.exe`。不要用 Unix 的 `bin/protoc`（不是 PE，os error 193）。本次工作区里该 `protoc.exe` 存在。

同一份 Windows2025-Readme 的 Shells 表有 `gitbash.exe`（`C:\Program Files\Git\bin\bash.exe`），所以下面草稿在 Windows 上也可以用 `shell: bash`。Rust Tools 段写的是 Rust 1.97.1、Rustup 1.29.0，**不是**仓库钉死的 1.94.0。镜像自带的 rustc 不能代替 `rust-toolchain.toml`。

`.cargo/config.toml` 的 `[target.x86_64-pc-windows-msvc]` 已把链接器设为 `rust-lld`，注释写明 MSVC `link.exe` 会在这个大二进制上以 `0xc0000409` 中止。workflow 不必再设链接器，也不要另设 `RUSTFLAGS` 把这段配置盖掉。该文件开头写明 rustflags 不可叠加。

### 2.3 Linux x86_64

在 `ubuntu-latest`（当天是 Ubuntu 24.04 x64）上安装 musl 工具链，再 `--target x86_64-unknown-linux-musl`。

编译手册第 4.4 节把这定为 Linux x86_64 **正式发布**方式：静态链接，不依赖目标机 glibc，以便 CentOS 7（glibc 2.17）能跑。默认 `x86_64-unknown-linux-gnu` 会吃构建机的 glibc，在新 Ubuntu 上编出来的动态二进制不能在 CentOS 7 上运行。手册给出的 apt 包是：

```bash
sudo apt install -y musl-tools musl-dev build-essential pkg-config git protobuf-compiler
rustup target add x86_64-unknown-linux-musl
```

`rust-toolchain.toml` 的 `targets` 只有 `x86_64-unknown-linux-gnu` 和 `aarch64-unknown-linux-gnu`，**没有** musl。rustup 文档写 host 会自动装上，文件里的 targets 是额外的。musl 必须再 `rustup target add`。

sqlite-vec 在 musl 上缺少 BSD 类型 `u_int8_t`。`.cargo/config.toml` 的 `[env]` 已设置 `CFLAGS_x86_64_unknown_linux_musl`。构建目录必须是带这份配置的仓库根，不能在丢掉 `.cargo/config.toml` 的目录里编。

Ubuntu 24.04 的 Included Software 在 CLI Tools 下列出了 GitHub CLI（检索片段里的小版本不一致，随每周镜像变，这里不记死一个版本）。本次检索没有在返回片段里看到 `protobuf` / `protoc` 或 `musl-tools`，所以草稿按编译手册用 apt 安装，不假设镜像已经带了 protoc。

### 2.4 macOS ARM

在 Apple Silicon 的 macOS runner 上原生编 `aarch64-apple-darwin`。当天 `macos-latest` 就是这台机器（macOS 26 arm64，标准机为 M1）。`macos-26` 同一镜像。`macos-15` 是 macOS 15 arm64，也是 Apple Silicon，但不是 `-latest`。

编译手册第 5 节：Apple Silicon 的 triple 是 `aarch64-apple-darwin`；protoc 用 `brew install protobuf`，或 DotSlash 走仓库的 `bin/protoc`。若终端在 Rosetta 下（`uname -m` 为 `x86_64`）却不加 `--target`，会编出 Intel 版。GitHub 的 `macos-latest` 在 reference 页标明是 arm64，不是 Intel label。草稿仍然显式 `--target aarch64-apple-darwin`，避免 host 判断出错。产物可用 `file` 确认是 `Mach-O 64-bit executable arm64`（编译手册第 5.2 节）。

[macos-26-arm64-Readme.md](https://github.com/actions/runner-images/blob/main/images/macos/macos-26-arm64-Readme.md) 的检索片段里有 Xcode、Homebrew 和 GitHub CLI。本次没有在返回片段里看到预装的 `protobuf` / `protoc`，因此草稿按编译手册执行 `brew install protobuf`，不把「镜像已带 protoc」写成事实。

**不要在 Linux 上交叉编 macOS。** 这句话不是从 GitHub 文档里抄来的。GitHub 提供的是原生 macOS runner（reference 页的 arm64 macOS label）。本仓库编译手册也是在 macOS 本机安装 Xcode Command Line Tools 后编译。交叉到 `aarch64-apple-darwin` 所需的 Apple SDK / 链接器，本次打开的 GitHub 文档没有写成一套现成配方，仓库的 `.cargo/config.toml` 也没有配置从 Linux 指到 Apple SDK 的 linker。

Electron 流水线钉死 `macos-15` 的事实记在第 5 节。未签名的 CLI 不必跟着钉。

### 2.5 若还要 Linux ARM

用文档里真实存在的 arm64 Ubuntu label，在那台机器上原生编。不要在 x64 的 `ubuntu-latest` 上假设交叉链接器已经配好。

可选 label（runner-images README 与 reference 页都有）：

| Label | 架构 | 说明 |
| --- | --- | --- |
| `ubuntu-24.04-arm` | arm64 | 与当天 `ubuntu-latest` 的 24.04 同一代 |
| `ubuntu-26.04-arm` | arm64 | 已 GA（runner-images#14747） |
| `ubuntu-22.04-arm` | arm64 | 仍在镜像表里。Ubuntu 24.04 镜像 Announcements 有一句 Ubuntu 22 将开始退役，横幅**没有写年份**，本文不推断它是否已经开始 |

`rust-toolchain.toml` 虽然列出了 `aarch64-unknown-linux-gnu`，那只表示 rustup 会安装这个 target。`.cargo/config.toml` 的 `[target.aarch64-unknown-linux-gnu]` 只设置了 `target-cpu=neoverse-v2` 和 `linker-features=-lld`，**没有** `linker = "aarch64-linux-gnu-gcc"` 之类的交叉链接器。在 x64 runner 上直接 `--target aarch64-unknown-linux-gnu` 不能当成已经配好的交叉编译。

在 `ubuntu-24.04-arm` 上，host 就是 `aarch64-unknown-linux-gnu`，这是原生编译。发布文件名按 CLI 发布指南是 `grok-{ver}-linux-aarch64`。

编译手册把 musl 正式发布写在 **x86_64**。Linux aarch64 的 ripgrep 资源名是 `ripgrep-15.0.0-aarch64-unknown-linux-gnu.tar.gz`（编译手册第 6 节），不是 musl 包。因此 Linux ARM 的默认建议是原生 gnu，而不是把 x86_64 的 musl 配方原样套过去。若另做 `aarch64-unknown-linux-musl`，配置里已有对应 `CFLAGS_*`，仍应在 arm64 runner 上原生加 target，并自行安装 musl 工具链。

`.cargo/config.toml` 把 `AARCH64_UNKNOWN_LINUX_GNU_JEMALLOC_SYS_WITH_LG_PAGE` 和 musl 对应项设为 `"16"`（64KB 页）。注释说部分 aarch64 Linux 主机使用 64KB 内核页。**GitHub 文档没有写 `ubuntu-*-arm` 的内核页大小**，编 Linux ARM 之前应在 runner 上自己看 `getconf PAGE_SIZE`。

### 2.6 Intel Mac、以及不要再用的 label

本任务的 macOS 产物是 ARM，不需要 Intel runner。Intel 标准机当天仍在表里：`macos-15-intel`、`macos-26-intel`（reference 页与 runner-images README）。larger 的 Intel 机是 `*-large`。

和「Intel 何时消失」有关的原文并不一致，不要合成一个单一日期：

- [runner-images#13045](https://github.com/actions/runner-images/issues/13045)（2025-09-19，Announcement）：`macos-15-intel` 可用到 2027 年 8 月，并称这是 Actions 最后一张 x86_64 镜像，之后不再支持 x86_64。
- 2026-10-08 的 runner-images README **同时**列出 `macos-26-intel`。因此不能把 2025 年那句「只剩 macos-15-intel」当成当前清单。2027 年 8 月之后是否下线，本次打开的当前文档没有更新确认。

已退出或正在退出、新 workflow 不要使用的 macOS label：

| Label | 依据 |
| --- | --- |
| `macos-12` | [runner-images#10721](https://github.com/actions/runner-images/issues/10721)：2024-10-07 起废弃，GitHub 上 2024-12-03 起完全不支持。当前 Available Images 表里没有它。 |
| `macos-13` / `macos-13` arm64 | 当前 Available Images 表里没有。日期在公告之间不一致：GitHub Changelog（2025-07-11）写 2025-10-01 开始关闭、2025-12-04 完全退役；[#13046](https://github.com/actions/runner-images/issues/13046) 标题和 Target date 写到 12 月 4 日，正文又写 2025-12-08。issue 于 2025-12-16 关闭。可用结论只是：当前镜像表已经没有这个 label。 |
| `macos-14`、`macos-14-large`、`macos-14-xlarge` | README 打了 deprecated，链到 [#13518](https://github.com/actions/runner-images/issues/13518)。该公告写废弃从 2026-07-06 开始，2026-11-02 起 GitHub Actions 与 Azure DevOps 完全不支持。调查日 2026-10-08 落在这段窗口内，10 月还有 brownout。reference 页仍把 `macos-14` 列在标准 arm64 表里，但新流水线不要用。 |

## 3. 可复制的 workflow 草稿

下面是当时的示意。已启用的文件是仓库根的 `.github/workflows/cli-release.yml`。与本段的差别只有 Windows protoc，见文首。

触发：

- `workflow_dispatch`：语法页写明，只有 workflow 文件在默认分支上时才会收到这个事件。`inputs` 示例见同一页。
- `push.tags`：只写 `tags`、不写 `branches` 时，workflow 不会因分支 push 运行。模式针对 Git ref 的名字，文档示例里 `v1.*` 匹配 `refs/tags/v1.9.1` 这种 tag。

`GROK_VERSION` 用输入的 semver（编译手册示例是 `0.2.131`，不带 `v`）。tag 触发时从 `GITHUB_REF` 去掉 `refs/tags/` 前缀，再去掉一个前导 `v`。语法页说明 tag 的 ref 形如 `refs/tags/v2`。

产物先用 `actions/upload-artifact` 上传。该 action README 的 Usage 示例是 `@v7`；文首仍留着「请改用 v4」的旧弃用提示（针对 v3 及更早）。矩阵教程页的示例还是 `upload-artifact@v4` 和 `download-artifact@v5`，落后于这两个 action 自己的 README。下载侧 README 的 Usage 是 `@v8`。checkout README 的 Usage 是 `@v7`，并建议 `permissions: contents: read`。

挂到 GitHub Release 是可选的，而且**不创建** Release：

- `gh release upload` 的手册只写「把文件上传到一个 GitHub Release」，没有写它会创建 Release，也没有写 Release 不存在时的退出码。
- `gh release create` 会创建 Release；tag 还不存在时，还会从默认分支最新状态自动建 tag。这不是本草稿要的行为。
- [Get a release by tag name](https://docs.github.com/en/rest/releases/releases#get-a-release-by-tag-name) 写的是取**已发布**的 release，不存在时 HTTP 404。草稿先看状态码，200 才 `gh release upload`。
- `softprops/action-gh-release` README 把该 action 定义为在 Linux / Windows / macOS 上**创建** GitHub Release；tag 上已有 Release 时则更新资产。输入表里没有「只更新、不存在就失败」的开关。因此草稿不用它。
- `--clobber` 会先删同名资产再上传；手册写明上传失败时原资产会丢。仓库若开启了 immutable releases，`gh release create` 手册写明发布之后资产不能再改、不能再删。重复上传前要先确认这不是 immutable release。

权限（[workflow 语法的 permissions](https://docs.github.com/en/actions/learn-github-actions/workflow-syntax-for-github-actions#permissions)）：

- `contents: read` 的例子是列出提交。checkout README 推荐构建 job 用这个权限。
- `contents: write` 的例子是允许 action 创建 release。`write` 包含 `read`。一旦写出 `permissions`，没点名的权限都是 `none`。
- 构建 job 保持 `contents: read`。只有真正调用 Release API 的 job 设 `contents: write`。
- `gh` 使用 `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}`。这是 [GITHUB_TOKEN 文档](https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/controlling-permissions-for-github_token) 里的写法。Ubuntu 24.04、macOS 26 arm64、Windows Server 2025 的软件清单都有 GitHub CLI，所以发布 job 放在 `ubuntu-latest` 上可以直接用 `gh`。

Rust：

- 用 `actions-rust-lang/setup-rust-toolchain@v2`，**不传** `toolchain`。它的 README 写明：仓库根有 `rust-toolchain` 或 `rust-toolchain.toml` 且没有 toolchain 输入时，会安装文件里的全部项；传了 toolchain 输入则会忽略该文件。额外 target 用 `target` 输入，装完文件里的项再追加。
- `dtolnay/rust-toolchain` 不是 GitHub 官方 action，作者是 David Tolnay，Rust 项目里很常用。当天 README 写的是用 action 的 `@rev` 选工具链（`@stable`、`@1.89.0`）。本次读到的 [master action.yml](https://github.com/dtolnay/rust-toolchain/blob/master/action.yml) 和 [stable 分支 action.yml](https://github.com/dtolnay/rust-toolchain/blob/stable/action.yml) 都是 `rustup toolchain install` 加 `rustup default`，**没有**解析 `rust-toolchain.toml`。`@stable` 的默认输入就是 `stable`，不是本仓库的 1.94.0。即便如此，rustup 的优先级仍是：命令行 `+`、`RUSTUP_TOOLCHAIN`、目录 override、`rust-toolchain.toml`、最后才是 default。文件会压过 `rustup default`。用 `@stable` 不能代替文件里的 pin。
- `Swatinem/rust-cache` 也不是 GitHub 官方 action。`setup-rust-toolchain` 在 `cache: true`（其默认值）时会用它。它的 README 写明缓存键包含 `rust-toolchain.toml` 和 `.cargo/config.toml` 的哈希，并会设置 `CARGO_INCREMENTAL=0`。缓存上限它指向 GitHub 的 caching 文档；本文没有再打开该计费页核对 10 GB 是否仍是当前数字。

`upload-artifact` README 写明：打成 zip 时不保留文件权限，文件会变成 `644`。需要保留权限时先 `tar`，再用 `archive: false` 上传那个 tar。`gh release upload` 上传的是构建 job 里改好名的原始文件；Unix 可执行位是否随 Release 资产保存，该手册没有写。安装侧应按编译手册 / 发布指南的安装脚本自行 `chmod +x`。

```yaml
# 示意：.github/workflows/cli-release.yml
# 已落地。Windows protoc 以仓库里的 workflow 为准（下载 29.3），不要用下面这段仓库内路径。
name: CLI release

on:
  workflow_dispatch:
    inputs:
      version:
        description: "semver，写入 GROK_VERSION，例如 0.2.131"
        required: true
        type: string
      release_tag:
        description: "已存在的 GitHub Release tag。留空则只上传 artifact，不创建 Release"
        required: false
        type: string
  push:
    tags:
      - "v*"

permissions:
  contents: read

jobs:
  build:
    strategy:
      fail-fast: false
      matrix:
        include:
          - runner: windows-latest
            triple: x86_64-pc-windows-msvc
            os: windows
            arch: x86_64
            ext: .exe
          # 2026-10-08 的 ubuntu-latest 就是 ubuntu-24.04。
          # 镜像公告写 ubuntu-latest 将于 2026 年 11 月改指向 Ubuntu 26.04，故这里钉死 24.04。
          - runner: ubuntu-24.04
            triple: x86_64-unknown-linux-musl
            os: linux
            arch: x86_64
            ext: ""
          # 未签名 CLI。不要改成 Electron 流水线里的 macos-15。
          # 若不想跟随 macos-latest 迁移，改成 macos-26（同一张 arm64 镜像）。
          - runner: macos-latest
            triple: aarch64-apple-darwin
            os: macos
            arch: aarch64
            ext: ""
    runs-on: ${{ matrix.runner }}
    steps:
      - uses: actions/checkout@v7

      - uses: actions-rust-lang/setup-rust-toolchain@v2
        with:
          # 不传 toolchain，让 action 安装 rust-toolchain.toml（当前 1.94.0）。
          # 不传 rustflags，避免盖掉 .cargo/config.toml 里分 target 的链接参数。
          cache: true

      - name: Add musl target
        if: matrix.triple == 'x86_64-unknown-linux-musl'
        run: rustup target add x86_64-unknown-linux-musl

      - name: Linux packages
        if: matrix.os == 'linux'
        run: |
          sudo apt-get update
          sudo apt-get install -y musl-tools musl-dev build-essential pkg-config protobuf-compiler

      - name: macOS protobuf
        if: matrix.os == 'macos'
        run: brew install protobuf

      - name: GROK_VERSION and Windows PROTOC
        shell: bash
        env:
          INPUT_VERSION: ${{ inputs.version }}
        run: |
          if [ -n "$INPUT_VERSION" ]; then
            ver="$INPUT_VERSION"
          else
            ver="${GITHUB_REF#refs/tags/}"
            ver="${ver#v}"
          fi
          echo "GROK_VERSION=$ver" >> "$GITHUB_ENV"
          if [ "${{ matrix.os }}" = "windows" ]; then
            echo "PROTOC=${GITHUB_WORKSPACE}/bin/protoc-win64/bin/protoc.exe" >> "$GITHUB_ENV"
          fi

      - name: Build
        shell: bash
        run: cargo build -p xai-grok-pager-bin --release --target ${{ matrix.triple }}

      - name: Name the artifact
        shell: bash
        run: |
          src="target/${{ matrix.triple }}/release/xai-grok-pager${{ matrix.ext }}"
          dest="dist/grok-${GROK_VERSION}-${{ matrix.os }}-${{ matrix.arch }}${{ matrix.ext }}"
          mkdir -p dist
          cp "$src" "$dest"

      - uses: actions/upload-artifact@v7
        with:
          name: grok-${{ env.GROK_VERSION }}-${{ matrix.os }}-${{ matrix.arch }}${{ matrix.ext }}
          path: dist/grok-${{ env.GROK_VERSION }}-${{ matrix.os }}-${{ matrix.arch }}${{ matrix.ext }}
          if-no-files-found: error
          archive: false

  attach-release:
    needs: build
    if: >-
      ${{ needs.build.result == 'success' &&
          (github.event_name == 'push' || inputs.release_tag != '') }}
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/download-artifact@v8
        with:
          path: dist
          merge-multiple: true

      - name: Upload to an existing release
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          INPUT_TAG: ${{ inputs.release_tag }}
        run: |
          if [ -n "$INPUT_TAG" ]; then
            tag="$INPUT_TAG"
          else
            tag="${GITHUB_REF#refs/tags/}"
          fi
          status=$(curl -sS -o /dev/null -w "%{http_code}" \
            -H "Authorization: Bearer ${GH_TOKEN}" \
            -H "Accept: application/vnd.github+json" \
            -H "X-GitHub-Api-Version: 2026-03-10" \
            "https://api.github.com/repos/${GITHUB_REPOSITORY}/releases/tags/${tag}")
          if [ "$status" = "200" ]; then
            gh release upload "$tag" dist/grok-* --clobber
          elif [ "$status" = "404" ]; then
            echo "已发布的 Release ${tag} 不存在，只保留 artifact，不创建 Release"
          else
            echo "查询 Release 失败：HTTP ${status}"
            exit 1
          fi
```

这个草稿只产出 GitHub Actions artifact，并可选地把文件挂到**已经存在**的 GitHub Release。它不会调用 `services/atlas-server` 的 `publish-release`，也不会改 `releases/stable`。企业更新通道仍按 [CLI-发布.md](../services/atlas-server/docs/CLI-发布.md)：凑齐 `grok-{ver}-{os}-{arch}` 后再发通道指针。缺一个平台文件时，该平台安装会报尚未提供。

Linux ARM 不在这份矩阵里。要加的话，另加一行 `runner: ubuntu-24.04-arm`、`triple: aarch64-unknown-linux-gnu`、`os: linux`、`arch: aarch64`，并且不要设 `RUSTFLAGS`。

## 4. 本仓库特有的坑

- **PROTOC。** 查找顺序是 `$PROTOC` → 仓库 `bin/` → `PATH`（编译手册第 2 节）。Windows 必须把 `PROTOC` 设为 `bin/protoc-win64/bin/protoc.exe` 的绝对路径。Unix 的 `bin/protoc` 不是 PE。Linux 用 apt 的 `protobuf-compiler`；macOS 用 Homebrew 的 `protobuf`。镜像是否预装 protoc，本次没有在软件清单片段里确认。
- **musl 与 sqlite-vec。** 正式 Linux x86_64 用 `x86_64-unknown-linux-musl`。`CFLAGS_x86_64_unknown_linux_musl`（以及 aarch64 musl 的对应变量）写在 `.cargo/config.toml`。musl 段还有 RELRO / `noexecstack`。这些都依赖 cargo 读到仓库根的配置。
- **ripgrep。** macOS / Linux 的 Release 会从 GitHub 下载 ripgrep 15.0.0；失败时设置 `GROK_TOOLS_BUNDLE_RG_PATH` 指向本地 `rg` 可执行文件。Windows Release 不会自动下载，运行时用 PATH 上的 `rg`；要打进二进制再设 `GROK_TOOLS_BUNDLE_RG_PATH` 指向 `rg.exe`。资源名见编译手册第 6 节。托管 runner 的网络要求里包含 `github.com`（reference 页），所以 GitHub 上的下载有可能成功；这不取消失败时的本地路径开关。musl 构建应使用 musl 版 rg。
- **工具链钉在 `rust-toolchain.toml`。** `channel = "1.94.0"`，`components` 含 rustfmt 与 clippy，`profile = "default"`。Windows 镜像清单上的 Rust 是 1.97.1。不要用 `dtolnay/rust-toolchain@stable` 当作本仓库的版本来源。
- **不要设置 `RUSTFLAGS`。** `.cargo/config.toml` 按 target 写了不同的链接参数，并注明 rustflags 不可叠加。Windows 依赖其中的 `linker=rust-lld`。
- **`build.rs` 需要 git。** 编译手册写 `build.rs` 用 `git rev-parse --short HEAD`。checkout README 写默认 `fetch-depth` 为 1，这次提交仍在。不需要为了短 SHA 去 `fetch-depth: 0`。`GROK_VERSION` 未设置时回落到 crate 版本，所以发布 job 要显式设置。
- **文件名。** `grok-{version}-windows-{arch}.exe`、`grok-{version}-linux-{arch}`、`grok-{version}-macos-{arch}`，`arch` 为 `x86_64` 或 `aarch64`。同一通道指针的各平台共用一个 semver。

## 5. 不要做的事

- 不要在一台 Linux runner 上交叉编出 Windows、Linux、macOS 三个产物。三个 OS 各用同架构的原生 runner。Linux ARM 同理，用 `ubuntu-24.04-arm` 或 `ubuntu-26.04-arm`，不要假设 x64 镜像上的交叉链接器已经写好。
- 不要使用 `macos-12`、`macos-13`。不要在新 workflow 里使用 `macos-14`、`macos-14-large`、`macos-14-xlarge`（2026-11-02 起公告称完全不支持）。
- 不要把 Electron 的签名 / 公证流程套到未签名 CLI 上。`services/grok-build-vscode/.github/workflows/desktop-release.yml` 把 macOS 钉在 `macos-15`，注释写明原因是 `macos-latest` 已经滚到 `macos-26-arm64`，在该镜像上 `notarytool` 上传成功后轮询 `appstoreconnect.apple.com` 失败（"No network route"）。同一文件还使用 Developer ID、`CSC_LINK` / `CSC_KEY_PASSWORD` 和 App Store Connect API 密钥。那是已签名的 `.dmg` 安装包。CLI 发布的是未签名的 `xai-grok-pager` 二进制，没有这套公证主机问题，不必为了那个故障钉死 `macos-15`。
- 不要让 workflow 自己 `gh release create` 或使用会创建 Release 的 action，除非另外决定这就是期望行为。官方手册把「创建」和「向已有 Release 上传」分成两个命令。
- 不要用镜像自带的 Rust 默认版本代替 `rust-toolchain.toml`。

## 6. 来源清单

GitHub 文档与镜像（调查日打开的页面）：

- https://docs.github.com/en/actions/reference/runners/github-hosted-runners — 标准 runner 的 label、CPU、内存、架构；`-latest` 的含义；公开/私有规格差异；macOS 在 “GitHub's own macOS cloud” 的说法；`ubuntu-24.04-arm` 等 arm64 label。
- https://docs.github.com/en/actions/concepts/runners/github-hosted-runners — 镜像由 `actions/runner-images` 维护、每周更新；job 日志里的 Included Software 链接；macOS 在 “Azure data centers” 的说法（与上一页不一致）。
- https://docs.github.com/en/actions/using-github-hosted-runners/using-github-hosted-runners/about-github-hosted-runners — 当天抓取结果与 concepts 页相同。
- https://github.com/actions/runner-images/blob/main/README.md — Available Images：`ubuntu-latest` = Ubuntu 24.04 x64，`macos-latest` = macOS 26 arm64，`windows-latest` 行指向 Windows Server 2025；macOS 14 标 deprecated；没有 `ubuntu-latest-arm`。
- https://github.com/actions/runner-images/blob/main/images/windows/Windows2025-Readme.md — Windows Server 2025 上的 VS 2022、`VC.Tools.x86.x64`、Git Bash、GitHub CLI、Rust 1.97.1。
- https://github.com/actions/runner-images/blob/main/images/ubuntu/Ubuntu2404-Readme.md — Announcements：`ubuntu-latest` 将于 2026 年 11 月改用 Ubuntu 26.04；CLI Tools 含 GitHub CLI。
- https://github.com/actions/runner-images/releases/tag/ubuntu26/20260927.149 — 同一条 `ubuntu-latest` → Ubuntu 26.04（2026 年 11 月）公告。
- https://github.com/actions/runner-images/issues/14747 — `ubuntu-26.04` 与 `ubuntu-26.04-arm` 已 GA。
- https://github.com/actions/runner-images/blob/main/images/macos/macos-26-arm64-Readme.md — macOS 26 arm64 镜像含 Xcode、Homebrew、GitHub CLI。本次片段未列出 protobuf。
- https://docs.github.com/en/actions/reference/runners/larger-runners — `macos-*-large` 为 Intel，`macos-*-xlarge` 为 arm64（M2）。
- https://github.com/actions/runner-images/issues/10721 — `macos-12` 在 GitHub 上于 2024-12-03 完全不支持。
- https://github.com/actions/runner-images/issues/13046 — `macos-13` 退役公告；正文日期与标题 / Changelog 不一致。
- https://github.com/actions/runner-images/issues/13045 — `macos-15-intel` 计划可用到 2027 年 8 月；与当前 README 里的 `macos-26-intel` 并存，不能单独当成当前清单。
- https://github.blog/changelog/2025-07-11-upcoming-changes-to-macos-hosted-runners-macos-latest-migration-and-xcode-support-policy-updates/ — 2025 年 `macos-latest` 迁到 macOS 15 的旧公告，以及 macOS 13 于 2025-12-04 退役。当前 `-latest` 已是 macOS 26，以 reference 页为准。
- https://github.com/actions/runner-images/issues/13518 — `macos-14` / `macos-14-large` / `macos-14-xlarge` 从 2026-07-06 起废弃，2026-11-02 完全不支持。
- https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/running-variations-of-jobs-in-a-workflow — matrix、`include`、`fail-fast`。
- https://docs.github.com/en/actions/learn-github-actions/workflow-syntax-for-github-actions — `runs-on`、`on.push.tags`、`workflow_dispatch`（仅默认分支上的文件会收到）、`permissions`（`contents: read` / `contents: write`）、`fail-fast` 默认 `true`。
- https://docs.github.com/en/actions/writing-workflows/choosing-what-your-workflow-does/controlling-permissions-for-github_token — `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}`。
- https://github.com/actions/upload-artifact — Usage 为 `@v7`；zip 不保留 Unix 权限；矩阵里 artifact 名必须唯一；`archive: false` 只上传单个文件。
- https://github.com/actions/download-artifact — Usage 为 `@v8`；与 `archive: false` 的对应关系。
- https://github.com/actions/checkout — Usage 为 `@v7`；建议 `contents: read`；默认 `fetch-depth: 1`。
- https://cli.github.com/manual/gh_release_upload — 向一个 Release 上传文件；`--clobber` 会先删除同名资产。
- https://cli.github.com/manual/gh_release_create — 创建 Release；tag 不存在时会从默认分支自动建 tag。草稿故意不调用。
- https://cli.github.com/manual/gh_release_view — 不带 tag 时显示最新 Release。草稿改用下面的 REST，避免看错「最新」。
- https://docs.github.com/en/rest/releases/releases#get-a-release-by-tag-name — 按 tag 取已发布 Release；404 表示没有。`X-GitHub-Api-Version: 2026-03-10` 来自该页说明。
- https://github.com/softprops/action-gh-release — 用于创建 Release 的非官方 action；已有 Release 时更新资产。需要 `contents: write`。草稿不用它来避免自动创建。

Rust（非 GitHub 官方 action 已在正文标明）：

- https://rust-lang.github.io/rustup/overrides.html — `rust-toolchain.toml` 由 rustup 读取；优先级高于 default toolchain；`targets` 在 host 之外追加。
- https://github.com/dtolnay/rust-toolchain — `@rev` 选择工具链；README 与 action.yml 都没有读 `rust-toolchain.toml`。
- https://github.com/actions-rust-lang/setup-rust-toolchain — 未传 `toolchain` 时安装工具链文件；`target` 额外追加；默认缓存走 Swatinem/rust-cache。
- https://github.com/Swatinem/rust-cache — 非官方缓存；缓存键含工具链文件与 `.cargo/config.toml`。

本仓库：

- `docs/atlas-编译手册.md` — `GROK_VERSION`、Windows `PROTOC` 与 MSVC、Linux x86_64 正式发布用 musl、macOS `aarch64-apple-darwin`、ripgrep 下载与 `GROK_TOOLS_BUNDLE_RG_PATH`、Windows 不自动打包 rg。
- `rust-toolchain.toml` — `channel = "1.94.0"`；targets 不含 musl / Windows / Darwin。
- `.cargo/config.toml` — musl 的 `CFLAGS_*`、Windows `linker=rust-lld`、各 target 的 rustflags、aarch64 Linux jemalloc 页大小 16。
- `services/atlas-server/docs/CLI-发布.md` — `grok-{ver}-{os}-{arch}[.exe]`，`os` 为 `windows` / `linux` / `macos`，`arch` 为 `x86_64` / `aarch64`。
- `services/grok-build-vscode/.github/workflows/desktop-release.yml` — `macos-latest` 已滚到 `macos-26-arm64`，Electron 签名/公证因此钉 `macos-15`。CLI 不要照搬。

## 7. 未在官方文档确认的点

- `gh release upload` 在 Release 不存在时的退出码。手册没有写。草稿用 REST 的 404 判断，不靠这个退出码。
- 该 REST 端点写的是「已发布」的 release。草稿状态的 tag 会不会也返回 404，页面没有另外说明。
- GitHub Release 资产是否保存 Unix 可执行位。`gh release upload` 手册没有写。
- `ubuntu-*-arm` 的内核页大小。仓库把 aarch64 Linux 的 jemalloc 页设为 64KB，但是否与这些 runner 一致，文档没写。
- macOS 26 arm64 与 Ubuntu 24.04 镜像是否已经预装 `protoc`。本次软件清单检索片段里没有看到，草稿改为显式安装。
- Ubuntu 22 退役横幅里的月份没有带年份，本文不推断 `ubuntu-22.04` 是否已经开始退役。
- Actions 的每分钟单价、缓存是否仍是 10 GB、job 默认超时。相关页只被其他文档点名，本次没有打开计费页。
- reference 页与 concepts 页对 macOS runner 所在机房的说法相反，本文没有选一个当事实。
- `windows-latest` 的 Included Software 到底是 Windows2025-Readme 还是 windows-2025-vs2026。两份索引不一致。本次读到的 MSVC 组件来自 Windows2025-Readme.md。
- 2025 年公告称 `macos-15-intel` 是最后的 x86_64 镜像并支持到 2027 年 8 月，但当前 README 还有 `macos-26-intel`。2027 年 8 月之后 Intel runner 是否还在，未在当前文档里更新确认。
