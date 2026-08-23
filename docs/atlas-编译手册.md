# Atlas 从源码编译手册

本文说明如何在 **Windows / Linux / macOS** 上编译 Atlas CLI，以及可选的 atlas-server。发布到企业更新通道见 [CLI 发布指南](../services/atlas-server/docs/CLI-发布.md)。

所有 CLI 命令均在**仓库根目录**执行（含 `Cargo.toml` 的那一层）。

---

## 1. 产物对照

| 组件 | 包 / 入口 | 本机产物 | 发布文件名 |
|------|-----------|----------|------------|
| CLI（TUI / ACP） | `cargo build -p xai-grok-pager-bin --release` | `target/release/xai-grok-pager`（Windows 带 `.exe`） | `grok-{ver}-{os}-{arch}` |
| CLI（Linux x86_64，兼容 CentOS 7 等老 glibc） | `cargo build -p xai-grok-pager-bin --release --target x86_64-unknown-linux-musl` | `target/x86_64-unknown-linux-musl/release/xai-grok-pager` | 同上 `linux` / `x86_64` |
| atlas-server | `go build ./cmd/server`（在 `services/atlas-server`） | `atlas-server` / `atlas-server.exe` | 自行部署，不走 `/cli` |

官方安装把 CLI 二进制改名为 `atlas` / `atlas.exe`。`--version` 展示的 semver 来自编译时的 `GROK_VERSION`（未设则用 crate 版本）。

建议发版时显式指定，例如 `0.2.131`：

```powershell
# Windows PowerShell
$env:GROK_VERSION = "0.2.131"
```

```bash
# Linux / macOS
export GROK_VERSION=0.2.131
```

---

## 2. 公共依赖

| 依赖 | 说明 |
|------|------|
| **Rust** | 由根目录 [`rust-toolchain.toml`](../rust-toolchain.toml) 钉死（当前 `1.94.0`）。安装 [rustup](https://rustup.rs/) 后第一次 `cargo build` 会自动拉工具链。 |
| **Git** | `build.rs` 用 `git rev-parse --short HEAD` 写入版本字符串里的 commit。 |
| **protoc** | proto 代码生成。查找顺序：`$PROTOC` → 仓库 `bin/` → `PATH`。 |
| **网络** | 首次会拉 crates；macOS / Linux 的 **Release** 还会尝试从 GitHub 下载 ripgrep 15.0.0 打进 CLI。访问不了 GitHub 时见各平台「ripgrep」小节。 |

可选：Unix 上安装 [DotSlash](https://dotslash-cli.com)（`cargo install dotslash`），以便使用仓库里的 `bin/protoc` 包装器。Windows 不要用这个包装器（不是 PE），改用下面的 `PROTOC`。

---

## 3. Windows（x86_64）

### 3.1 环境

- Windows 10/11，PowerShell
- [rustup](https://rustup.rs/)（MSVC 工具链：`x86_64-pc-windows-msvc`）
- Visual Studio Build Tools 或 VS，带「使用 C++ 的桌面开发」
- 仓库内已解压的 protoc：

```powershell
$env:PROTOC = "E:\work\mygit\architechure\grok-build\bin\protoc-win64\bin\protoc.exe"
```

把路径换成你的仓库根目录。也可写进用户环境变量，避免每次开终端都设。

Windows Release **不会**自动下载 ripgrep（无 zip 解压路径），运行时用 PATH 上的 `rg`。需要捆绑时再设 `GROK_TOOLS_BUNDLE_RG_PATH` 指向本地 `rg.exe`。

### 3.2 编译 CLI

```powershell
cd E:\work\mygit\architechure\grok-build
$env:GROK_VERSION = "0.2.131"
$env:PROTOC = "$PWD\bin\protoc-win64\bin\protoc.exe"

cargo build -p xai-grok-pager-bin --release
```

产物：`target\release\xai-grok-pager.exe`

本地试跑：

```powershell
.\target\release\xai-grok-pager.exe --version
```

### 3.3 编译 atlas-server（可选）

```powershell
cd services\atlas-server
$env:CGO_ENABLED = "0"
# 若 go env GOOS 被设成 linux，先改回本机：
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o atlas-server.exe ./cmd/server
```

---

## 4. Linux（x86_64 / aarch64）

### 4.1 环境

- rustup + 钉死的 1.94 工具链
- 系统包：常见需要 `build-essential`、`pkg-config`、`git`
- protoc：安装 `protobuf-compiler`，或 `export PROTOC=$(which protoc)`，或 `cargo install dotslash` 后使用仓库 `bin/protoc`

### 4.2 编译 CLI

```bash
cd /path/to/grok-build
export GROK_VERSION=0.2.131

# 访问不了 GitHub 时，先装 rg 再指定路径（见第 6 节）
# export GROK_TOOLS_BUNDLE_RG_PATH="$(which rg)"

cargo build -p xai-grok-pager-bin --release
```

产物：`target/release/xai-grok-pager`

**注意：** 上述命令默认链接**本机 glibc**（`x86_64-unknown-linux-gnu`）。在 Ubuntu 22.04+ 等新系统上编出的二进制**无法在 CentOS 7 / RHEL 7**（glibc 2.17）上运行，会报 `GLIBC_2.xx not found`。要给老发行版发 Linux x86_64 包，请用 [4.4 节 musl 静态构建](#44-centos-7--老-glibc-兼容musl-静态构建)。

本机是 x86_64 就得到 linux-x86_64；本机是 ARM 得到 linux-aarch64。交叉编译需先加 target（链接器另配，本文以本机编译为准）：

```bash
rustup target add aarch64-unknown-linux-gnu   # 仅在 x86 主机交叉编 ARM 时需要
```

确认架构：

```bash
file target/release/xai-grok-pager
```

### 4.3 编译 atlas-server（可选）

```bash
cd services/atlas-server
chmod +x scripts/build-linux.sh
./scripts/build-linux.sh          # linux/amd64 → ./atlas-server
./scripts/build-linux.sh arm64    # linux/arm64
# 或：
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o atlas-server ./cmd/server
```

需要 Go **1.25+**。

### 4.4 CentOS 7 / 老 glibc 兼容（musl 静态构建）

#### 现象

在 CentOS 7.9 上运行在较新 Linux（如 Ubuntu Server 26）上编译的 `atlas`，会出现类似错误：

```text
./atlas: /lib64/libc.so.6: version `GLIBC_2.27' not found (required by ./atlas)
./atlas: /lib64/libc.so.6: version `GLIBC_2.39' not found (required by ./atlas)
...
```

#### 原因

| 环境 | 典型 glibc |
|------|------------|
| CentOS 7 / RHEL 7 | **2.17** |
| Ubuntu 22.04 | 2.35 |
| Ubuntu 24.04+ | 2.39+ |

glibc **只能向前兼容**：在新系统上用默认 `linux-gnu` 目标编出的动态链接二进制，依赖较新的 glibc 符号，老系统无法加载。把新系统的 `libc.so.6` 拷到 CentOS 7 **无效且危险**，不要这样做。

本仓库 Linux x86_64 **正式发布**使用 `x86_64-unknown-linux-musl` 静态链接（见根目录 [`.cargo/config.toml`](../.cargo/config.toml) 及 `xai-grok-tools` / `xai-grok-shell` 的 `build.rs`）。musl 静态二进制不依赖目标机 glibc，可在 CentOS 7 及多数现代 Linux 上直接运行。

`sqlite-vec`（记忆检索依赖）在 musl 上会因缺少 BSD 类型 `u_int8_t` 等编译失败。仓库 [`.cargo/config.toml`](../.cargo/config.toml) 的 `[env]` 已为 musl 注入对应 `CFLAGS_*`；若构建机未带上该配置，需手动 export（见下方「常见失败」）。

#### 构建步骤（在任意较新 Linux 上交叉编即可）

```bash
# 1. 安装 musl 工具链（Debian / Ubuntu）
sudo apt update
sudo apt install -y musl-tools musl-dev build-essential pkg-config git protobuf-compiler

# 2. 添加 Rust target
rustup target add x86_64-unknown-linux-musl

# 3. 编译（仓库根目录；需能读到 .cargo/config.toml）
cd /path/to/grok-build
export GROK_VERSION=0.2.131

# 若仍报 sqlite-vec u_int8_t 错误，显式注入（与 .cargo/config.toml 等价）：
# export CFLAGS_x86_64_unknown_linux_musl="-Du_int8_t=uint8_t -Du_int16_t=uint16_t -Du_int64_t=uint64_t"

# 访问不了 GitHub 时先指定本地 rg（见第 6 节；musl 构建应使用 musl 版 rg）
# export GROK_TOOLS_BUNDLE_RG_PATH=/path/to/rg

cargo build -p xai-grok-pager-bin --release --target x86_64-unknown-linux-musl
```

产物：

```text
target/x86_64-unknown-linux-musl/release/xai-grok-pager
```

#### 常见失败：`sqlite-vec` / `u_int8_t`

报错类似 `unknown type name 'u_int8_t'`、`conflicting types for 'uint8_t'`、`expected 'u8 *' {aka 'int *'}`：

```bash
export CFLAGS_x86_64_unknown_linux_musl="-Du_int8_t=uint8_t -Du_int16_t=uint16_t -Du_int64_t=uint64_t"
# aarch64 musl 则用：
# export CFLAGS_aarch64_unknown_linux_musl="-Du_int8_t=uint8_t -Du_int16_t=uint16_t -Du_int64_t=uint64_t"

cargo build -p xai-grok-pager-bin --release --target x86_64-unknown-linux-musl
```

#### 发布前自检

```bash
BINARY=target/x86_64-unknown-linux-musl/release/xai-grok-pager

file "$BINARY"
# 期望：ELF 64-bit LSB executable, x86-64, statically linked

ldd "$BINARY"
# 期望：not a dynamic executable（或 statically linked）

"$BINARY" --version
```

拷到 CentOS 7 验证：

```bash
cp "$BINARY" ./atlas
chmod +x ./atlas
./atlas --version
```

#### 与默认 gnu 构建对比

| 构建方式 | 命令 | 产物路径 | CentOS 7 |
|----------|------|----------|----------|
| 默认（本机 gnu） | `cargo build -p xai-grok-pager-bin --release` | `target/release/xai-grok-pager` | ❌ glibc 过新 |
| musl 静态（推荐发 Linux x86_64） | `cargo build … --target x86_64-unknown-linux-musl` | `target/x86_64-unknown-linux-musl/release/xai-grok-pager` | ✅ |

#### 备选方案

若 musl 构建环境暂时搭不起来，可在 **glibc 足够老** 的容器里用默认 gnu 目标编译，例如 `centos:7` 或 `rockylinux:8` 镜像中安装 rustup 后执行 `cargo build -p xai-grok-pager-bin --release`。产物仍动态链接 glibc，但下限与构建环境一致，可跑在 CentOS 7 上。维护成本通常高于 musl 静态构建。

#### 发布到 atlas-server

musl 产物路径与默认 gnu 不同，发布时 `--binary` 要指向 musl 目录：

```bash
cd services/atlas-server
./scripts/publish-release.sh \
  --binary ../../target/x86_64-unknown-linux-musl/release/xai-grok-pager \
  --version 0.2.131 \
  --os linux \
  --arch x86_64
```

---

## 5. macOS（Apple Silicon / Intel）

### 5.1 环境

- Xcode Command Line Tools：`xcode-select --install`
- rustup
- protoc：`brew install protobuf`，或 `cargo install dotslash` 使用 `bin/protoc`

Apple Silicon 默认 triple 是 `aarch64-apple-darwin`。若终端跑在 **Rosetta** 下（`uname -m` 为 `x86_64`），必须显式加 `--target`，否则编出的是 Intel 版。

### 5.2 编译 CLI（Apple Silicon，推荐）

Release 会从 GitHub 拉 `ripgrep-15.0.0-aarch64-apple-darwin.tar.gz`。国内网络常失败，**建议先指定本地 rg**：

```bash
brew install ripgrep protobuf
export GROK_TOOLS_BUNDLE_RG_PATH="$(which rg)"   # 一般为 /opt/homebrew/bin/rg
export GROK_VERSION=0.2.131
# 可选：export PROTOC="$(which protoc)"

cd /path/to/grok-build
cargo build -p xai-grok-pager-bin --release
```

产物：`target/release/xai-grok-pager`

写死架构（Rosetta 终端或交叉时用）：

```bash
rustup target add aarch64-apple-darwin
cargo build -p xai-grok-pager-bin --release --target aarch64-apple-darwin
# 产物：target/aarch64-apple-darwin/release/xai-grok-pager
```

确认是 arm64：

```bash
file target/release/xai-grok-pager
# 期望：Mach-O 64-bit executable arm64
```

### 5.3 Intel Mac

```bash
export GROK_VERSION=0.2.131
export GROK_TOOLS_BUNDLE_RG_PATH="$(which rg)"
cargo build -p xai-grok-pager-bin --release
# 或：--target x86_64-apple-darwin
```

### 5.4 编译 atlas-server（可选）

```bash
cd services/atlas-server
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o atlas-server ./cmd/server
```

---

## 6. ripgrep 下载失败（macOS / Linux Release）

报错类似：

```text
Failed to download ripgrep: error sending request for url
(https://github.com/BurntSushi/ripgrep/releases/download/15.0.0/...)
Set GROK_TOOLS_BUNDLE_RG_PATH to a local rg for offline builds.
```

处理：

```bash
# 方案 A：包管理器
brew install ripgrep          # macOS
# sudo apt install ripgrep    # Debian/Ubuntu

export GROK_TOOLS_BUNDLE_RG_PATH="$(which rg)"
cargo build -p xai-grok-pager-bin --release
```

方案 B：在能访问 GitHub 的机器下载对应 tar.gz，拷到构建机后解压，把路径指到 **`rg` 可执行文件**（不是目录）：

| 平台 | 资源名 |
|------|--------|
| macOS Apple Silicon | `ripgrep-15.0.0-aarch64-apple-darwin.tar.gz` |
| macOS Intel | `ripgrep-15.0.0-x86_64-apple-darwin.tar.gz` |
| Linux x86_64 | `ripgrep-15.0.0-x86_64-unknown-linux-musl.tar.gz` |
| Linux aarch64 | `ripgrep-15.0.0-aarch64-unknown-linux-gnu.tar.gz` |

Windows Release 默认跳过自动下载，一般无此问题。

`GROK_TOOLS_BUNDLE_RG_PATH` 只在 **release** 且非 Windows 自动打包时必需；`cargo check` / debug 不会去拉 rg。

---

## 7. 常见问题

**`protoc not found`（Windows）**  
设置 `PROTOC` 为 `bin\protoc-win64\bin\protoc.exe` 的绝对路径。不要用 Unix 的 `bin/protoc`（os error 193）。

**版本号不是预期 semver**  
`build.rs` 读 `GROK_VERSION`。PowerShell 用 `$env:GROK_VERSION`，同一会话里先设再 `cargo build`。改了环境变量后若没重编 `xai-grok-pager-bin`，可能仍显示旧号。

**`cannot find value origin` 一类合并编译错误**  
多半是叠了上游 `origin/main` 后冲突没消干净。以当前 `feature-atlas` 为准，task report 应传 `input_origin.as_prompt_origin()`。

**jemalloc / Apple Silicon**  
仓库 [`.cargo/config.toml`](../.cargo/config.toml) 已为 `aarch64-apple-darwin` 设置 16KB 页（`AARCH64_APPLE_DARWIN_JEMALLOC_SYS_WITH_LG_PAGE=14`），无需手改。

**CentOS 7 上 `GLIBC_2.xx not found`**  
说明二进制是在较新 Linux 上用默认 gnu 目标编的。请按 [4.4 节](#44-centos-7--老-glibc-兼容musl-静态构建) 用 `x86_64-unknown-linux-musl` 重新构建并发布。

**musl 构建报 `sqlite-vec` / `u_int8_t`**  
`sqlite-vec` 0.1.7-alpha.2 依赖 BSD 别名类型，musl 没有。仓库 `.cargo/config.toml` 已注入 `CFLAGS_*_unknown_linux_musl`；也可手动 `export CFLAGS_x86_64_unknown_linux_musl="-Du_int8_t=uint8_t -Du_int16_t=uint16_t -Du_int64_t=uint64_t"` 后再编。

---

## 8. 编完之后怎么发布

把本机产物交给 atlas-server 的 `releases/`：

```powershell
# Windows 产物
cd services\atlas-server
.\scripts\publish-release.ps1 `
  -Binary ..\..\target\release\xai-grok-pager.exe `
  -Version 0.2.131 `
  -Os windows `
  -Arch x86_64
```

```bash
# Linux / macOS 产物（在对应机器上编完再拷或本机发）
cd services/atlas-server
./scripts/publish-release.sh \
  --binary ../../target/release/xai-grok-pager \
  --version 0.2.131 \
  --os macos \
  --arch aarch64
```

同一版本号应凑齐各平台文件后再让客户端升级。细节与 HTTP 路径见 [CLI 发布指南](../services/atlas-server/docs/CLI-发布.md)。
