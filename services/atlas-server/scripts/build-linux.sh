#!/usr/bin/env bash
# Build atlas-server for Linux.
#
# Usage:
#   ./scripts/build-linux.sh              # linux/amd64 → ./atlas-server
#   ./scripts/build-linux.sh arm64        # linux/arm64 → ./atlas-server
#   OUT=dist/atlas-server ./scripts/build-linux.sh
#
# Requires: Go 1.25+ on PATH.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ARCH="${1:-amd64}"
case "$ARCH" in
  amd64|arm64|386) ;;
  *)
    echo "unsupported arch: $ARCH (use amd64, arm64, or 386)" >&2
    exit 1
    ;;
esac

OUT="${OUT:-$ROOT/atlas-server}"
export GOOS=linux
export GOARCH="$ARCH"
export CGO_ENABLED=0

echo "building atlas-server (GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=$CGO_ENABLED) → $OUT"
go build -trimpath -ldflags="-s -w" -o "$OUT" ./cmd/server

if [[ ! -f "$ROOT/atlas-server.toml" ]]; then
  EXAMPLE=""
  for candidate in \
    "$ROOT/atlas-server.toml.example" \
    "$ROOT/atlas-server.toml copy.example"
  do
    if [[ -f "$candidate" ]]; then
      EXAMPLE="$candidate"
      break
    fi
  done
  if [[ -n "$EXAMPLE" ]]; then
    cp "$EXAMPLE" "$ROOT/atlas-server.toml"
    echo "created atlas-server.toml from $(basename "$EXAMPLE") — edit MySQL settings before production use."
  fi
fi

echo "ok: $OUT ($(du -h "$OUT" | awk '{print $1}'))"
echo "run:  $OUT"
echo "      # or: ATLAS_CONFIG=$ROOT/atlas-server.toml $OUT"
