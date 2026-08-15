# Atlas Server

Go management backend for Atlas CLI. **Phase 1** implements xAI-compatible OAuth device-code login. Later phases will add settings, skill distribution, telemetry ingestion, and admin APIs.

## Configuration

Copy the example config and edit:

```bash
cp atlas-server.toml.example atlas-server.toml
```

Load order: **built-in defaults → `atlas-server.toml` → `ATLAS_*` env vars** (env wins).

| Config path | Description |
|-------------|-------------|
| `ATLAS_CONFIG` | Explicit config file path |
| `./atlas-server.toml` | Default (auto-discovered) |
| `./config/atlas-server.toml` | Alternate location |

MySQL can be set either as a full DSN or structured fields:

```toml
[mysql]
host = "127.0.0.1"
port = 3306
user = "atlas"
password = "atlas"
database = "atlas"
# or: dsn = "atlas:atlas@tcp(127.0.0.1:3306)/atlas?parseTime=true&charset=utf8mb4"
```

See [`atlas-server.toml.example`](atlas-server.toml.example) for all options.

## Run

```bash
cd services/atlas-server
# MySQL required (tables auto-migrate on startup)
# Default DSN: atlas:atlas@tcp(127.0.0.1:3306)/atlas?parseTime=true&charset=utf8mb4
# Quick local DB: docker run -d --name atlas-mysql -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=atlas -e MYSQL_USER=atlas -e MYSQL_PASSWORD=atlas -p 3306:3306 mysql:8

# On Windows, if `go env GOOS` is linux, force the host target:
#   set GOOS=windows&& set GOARCH=amd64
go run ./cmd/server
```

Windows helper: [`scripts/run-windows.ps1`](scripts/run-windows.ps1).

Linux build:

```bash
chmod +x scripts/build-linux.sh
./scripts/build-linux.sh          # → ./atlas-server (linux/amd64)
./scripts/build-linux.sh arm64    # linux/arm64
OUT=dist/atlas-server ./scripts/build-linux.sh
```

Listens on `:22255` by default. App routes are under **`/atlas`**
(public base `http://10.218.220.237:22255/atlas`).

## Account login (MySQL)

Users are stored in MySQL. **Web login is required** before approving a CLI device code.

1. Open `http://10.218.220.237:22255/atlas/login`
2. Sign in (bootstrap dev user below) or register a new account
3. Open `/atlas/account` to view your **machine code**
4. Complete CLI device login at `/atlas/oauth2/device` with CLI `user_code` + your machine code

Bootstrap dev user (created on first startup if missing):

| Field | Value |
|-------|-------|
| email | `dev@atlas.local` |
| password | `atlas-dev` (override with `ATLAS_BOOTSTRAP_PASSWORD`) |
| user_id | `atlas-local-user` |
| first_name / last_name | Atlas / Dev |

## CLI wiring

```bash
# PowerShell (OAuth issuer + proxy already default to 10.218.220.237 after rebuild)
$env:GROK_LOGIN_DEVICE_FLOW = "1"
# optional: force loopback IdP instead of the default 220.237 issuer
# $env:GROK_LOCAL_AUTH = "1"
atlas login --device-auth
```

1. CLI prints a verification URL + `user_code`
2. **Sign in** at `http://10.218.220.237:22255/atlas/login` if needed
3. Open the device URL, enter CLI `user_code` and your account **machine code**, click **Approve**
4. CLI polls `/atlas/oauth2/token` and writes `~/.atlas/auth.json`

## Environment

Env vars override the config file. Common overrides:

| Variable | Config file key | Description |
|----------|-----------------|-------------|
| `ATLAS_CONFIG` | — | Explicit config file path |
| `ATLAS_MYSQL_DSN` | `[mysql].dsn` | MySQL DSN (overrides structured fields) |
| `ATLAS_BOOTSTRAP_PASSWORD` | `[bootstrap].password` | Default dev user password |
| `ATLAS_GROK_HOME` | `[data].grok_home` | Grok data directory |
| `ATLAS_DOWNLOAD_DIR` | `[data].download_dir` | Probe cache directory |
| `ATLAS_UPSTREAM` | `[upstream].enabled` | `0` to disable upstream proxy |

## Endpoints (phase 1)

All application paths are under `/atlas` (except root `/healthz`).

| Method | Path | Status |
|--------|------|--------|
| GET/POST | `/atlas/login`, `/atlas/register`, `/atlas/logout` | ready (web account) |
| GET | `/atlas/account` | ready (machine code) |
| POST | `/atlas/api/auth/login` | ready |
| GET | `/atlas/api/auth/me` | ready |
| GET | `/atlas/.well-known/openid-configuration` | ready |
| POST | `/atlas/oauth2/device/code` | ready |
| GET/POST | `/atlas/oauth2/device` | ready (login + machine code required) |
| POST | `/atlas/oauth2/token` | ready (device + refresh) |
| GET | `/atlas/v1/user` | ready |
| GET | `/atlas/v1/settings` | ready (`allow_access`) |
| GET | `/atlas/v1/models` | ready |
| POST | `/atlas/v1/responses` | ready (SSE local echo) |
| GET | `/atlas/v1/billing` | stub |
| GET | `/atlas/v1/mcp/configs` | stub |
| GET | `/atlas/v1/feedback/config` | stub |
| GET | `/atlas/v1/bundle/archive` | 404 soft |
| GET | `/atlas/v1/subagents/bundle` | 404 soft |
| GET | `/atlas/v1/skills` | stub (501) |
| GET | `/atlas/cli/{channel}` | channel pointer (plain-text semver: stable/alpha/enterprise) |
| GET | `/atlas/cli/grok-{ver}-{platform}` | CLI binary download |
| GET | `/atlas/cli/install.ps1` | optional bootstrap script (if published into releases/) |
| POST | `/atlas/v1/events` | stub sink |
| POST | `/atlas/v1/traces` | ready (MySQL, by user) |
| POST | `/atlas/v1/task-reports` | ready (MySQL, by user) |
| POST | `/atlas/v1/sessions/{sessionId}/signals` | ready (MySQL session metrics snapshots) |
| GET | `/atlas/admin/api/traces?user_id=` | ready (list by user) |
| GET | `/atlas/admin/api/task-reports?from=&to=` | ready (overall summary + by-user + by-agent; `from`/`to` 默认当天，含首尾；`all` 表示不限日期；兼容旧参 `date=`) |
| GET | `/atlas/admin/api/task-reports?user_id=&email=&limit=&aggregate=1&from=&to=` | ready (list/aggregate by user + date range) |
| GET | `/atlas/admin/task-reports` | ready (HTML dashboard: date range → overall → per-user) |
| GET | `/atlas/admin/api/session-signals?user_id=&session_id=&email=&limit=` | ready (list signal snapshots) |
| GET | `/atlas/admin/api/status` | stub |
| GET | `/healthz`, `/atlas/healthz` | ready |

## Data source

Probe / real-API response files are read from **`<cwd>/download`** by default
(`ATLAS_DOWNLOAD_DIR` overrides; falls back to `<cwd>/downloads` if that exists).

CLI update artifacts are served from **`<cwd>/releases`** (`ATLAS_RELEASES_DIR`
or `[data] releases_dir`). Full guide: **[docs/CLI-发布.md](docs/CLI-发布.md)**.

Remote settings field reference: **[docs/settings.md](docs/settings.md)**
(`GET /atlas/v1/settings` / `download/probe_settings.json`).

```powershell
.\scripts\publish-release.ps1 -Binary path\to\xai-grok-pager.exe -Version 0.2.110
# Linux / macOS 产物加 -Os linux|macos（无 .exe）
```

CLI clients should set:

```toml
[endpoints]
cli_chat_proxy_base_url = "http://10.218.220.237:22255/atlas/v1"
cli_update_base_url = "http://10.218.220.237:22255/atlas/cli"
```

(or `GROK_CLI_BASE_URL` / derive `/cli` from a non-public proxy URL). Public x.ai CDN is not used.

Optional `ATLAS_GROK_HOME` (default `%USERPROFILE%\.grok`) supplies:

- `auth.json` for upstream proxying
- `bundled/` + `skills/` when packing bundles locally

Startup behavior:

1. Prefer `download/probe_*` (models, settings, subagents bundle, archive, …)
2. Else pack `/v1/bundle/*` from `$ATLAS_GROK_HOME/bundled` (+ user `skills/`)
3. When `ATLAS_UPSTREAM=1` (default), proxy `/v1/responses` using `$ATLAS_GROK_HOME/auth.json`

Place or refresh probes:

```powershell
mkdir download
# copy probe_* into .\download  OR:
$env:HTTPS_PROXY="http://127.0.0.1:7890"
python scripts/probe_xai_proxy.py   # writes into ATLAS_DOWNLOAD_DIR / ./download
```

## Layout

```
cmd/server          entrypoint
internal/auth       OIDC + device-code
internal/user       /v1/user
internal/settings   remote settings (stub)
internal/skills     skill delivery (stub)
internal/telemetry  data collection (stub)
internal/admin      ops API (stub)
internal/store      MySQL users/sessions + in-memory OAuth device store
web/login           account login/register HTML
web/account         machine code dashboard
web/device          device approval HTML
```
