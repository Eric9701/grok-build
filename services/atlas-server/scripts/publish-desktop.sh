#!/usr/bin/env bash
# Publish one AtlasBuddy desktop build into releases/desktop/{channel}/.
#
# Same behaviour as publish-desktop.ps1. On macOS pass the built .app and this
# script packs it into <Product>_<ver>_<arch>.app.tar.gz (keeps exec bits and
# symlinks — do not pack .app bundles on Windows).
#
#   ./scripts/publish-desktop.sh --version 0.2.43 --platform darwin-aarch64 \
#       --payload ../../../grok-app/src-tauri/target/release/bundle/macos/AtlasBuddy.app \
#       --installer ../../../grok-app/src-tauri/target/release/bundle/dmg/AtlasBuddy_0.2.43_aarch64.dmg
#
#   ./scripts/publish-desktop.sh --version 0.2.43 --platform windows-x86_64 \
#       --payload ./AtlasBuddy_0.2.43_x64-setup.exe
#
# Requires python3 (manifest merge). manifest.json is written last via rename.
set -euo pipefail

VERSION="" PLATFORM="" PAYLOAD="" INSTALLER="" CHANNEL="stable" NOTES="" MIN_SUPPORTED=""
RELEASES_DIR="$(cd "$(dirname "$0")/.." && pwd)/releases"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --platform) PLATFORM="$2"; shift 2 ;;
    --payload) PAYLOAD="$2"; shift 2 ;;
    --installer) INSTALLER="$2"; shift 2 ;;
    --channel) CHANNEL="$2"; shift 2 ;;
    --notes) NOTES="$2"; shift 2 ;;
    --min-supported) MIN_SUPPORTED="$2"; shift 2 ;;
    --releases-dir) RELEASES_DIR="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 1 ;;
  esac
done

die() { echo "error: $*" >&2; exit 1; }

[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-+][0-9A-Za-z.-]+)?$ ]] || die "--version must be semver, got '$VERSION'"
case "$PLATFORM" in windows-x86_64|windows-x86_64-portable|darwin-aarch64|darwin-x86_64) ;; *) die "--platform must be windows-x86_64|windows-x86_64-portable|darwin-aarch64|darwin-x86_64" ;; esac
case "$CHANNEL" in stable|alpha|enterprise) ;; *) die "--channel must be stable|alpha|enterprise" ;; esac
[[ -e "$PAYLOAD" ]] || die "payload not found: $PAYLOAD"
command -v python3 >/dev/null || die "python3 is required"

CHANNEL_DIR="$RELEASES_DIR/desktop/$CHANNEL"
mkdir -p "$CHANNEL_DIR"

safe_name() { [[ "$1" =~ ^[A-Za-z0-9._-]+$ && "$1" != *..* ]] || die "file name must match [A-Za-z0-9._-]+ : $1"; }

copy_artifact() {
  local src="$1" name
  name="$(basename "$src")"
  safe_name "$name"
  cp -f "$src" "$CHANNEL_DIR/$name.uploading"
  mv -f "$CHANNEL_DIR/$name.uploading" "$CHANNEL_DIR/$name"
  echo "$name"
}

sha256_of() {
  if command -v shasum >/dev/null; then shasum -a 256 "$1" | awk '{print $1}'; else sha256sum "$1" | awk '{print $1}'; fi
}

case "$PLATFORM" in
  windows-x86_64)
    [[ "$PAYLOAD" == *-setup.exe ]] || die "Windows payload must be the NSIS *-setup.exe"
    KIND="nsis"
    PAYLOAD_FILE="$(copy_artifact "$PAYLOAD")"
    [[ -n "$INSTALLER" ]] || INSTALLER="$PAYLOAD"
    ;;
  windows-x86_64-portable)
    [[ "$PAYLOAD" == *-portable.zip ]] || die "Windows portable payload must be the *-portable.zip"
    KIND="portable_zip"
    PAYLOAD_FILE="$(copy_artifact "$PAYLOAD")"
    [[ -n "$INSTALLER" ]] || INSTALLER="$PAYLOAD"
    ;;
  darwin-*)
    KIND="app_tar_gz"
    ARCH="${PLATFORM#darwin-}"
    [[ "$ARCH" == "x86_64" ]] && ARCH="x64"
    if [[ -d "$PAYLOAD" && "$PAYLOAD" == *.app ]]; then
      [[ "$(uname -s)" == "Darwin" ]] || die "pack .app bundles on macOS"
      APP_NAME="$(basename "$PAYLOAD" .app)"
      PAYLOAD_FILE="${APP_NAME}_${VERSION}_${ARCH}.app.tar.gz"
      safe_name "$PAYLOAD_FILE"
      # Archive root is the .app directory itself.
      COPYFILE_DISABLE=1 tar -czf "$CHANNEL_DIR/$PAYLOAD_FILE.uploading" -C "$(dirname "$PAYLOAD")" "$(basename "$PAYLOAD")"
      mv -f "$CHANNEL_DIR/$PAYLOAD_FILE.uploading" "$CHANNEL_DIR/$PAYLOAD_FILE"
    elif [[ "$PAYLOAD" == *.app.tar.gz ]]; then
      PAYLOAD_FILE="$(copy_artifact "$PAYLOAD")"
    else
      die "macOS payload must be a .app directory or .app.tar.gz"
    fi
    ;;
esac

[[ "$PAYLOAD_FILE" == *"_${VERSION}_"* ]] || die "payload file name must contain _${VERSION}_ (got $PAYLOAD_FILE)"

SHA="$(sha256_of "$CHANNEL_DIR/$PAYLOAD_FILE")"
SIZE="$(wc -c < "$CHANNEL_DIR/$PAYLOAD_FILE" | tr -d ' ')"

INSTALLER_FILE=""
if [[ -n "$INSTALLER" ]]; then
  [[ -f "$INSTALLER" ]] || die "installer not found: $INSTALLER"
  if [[ "$(cd "$(dirname "$INSTALLER")" && pwd)/$(basename "$INSTALLER")" == "$CHANNEL_DIR/$PAYLOAD_FILE" ]]; then
    INSTALLER_FILE="$PAYLOAD_FILE"
  else
    INSTALLER_FILE="$(copy_artifact "$INSTALLER")"
  fi
fi

MANIFEST="$CHANNEL_DIR/manifest.json" \
VERSION="$VERSION" PLATFORM="$PLATFORM" KIND="$KIND" PAYLOAD_FILE="$PAYLOAD_FILE" \
SHA="$SHA" SIZE="$SIZE" INSTALLER_FILE="$INSTALLER_FILE" NOTES="$NOTES" MIN_SUPPORTED="$MIN_SUPPORTED" \
python3 - <<'PY'
import json, os, datetime
path = os.environ["MANIFEST"]
version = os.environ["VERSION"]
old = {}
if os.path.exists(path):
    with open(path, encoding="utf-8") as f:
        old = json.load(f)
same = old.get("version") == version
platforms = dict(old.get("platforms") or {}) if same else {}
installers = dict(old.get("installers") or {}) if same else {}
platforms[os.environ["PLATFORM"]] = {
    "kind": os.environ["KIND"],
    "file": os.environ["PAYLOAD_FILE"],
    "sha256": os.environ["SHA"].lower(),
    "size": int(os.environ["SIZE"]),
}
if os.environ["INSTALLER_FILE"]:
    installers[os.environ["PLATFORM"]] = os.environ["INSTALLER_FILE"]
manifest = {
    "version": version,
    "notes": os.environ["NOTES"] or (old.get("notes", "") if same else ""),
    "pub_date": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "min_supported": os.environ["MIN_SUPPORTED"] or (old.get("min_supported", "") if same else ""),
    "platforms": platforms,
    "installers": installers,
}
tmp = path + ".tmp"
with open(tmp, "w", encoding="utf-8") as f:
    json.dump(manifest, f, ensure_ascii=False, indent=2)
os.replace(tmp, path)
PY

echo "Published $PLATFORM $VERSION ($KIND) -> $PAYLOAD_FILE"
echo "sha256 $SHA  size $SIZE"
echo "Check: GET {public_base_url}/desktop/$CHANNEL/update.json"
