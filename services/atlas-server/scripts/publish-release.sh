#!/usr/bin/env bash
# Publish Atlas CLI binary into atlas-server releases/ layout.
#
# Usage (from services/atlas-server):
#   ./scripts/publish-release.sh --binary ../../target/release/xai-grok-pager --version 0.2.110
#   ./scripts/publish-release.sh --binary ./atlas --version 0.2.110 --os linux --arch x86_64 --channel enterprise

set -euo pipefail

BINARY=""
VERSION=""
CHANNEL="stable"
RELEASES_DIR=""
OS=""
ARCH=""

usage() {
    cat <<'EOF'
Usage: publish-release.sh --binary PATH --version X.Y.Z [options]

  --binary PATH       CLI binary to copy
  --version X.Y.Z     semver
  --channel NAME      stable | alpha | enterprise (default: stable)
  --os NAME           windows | linux | macos (default: host / .exe)
  --arch NAME         x86_64 | aarch64 (default: host)
  --releases-dir DIR  default: ../releases relative to this script
EOF
    exit 1
}

while [ $# -gt 0 ]; do
    case "$1" in
        --binary) BINARY="${2:-}"; shift 2 ;;
        --version) VERSION="${2:-}"; shift 2 ;;
        --channel) CHANNEL="${2:-}"; shift 2 ;;
        --os) OS="${2:-}"; shift 2 ;;
        --arch) ARCH="${2:-}"; shift 2 ;;
        --releases-dir) RELEASES_DIR="${2:-}"; shift 2 ;;
        -h|--help) usage ;;
        *) echo "unknown arg: $1" >&2; usage ;;
    esac
done

[ -n "$BINARY" ] && [ -n "$VERSION" ] || usage
[ -f "$BINARY" ] || { echo "Binary not found: $BINARY" >&2; exit 1; }

if ! printf '%s' "$VERSION" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+'; then
    echo "Version must look like semver (e.g. 0.2.110), got: $VERSION" >&2
    exit 1
fi

case "$CHANNEL" in
    stable|alpha|enterprise) ;;
    *) echo "Channel must be stable, alpha, or enterprise" >&2; exit 1 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if [ -z "$RELEASES_DIR" ]; then
    RELEASES_DIR="$(cd "$SCRIPT_DIR/.." && pwd)/releases"
fi
mkdir -p "$RELEASES_DIR"

if [ -z "$OS" ]; then
    case "$BINARY" in
        *.exe) OS="windows" ;;
        *)
            case "$(uname -s)" in
                Darwin) OS="macos" ;;
                Linux) OS="linux" ;;
                MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
                *) echo "Unsupported OS $(uname -s); pass --os" >&2; exit 1 ;;
            esac
            ;;
    esac
fi
case "$OS" in
    windows|linux|macos) ;;
    *) echo "os must be windows, linux, or macos" >&2; exit 1 ;;
esac

if [ -z "$ARCH" ]; then
    case "$(uname -m)" in
        x86_64|amd64|AMD64) ARCH="x86_64" ;;
        arm64|aarch64|ARM64) ARCH="aarch64" ;;
        *) echo "Unsupported arch $(uname -m); pass --arch" >&2; exit 1 ;;
    esac
fi
case "$ARCH" in
    x86_64|aarch64) ;;
    *) echo "arch must be x86_64 or aarch64" >&2; exit 1 ;;
esac

PLATFORM="${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
    ARTIFACT="grok-${VERSION}-${PLATFORM}.exe"
else
    ARTIFACT="grok-${VERSION}-${PLATFORM}"
fi

cp -f "$BINARY" "$RELEASES_DIR/$ARTIFACT"
printf '%s' "$VERSION" > "$RELEASES_DIR/$CHANNEL"

PAGER_SCRIPTS="$SCRIPT_DIR/../../../crates/codegen/xai-grok-pager/scripts"
for name in install.ps1 install-enterprise.ps1 install.sh install-enterprise.sh; do
    if [ -f "$PAGER_SCRIPTS/$name" ]; then
        cp -f "$PAGER_SCRIPTS/$name" "$RELEASES_DIR/$name"
    fi
done

echo "Published $ARTIFACT"
echo "Channel $CHANNEL -> $VERSION"
echo "Dir: $RELEASES_DIR"
