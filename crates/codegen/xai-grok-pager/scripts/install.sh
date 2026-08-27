#!/bin/bash
#
# Atlas CLI installer — served from atlas-server /atlas/cli/install.sh
#
# Auth: GROK_DEPLOYMENT_KEY (takes precedence) or ~/.atlas/auth.json from `atlas login`.
# Env: GROK_CHANNEL, GROK_BIN_DIR, GROK_PROXY_URL, GROK_CLI_BASE_URL,
#      GROK_ATLAS_SERVER (default: http://10.218.220.237:22255),
#      GROK_PLUGIN_MARKETPLACE, GROK_SKIP_ATLAS_SDD=1, GROK_SKIP_ATLAS_SKILLS=1
#      (default CLI base: ${GROK_ATLAS_SERVER}/atlas/cli)
#
# Usage:
#   curl -fsSL http://10.218.220.237:22255/atlas/cli/install.sh | bash
#   curl -fsSL http://10.218.220.237:22255/atlas/cli/install.sh | bash -s 0.2.120
#
# Windows: run under Git for Windows / MSYS2 Bash (same curl | bash flow); WSL
# uses the Linux binary.

set -e

TARGET="$1"

if [[ -n "$TARGET" ]] && [[ ! "$TARGET" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9._]+)?$ ]]; then
    echo "Invalid version format: $TARGET (expected X.Y.Z or X.Y.Z-suffix)" >&2
    exit 1
fi

DOWNLOADER=""
if command -v curl >/dev/null 2>&1; then
    DOWNLOADER="curl"
elif command -v wget >/dev/null 2>&1; then
    DOWNLOADER="wget"
else
    echo "Either curl or wget is required but neither is installed" >&2
    exit 1
fi

download_file() {
    local url="$1" output="$2"
    if [ "$DOWNLOADER" = "curl" ]; then
        if [ -n "$output" ]; then
            curl -fsSL -o "$output" "$url"
        else
            curl -fsSL "$url"
        fi
    else
        if [ -n "$output" ]; then
            wget -q -O "$output" "$url"
        else
            wget -q -O - "$url"
        fi
    fi
}

# Parallel byte-range download. Falls back to single-connection download_file
# whenever HEAD lacks Content-Length, the file is small (<16 MiB), curl is
# unavailable, or any chunk fetch / concat fails.
download_file_parallel() {
    local url="$1" output="$2"
    if [ "$DOWNLOADER" != "curl" ]; then
        download_file "$url" "$output"
        return
    fi
    local size
    size=$(curl -fsSL --head "$url" 2>/dev/null | awk -F'[: \r\n]+' 'tolower($1)=="content-length"{print $2; exit}')
    if [ -z "$size" ] || ! [ "$size" -ge 16777216 ] 2>/dev/null; then
        download_file "$url" "$output"
        return
    fi
    local n=8
    local chunk_size=$(( (size + n - 1) / n ))
    local tmpdir
    tmpdir=$(mktemp -d 2>/dev/null) || { download_file "$url" "$output"; return; }
    local pids=() i start end
    for i in $(seq 0 $((n - 1))); do
        start=$((i * chunk_size))
        end=$((start + chunk_size - 1))
        [ $end -ge $size ] && end=$((size - 1))
        curl -fsSL -r "${start}-${end}" -o "${tmpdir}/$(printf 'chunk.%03d' "$i")" "$url" &
        pids+=($!)
    done
    local all_ok=true pid
    for pid in "${pids[@]}"; do
        wait "$pid" || all_ok=false
    done
    if [ "$all_ok" = true ] && cat "${tmpdir}"/chunk.* > "$output" 2>/dev/null; then
        rm -rf "$tmpdir"
        return 0
    fi
    rm -rf "$tmpdir"
    download_file "$url" "$output"
}

# Return 0 if a HEAD request for the URL gets HTTP 404.
is_not_found() {
    local url="$1" code
    if [ "$DOWNLOADER" = "curl" ]; then
        code=$(curl -o /dev/null -sSL -w '%{http_code}' --head "$url" 2>/dev/null) || true
    else
        code=$(wget --server-response --spider "$url" 2>&1 | awk '/HTTP\//{print $2}' | tail -1) || true
    fi
    [ "$code" = "404" ]
}

# JSON field extractor ??extract a top-level string value using sed.
json_get() {
    local json="$1" field="$2"
    # Extract value (handling \" inside strings), then unescape JSON sequences.
    printf '%s' "$json" | sed -n -E 's/.*"'"$field"'"[[:space:]]*:[[:space:]]*"(([^"\\]|\\.)*)".*/\1/p' | head -1 \
        | sed -e 's/\\"/"/g' -e 's/\\n/\'$'\n''/g' -e 's/\\t/\'$'\t''/g' -e 's/\\\\/\\/g'
}

# Read a token from ~/.atlas/auth.json for the given scope key.
# Format: {"scope_url": {"key": "token"}, ...}
read_atlas_token() {
    local auth_file="$HOME/.atlas/auth.json"
    local scope="$1"
    [ -f "$auth_file" ] || return 1
    # Flatten to one line then extract: find the scope, then the "key" value after it
    tr -d '\n' < "$auth_file" | sed -n 's|.*"'"$scope"'"[[:space:]]*:[[:space:]]*{[^}]*"key"[[:space:]]*:[[:space:]]*"\([^"]*\)".*|\1|p' | head -1
}

# Resolve auth: GROK_DEPLOYMENT_KEY > OIDC token > legacy token
OIDC_SCOPE="https://auth.x.ai::b1a00492-073a-47ea-816f-4c329264a828"
LEGACY_SCOPE="https://accounts.x.ai/sign-in"
AUTH_SOURCE=""

if [ -n "$GROK_DEPLOYMENT_KEY" ]; then
    AUTH_SOURCE="deployment key"
    echo "Auth: using deployment key." >&2
else
    OIDC_TOKEN=$(read_atlas_token "$OIDC_SCOPE" 2>/dev/null) || true
    LEGACY_TOKEN=$(read_atlas_token "$LEGACY_SCOPE" 2>/dev/null) || true
    if [ -n "$OIDC_TOKEN" ]; then
        AUTH_SOURCE="auth.json (oidc)"
        echo "Auth: using OIDC token from ~/.atlas/auth.json." >&2
    elif [ -n "$LEGACY_TOKEN" ]; then
        AUTH_SOURCE="auth.json (legacy)"
        echo "Auth: using legacy token from ~/.atlas/auth.json." >&2
    fi
fi

case "$(uname -s)" in
    Darwin) os="macos" ;;
    Linux)  os="linux" ;;
    # Git for Windows / MSYS2 / Cygwin host ??native Windows builds
    MINGW* | MSYS* | CYGWIN*) os="windows" ;;
    *)      echo "Unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
    x86_64|amd64|AMD64) arch="x86_64" ;;
    arm64|aarch64|ARM64) arch="aarch64" ;;
    *)                    echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

# Rosetta lies: in a translated shell on Apple Silicon, uname -m reports
# x86_64. Install the native arm64 build (faster startup, no translation).
# sysctl lives in /usr/sbin, which pruned PATHs often drop — resolve the
# binary first (PATH, then absolute) so the probe cannot quietly keep
# x86_64. A probe that runs and finds no key is a genuine Intel Mac.
if [ "$os" = "macos" ] && [ "$arch" = "x86_64" ]; then
    sysctl_bin="$(command -v sysctl || echo /usr/sbin/sysctl)"
    if [ "$("$sysctl_bin" -n hw.optional.arm64 2>/dev/null)" = "1" ]; then
        echo "Apple Silicon detected (Rosetta shell); installing the native arm64 build." >&2
        arch="aarch64"
    fi
fi

ATLAS_SERVER="${GROK_ATLAS_SERVER:-http://10.218.220.237:22255}"
ATLAS_SERVER="${ATLAS_SERVER%/}"
BASE_URL="${GROK_CLI_BASE_URL:-${ATLAS_SERVER}/atlas/cli}"
BASE_URL="${BASE_URL%/}"
PROXY_URL_DEFAULT="${ATLAS_SERVER}/atlas/v1"
DOWNLOAD_DIR="$HOME/.atlas/downloads"
BIN_DIR="${GROK_BIN_DIR:-$HOME/.atlas/bin}"
mkdir -p "$DOWNLOAD_DIR" "$BIN_DIR"

platform="${os}-${arch}"
CHANNEL="${GROK_CHANNEL:-stable}"

if [ -z "$TARGET" ]; then echo "Fetching latest ${CHANNEL} version from ${BASE_URL}..." >&2; fi
probe_result=$(download_file "${BASE_URL}/${CHANNEL}" 2>/dev/null) || true

if [ -n "$TARGET" ]; then
    version="$TARGET"
else
    version=$(printf '%s' "$probe_result" | tr -d '\r' | head -n1 | tr -d '[:space:]')
    if [ -z "$version" ]; then
        echo "Error: failed to fetch latest version from ${BASE_URL}/${CHANNEL}" >&2
        exit 1
    fi
fi

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9._]+)?$ ]]; then
    echo "Invalid version format: $version (expected X.Y.Z or X.Y.Z-suffix)" >&2
    exit 1
fi

if [ -n "$AUTH_SOURCE" ]; then
    echo "Installing Grok $version ($platform, $AUTH_SOURCE)..." >&2
else
    echo "Installing Grok $version ($platform)..." >&2
fi

binary_path="$DOWNLOAD_DIR/grok-$platform"
artifact_base="${BASE_URL}/grok-${version}-${platform}"

if [ "$os" = "windows" ]; then
    binary_path="${binary_path}.exe"
fi

binary_tmp="${binary_path}.tmp.$$"
rm -f "$binary_tmp" 2>/dev/null || true

echo "  Downloading grok ${version}..." >&2
if [ "$os" = "windows" ]; then
    if ! download_file_parallel "${artifact_base}.exe" "$binary_tmp"; then
        if ! download_file_parallel "$artifact_base" "$binary_tmp"; then
            rm -f "$binary_tmp"
            if is_not_found "${artifact_base}.exe"; then
                echo "Error: Grok is not yet available for your system ($platform)." >&2
            else
                echo "Error: binary download failed (${artifact_base}.exe and ${artifact_base})" >&2
            fi
            exit 1
        fi
    fi
elif ! download_file_parallel "$artifact_base" "$binary_tmp"; then
    rm -f "$binary_tmp"
    if is_not_found "$artifact_base"; then
        echo "Error: Grok is not yet available for your system ($platform)." >&2
    else
        echo "Error: binary download failed from ${artifact_base}" >&2
    fi
    exit 1
fi

if [ "$os" = "windows" ]; then
    mv -f "$binary_tmp" "$binary_path"
    # Symlinks require Developer Mode on Windows; copy instead.
    # If the exe is locked by a running process, rename it aside then retry.
    for bin_name in atlas.exe agent.exe; do
        rm -f "$BIN_DIR/$bin_name.old" 2>/dev/null || true  # stale backup from prior update
        if ! cp -f "$binary_path" "$BIN_DIR/$bin_name" 2>/dev/null; then
            mv -f "$BIN_DIR/$bin_name" "$BIN_DIR/$bin_name.old" 2>/dev/null || true
            if ! cp -f "$binary_path" "$BIN_DIR/$bin_name" 2>/dev/null; then
                # Rollback: restore the old binary so the install isn't broken.
                mv -f "$BIN_DIR/$bin_name.old" "$BIN_DIR/$bin_name" 2>/dev/null || true
                echo "Error: failed to install $bin_name" >&2
                exit 1
            fi
        fi
    done
    echo "  Binary installed to $BIN_DIR/atlas.exe and $BIN_DIR/agent.exe." >&2
else
    chmod +x "$binary_tmp"
    if ! "$binary_tmp" --version </dev/null >/dev/null 2>&1; then
        echo "Error: downloaded grok failed to run; keeping the existing install." >&2
        rm -f "$binary_tmp"
        exit 1
    fi
    mv -f "$binary_tmp" "$binary_path"
    # Use relative symlinks when BIN_DIR and DOWNLOAD_DIR share a parent
    # (default layout: ~/.atlas/bin and ~/.atlas/downloads are siblings).
    # Relative symlinks survive Docker bind-mounts with a different $HOME.
    if [ "$(dirname "$BIN_DIR")" = "$(dirname "$DOWNLOAD_DIR")" ]; then
        link_target="../$(basename "$DOWNLOAD_DIR")/$(basename "$binary_path")"
    else
        link_target="$binary_path"
    fi
    ln -sf "$link_target" "$BIN_DIR/atlas"
    ln -sf "$link_target" "$BIN_DIR/agent"
    echo "  Binary linked to $BIN_DIR/atlas and $BIN_DIR/agent." >&2
fi

# Generate shell completions (best-effort)
mkdir -p "$HOME/.atlas/completions/bash" "$HOME/.atlas/completions/zsh"
"$BIN_DIR/atlas" completions bash > "$HOME/.atlas/completions/bash/atlas.bash" 2>/dev/null || true
"$BIN_DIR/atlas" completions zsh  > "$HOME/.atlas/completions/zsh/_atlas"     2>/dev/null || true
# Fish: write to the auto-loaded completions dir so it works immediately
if mkdir -p "$HOME/.config/fish/completions" 2>/dev/null; then
    "$BIN_DIR/atlas" completions fish > "$HOME/.config/fish/completions/atlas.fish" 2>/dev/null || true
fi

# Persist installer source, channel, and enterprise endpoints
CONFIG_FILE="$HOME/.atlas/config.toml"
CLI_BLOCK="installer = \"internal\""
if [ "$CHANNEL" != "stable" ]; then
    CLI_BLOCK="${CLI_BLOCK}\nchannel = \"${CHANNEL}\""
fi
ENDPOINTS_BLOCK="cli_chat_proxy_base_url = \"${PROXY_URL_DEFAULT}\"\ncli_update_base_url = \"${BASE_URL}\""

upsert_toml_section() {
    local file="$1" section="$2" keys_regex="$3" block="$4"
    if [ ! -f "$file" ]; then
        printf '[%s]\n%b\n' "$section" "$block" > "$file"
        return
    fi
    if ! grep -q "^\[${section}\]" "$file"; then
        printf '\n[%s]\n%b\n' "$section" "$block" >> "$file"
        return
    fi
    local tmp="$file.tmp.$$"
    awk -v section="$section" -v block="$block" -v keys_regex="$keys_regex" '
        $0 ~ "^\\[" section "\\][[:space:]]*(#.*)?$" {
            print; printf "%s\n", block; in_sec=1; next
        }
        /^\[.*\][[:space:]]*(#.*)?$/ { in_sec=0 }
        in_sec && $0 ~ "^[[:space:]]*(" keys_regex ")[[:space:]]*=" { next }
        { print }
    ' "$file" > "$tmp" && mv "$tmp" "$file"
}

upsert_toml_section "$CONFIG_FILE" "cli" "installer|channel" "$CLI_BLOCK"
upsert_toml_section "$CONFIG_FILE" "endpoints" "cli_chat_proxy_base_url|cli_update_base_url" "$ENDPOINTS_BLOCK"

# Fetch managed_config.toml + requirements.toml from server (deployment key only).
if [ -n "$GROK_DEPLOYMENT_KEY" ]; then
    PROXY_URL="${GROK_PROXY_URL:-$PROXY_URL_DEFAULT}"
    # Refuse userinfo / empty-host proxies before attaching the key.
    # Atlas enterprise may use an internal http atlas-server; https is not required.
    proxy_authority="${PROXY_URL#*://}"
    proxy_authority="${proxy_authority%%[/?#]*}"
    proxy_ok=
    case "$PROXY_URL" in
        [hH][tT][tT][pP][sS]://*|[hH][tT][tT][pP]://*)
            case "$proxy_authority" in
                ""|*@*) ;;
                *) proxy_ok=1 ;;
            esac
            ;;
    esac
    if [ -z "$proxy_ok" ]; then
        echo "Error: GROK_PROXY_URL must be an http:// or https:// URL." >&2
        exit 1
    fi
    echo "  Fetching deployment config..." >&2
    DEPLOY_RESPONSE=""
    AUTH_HEADER_FILE=$(mktemp 2>/dev/null) || AUTH_HEADER_FILE=""
    if [ -n "$AUTH_HEADER_FILE" ]; then
        chmod 600 "$AUTH_HEADER_FILE" 2>/dev/null || true
        printf 'Authorization: Bearer %s\n' "$GROK_DEPLOYMENT_KEY" > "$AUTH_HEADER_FILE"
        DEPLOY_RESPONSE=$(curl -sS -f --proto '=https' \
            -H "@${AUTH_HEADER_FILE}" \
            "${PROXY_URL}/deployment/config" 2>/dev/null) || DEPLOY_RESPONSE=""
        : > "$AUTH_HEADER_FILE" 2>/dev/null || true
        rm -f "$AUTH_HEADER_FILE"
    fi
    if [ -z "$DEPLOY_RESPONSE" ]; then
        echo "  Warning: failed to fetch deployment config from ${PROXY_URL}/deployment/config" >&2
    fi
    if [ -n "$DEPLOY_RESPONSE" ]; then
        MANAGED_CONFIG=$(json_get "$DEPLOY_RESPONSE" "managed_config")
        REQUIREMENTS=$(json_get "$DEPLOY_RESPONSE" "requirements")
        if [ -n "$MANAGED_CONFIG" ] && [ "$MANAGED_CONFIG" != "null" ]; then
            printf '%s\n' "$MANAGED_CONFIG" > "$HOME/.atlas/managed_config.toml"
            echo "  Managed config applied." >&2
        else
            rm -f "$HOME/.atlas/managed_config.toml"
        fi
        if [ -n "$REQUIREMENTS" ] && [ "$REQUIREMENTS" != "null" ]; then
            printf '%s\n' "$REQUIREMENTS" > "$HOME/.atlas/requirements.toml"
            echo "  Requirements applied." >&2
        else
            rm -f "$HOME/.atlas/requirements.toml"
        fi
    fi
fi

if [ "$os" = "windows" ]; then
    echo "Atlas $version installed to $BIN_DIR/atlas.exe" >&2
else
    echo "Atlas $version installed to $BIN_DIR/atlas" >&2
fi

# --- Ensure grok is on PATH ---

path_has_dir() {
    case ":$PATH:" in *":$1:"*) return 0 ;; *) return 1 ;; esac
}

# Try to symlink into a directory already on PATH so grok works immediately
# without restarting the shell. Candidate dirs in preference order.
SYMLINK_CREATED=""
if [ "$os" != "windows" ] && ! path_has_dir "$BIN_DIR"; then
    for candidate in "$HOME/.local/bin" "/usr/local/bin"; do
        if path_has_dir "$candidate" && [ -d "$candidate" ] && [ -w "$candidate" ]; then
            ln -sf "$BIN_DIR/atlas" "$candidate/atlas"
            ln -sf "$BIN_DIR/agent" "$candidate/agent"
            SYMLINK_CREATED="$candidate"
            echo "  Symlinked $candidate/atlas -> $BIN_DIR/atlas" >&2
            echo "  Symlinked $candidate/agent -> $BIN_DIR/agent" >&2
            break
        fi
    done
fi

# Also update shell config so ~/.atlas/bin is on PATH for future sessions
user_shell="$(basename "${SHELL:-}")"
config_file=""

case "$user_shell" in
    bash) config_file="$HOME/.bashrc" ;;
    zsh)  config_file="$HOME/.zshrc" ;;
    fish) config_file="$HOME/.config/fish/config.fish" ;;
esac

if [ -n "$config_file" ]; then
    mkdir -p "$(dirname "$config_file")"

    # Resolve symlinks so tmp+mv rewrites the stow/dotfiles target, not the link.
    if [ -e "$config_file" ] || [ -L "$config_file" ]; then
        _cf="$config_file"
        _depth=0
        while [ -L "$_cf" ] && [ "$_depth" -lt 40 ]; do
            _link="$(readlink "$_cf")" || break
            case "$_link" in
                /*) _cf="$_link" ;;
                *)  _cf="$(cd "$(dirname "$_cf")" && pwd -P)/$_link" ;;
            esac
            _depth=$((_depth + 1))
        done
        # Still a symlink (cycle/cap): leave original path so we never rewrite the link.
        if [ ! -L "$_cf" ]; then
            config_file="$(cd "$(dirname "$_cf")" && pwd -P)/$(basename "$_cf")"
        fi
        unset _cf _link _depth
    fi

    # Build the new installer block
    if [ "$user_shell" = "fish" ]; then
        new_block='# >>> grok installer >>>
fish_add_path $HOME/.atlas/bin
# <<< grok installer <<<'
    elif [ "$user_shell" = "zsh" ]; then
        new_block='# >>> grok installer >>>
export PATH="$HOME/.atlas/bin:$PATH"
fpath=(~/.atlas/completions/zsh $fpath)
autoload -Uz compinit && compinit -C
# <<< grok installer <<<'
    else
        new_block='# >>> grok installer >>>
export PATH="$HOME/.atlas/bin:$PATH"
[[ -r "$HOME/.atlas/completions/bash/atlas.bash" ]] && source "$HOME/.atlas/completions/bash/atlas.bash"
# <<< grok installer <<<'
    fi

    if grep -qs "grok installer" "$config_file" 2>/dev/null; then
        # Replace existing block in-place (strip old >>> to <<< lines, insert new)
        tmp="$config_file.tmp.$$"
        awk '
            /# >>> grok installer >>>/ { skip=1; next }
            /# <<< grok installer <<</ { skip=0; next }
            !skip { print }
        ' "$config_file" > "$tmp" && mv "$tmp" "$config_file"
    else
        [ -f "$config_file" ] && cp "$config_file" "$config_file.bak.$(date +%s)"

        # macOS bash: ensure bash_profile sources bashrc
        if [ "$user_shell" = "bash" ] && [ "$(uname -s)" = "Darwin" ]; then
            if [ -f "$HOME/.bash_profile" ] && ! grep -qs "source ~/.bashrc" "$HOME/.bash_profile"; then
                printf '\n[[ -r ~/.bashrc ]] && source ~/.bashrc\n' >> "$HOME/.bash_profile"
            fi
        fi
    fi

    printf '\n%s\n' "$new_block" >> "$config_file"
    echo "  Updated $BIN_DIR in PATH in $config_file." >&2
fi

echo "" >&2

# Install atlas-sdd + atlas-skills plugins (best-effort)
install_atlas_marketplace_plugin() {
    local name="$1"
    if ! "$ATLAS_BIN" plugin install "$name" --trust; then
        "$ATLAS_BIN" plugin install "${name}@git/atlas-plugins" --trust || \
            echo "  Warning: ${name} install failed. Later: atlas plugin install ${name} --trust" >&2
    fi
    echo "  ${name} plugin install attempted (new session required to load)." >&2
}

SKIP_SDD=0
SKIP_SKILLS=0
case "${GROK_SKIP_ATLAS_SDD:-}" in 1|true|yes|YES|True) SKIP_SDD=1 ;; esac
case "${GROK_SKIP_ATLAS_SKILLS:-}" in 1|true|yes|YES|True) SKIP_SKILLS=1 ;; esac

if [ "$SKIP_SDD" -eq 0 ] || [ "$SKIP_SKILLS" -eq 0 ]; then
    MARKETPLACE="${GROK_PLUGIN_MARKETPLACE:-https://gitlab.imyai.cn/zhangyufeng/atlas-plugins.git}"
    ATLAS_BIN="$BIN_DIR/atlas"
    [ "$os" = "windows" ] && ATLAS_BIN="$BIN_DIR/atlas.exe"
    echo "Adding plugin marketplace ${MARKETPLACE}..." >&2
    "$ATLAS_BIN" plugin marketplace add "$MARKETPLACE" >/dev/null 2>&1 || \
        echo "  Warning: marketplace add failed (may already exist)." >&2
    if [ "$SKIP_SDD" -eq 0 ]; then
        echo "Installing atlas-sdd plugin..." >&2
        install_atlas_marketplace_plugin atlas-sdd
    else
        echo "  Skipped atlas-sdd (GROK_SKIP_ATLAS_SDD set)." >&2
    fi
    if [ "$SKIP_SKILLS" -eq 0 ]; then
        echo "Installing atlas-skills plugin..." >&2
        install_atlas_marketplace_plugin atlas-skills
    else
        echo "  Skipped atlas-skills (GROK_SKIP_ATLAS_SKILLS set)." >&2
    fi
fi

echo "Update base: ${BASE_URL}" >&2
echo "Chat proxy:  ${PROXY_URL_DEFAULT}" >&2

if path_has_dir "$BIN_DIR" || [ -n "$SYMLINK_CREATED" ]; then
    echo "Run 'atlas' or 'agent' to get started!" >&2
elif [ -n "$config_file" ]; then
    echo "Restart your terminal, then run 'atlas' or 'agent' to get started!" >&2
else
    echo "Add $BIN_DIR to your PATH, then run 'atlas' or 'agent' to get started:" >&2
    echo '  export PATH="$HOME/.atlas/bin:$PATH"' >&2
fi

if [ "$os" = "windows" ]; then
    echo "To use atlas from cmd.exe or PowerShell, add %USERPROFILE%\\.atlas\\bin to your PATH." >&2
fi
