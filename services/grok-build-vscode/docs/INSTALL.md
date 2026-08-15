# Advanced install (Atlas)

面向终端用户的中文安装与命令说明见仓库根目录 **[atlas-runtime 安装与使用手册](../../../docs/atlas-runtime-安装手册.md)**。

The extension is an ACP client for the **Atlas CLI** (`atlas`). Prefer installing the CLI with the enterprise scripts from this monorepo, then point the extension at it if needed.

## Install the Atlas CLI (enterprise)

From the monorepo (after building/publishing releases to your atlas-server):

- Windows: [`install-enterprise.ps1`](../../crates/codegen/xai-grok-pager/scripts/install-enterprise.ps1)
- Unix: [`install-enterprise.sh`](../../crates/codegen/xai-grok-pager/scripts/install-enterprise.sh)

Typical layout after install:

- Binary: `~/.atlas/bin/atlas` (Windows: `%USERPROFILE%\.atlas\bin\atlas.exe`)
- Config / auth: `~/.atlas/config.toml`, `~/.atlas/auth.json`
- Legacy `~/.grok` is still discovered if `~/.atlas` does not exist

### Environment (client still uses `GROK_*`)

```bash
# Optional overrides
export GROK_HOME="$HOME/.atlas"
export GROK_CLI_CHAT_PROXY_BASE_URL="http://YOUR_HOST:PORT/atlas/v1"
export GROK_CLI_BASE_URL="http://YOUR_HOST:PORT/atlas/cli"
# Optional API key path used by some tools
export XAI_API_KEY="..."
```

Then:

```bash
atlas login
```

## Extension setting

| Setting | Purpose |
|---------|---------|
| `atlas.cliPath` | Absolute path to `atlas` / `atlas.exe`. Empty = auto-discover (`$GROK_HOME/bin`, `~/.atlas/bin`, then `~/.grok/bin`, then PATH). |

Legacy `grok.*` settings are still read if the corresponding `atlas.*` key was never set.

## Build the extension from source

```bash
cd services/grok-build-vscode
npm install
npm run compile
# Package / install VSIX as usual for your org
```

## Multi-IDE

Same VSIX works in VS Code and Cursor (ACP `atlas agent stdio`).
