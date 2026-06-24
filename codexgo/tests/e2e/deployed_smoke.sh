#!/usr/bin/env bash
set -euo pipefail

base="${CODEX_SERVER_BASE:-http://codex-server.localdev.com}"

check() {
  local path="$1"
  local url="${base%/}${path}"
  echo "[deployed-smoke] checking ${url}"
  curl -fsS -o /dev/null "${url}"
}

check "/healthz"
check "/readyz"

echo "[deployed-smoke] ok"
