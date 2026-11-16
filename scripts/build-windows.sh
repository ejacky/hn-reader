#!/usr/bin/env bash
# Build the Windows local server and installer from Linux.
#
# This does not run the installer: UAC, service removal and per-user registry
# behaviour can only be verified on Windows (CI or a real machine). It exists so
# the Go build and the NSIS scripts can be compiled and inspected locally.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${VERSION:-1.3.4}"

MAKENSIS="${MAKENSIS:-}"
if [ -z "$MAKENSIS" ]; then
  if command -v makensis >/dev/null 2>&1; then
    MAKENSIS="$(command -v makensis)"
  elif [ -x "$ROOT/.gotmp/nsis/root/usr/bin/makensis" ]; then
    MAKENSIS="$ROOT/.gotmp/nsis/root/usr/bin/makensis"
  else
    echo "makensis not found. Install NSIS or set MAKENSIS/NSISDIR." >&2
    echo "On Ubuntu: apt-get download nsis nsis-common && dpkg-deb -x ..." >&2
    exit 1
  fi
fi

if [ -z "${NSISDIR:-}" ] && [ -d "$ROOT/.gotmp/nsis/root/usr/share/nsis" ]; then
  export NSISDIR="$ROOT/.gotmp/nsis/root/usr/share/nsis"
fi

echo "==> building Windows local server $VERSION"
(
  cd "$ROOT/server"
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build \
    -trimpath \
    -ldflags "-s -w -H windowsgui -X main.Version=$VERSION" \
    -o hn-reader.exe ./cmd/hn-reader
)

echo "==> building legacy migration helper"
(
  cd "$ROOT/installer"
  "$MAKENSIS" -INPUTCHARSET UTF8 legacy-cleanup.nsi
)

echo "==> building installer"
(
  cd "$ROOT/installer"
  "$MAKENSIS" -INPUTCHARSET UTF8 -DPRODUCT_VERSION="$VERSION" setup.nsi
)

echo "==> artifacts"
ls -lh "$ROOT/server/hn-reader.exe" \
  "$ROOT/installer/legacy-cleanup.exe" \
  "$ROOT/installer/HNReader-Setup-$VERSION.exe"
sha256sum "$ROOT/installer/HNReader-Setup-$VERSION.exe"
