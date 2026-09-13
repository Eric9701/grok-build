#!/usr/bin/env bash
# Build atlas-relay-demo for Linux (static, CGO off).
#
# Usage:
#   ./scripts/build-linux.sh              # linux/amd64 → ./atlas-relay-demo
#   ./scripts/build-linux.sh arm64        # linux/arm64
#   OUT=dist/atlas-relay-demo ./scripts/build-linux.sh
#
# On Windows (PowerShell, Go on PATH):
#   $env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
#   go build -trimpath -ldflags='-s -w' -o atlas-relay-demo .
#
# Requires: Go 1.22+ on PATH.

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

OUT="${OUT:-$ROOT/atlas-relay-demo}"
export GOOS=linux
export GOARCH="$ARCH"
export CGO_ENABLED=0

echo "building atlas-relay-demo (GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=$CGO_ENABLED) → $OUT"
go build -trimpath -ldflags="-s -w" -o "$OUT" .

echo "ok: $OUT ($(du -h "$OUT" | awk '{print $1}'))"
echo "run:  $OUT"
echo "      # default: LAN IPv4:2420  (not 127.0.0.1)"
