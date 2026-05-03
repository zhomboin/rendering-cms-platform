#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${ENV_FILE:-$ROOT_DIR/.env}"
DEV_ENV_EXAMPLE="$ROOT_DIR/scripts/env/dev.env.example"

if [ ! -f "$ENV_FILE" ]; then
  if [ -f "$DEV_ENV_EXAMPLE" ]; then
    cp "$DEV_ENV_EXAMPLE" "$ENV_FILE"
  else
    echo "Missing .env and scripts/env/dev.env.example." >&2
    exit 1
  fi
fi

if [ -z "${WSL_IP:-}" ] && [ -z "${WIN_IP:-}" ] && [ -f "$HOME/.bashrc" ]; then
  set +u
  # Allow users to export WIN_IP or WSL_IP from ~/.bashrc without requiring
  # every caller to be an interactive shell.
  # shellcheck disable=SC1090
  source "$HOME/.bashrc" >/dev/null 2>&1 || true
  set -u
fi

resolve_wsl_ip() {
  local candidate="${WSL_IP:-${WIN_IP:-}}"
  if [ -n "$candidate" ]; then
    printf '%s\n' "$candidate"
    return
  fi

  candidate="$(ip -4 addr show eth0 2>/dev/null | awk '/inet / { sub(/\/.*/, "", $2); print $2; exit }')"
  if [ -n "$candidate" ]; then
    printf '%s\n' "$candidate"
    return
  fi

  hostname -I | tr ' ' '\n' | awk 'NF { print; exit }'
}

resolve_browser_host() {
  local candidate="${BROWSER_HOST:-127.0.0.1}"
  case "$candidate" in
    localhost)
      printf '127.0.0.1\n'
      ;;
    wsl|auto)
      resolve_wsl_ip
      ;;
    *)
      printf '%s\n' "$candidate"
      ;;
  esac
}

upsert_env() {
  local key="$1"
  local value="$2"
  local tmp
  tmp="$(mktemp)"
  awk -v key="$key" -v value="$value" '
    BEGIN { found = 0 }
    $0 ~ "^" key "=" {
      print key "=" value
      found = 1
      next
    }
    { print }
    END {
      if (!found) {
        print key "=" value
      }
    }
  ' "$ENV_FILE" > "$tmp"
  mv "$tmp" "$ENV_FILE"
}

BROWSER_ACCESS_HOST="$(resolve_browser_host)"
if [ -z "$BROWSER_ACCESS_HOST" ]; then
  echo "Unable to resolve browser access host. Set BROWSER_HOST=127.0.0.1 or export WIN_IP/WSL_IP with BROWSER_HOST=wsl." >&2
  exit 1
fi

case "$BROWSER_ACCESS_HOST" in
  *[!0-9.]*)
    echo "Resolved browser access host is invalid: $BROWSER_ACCESS_HOST" >&2
    exit 1
    ;;
esac

FRONTEND_PORT="${FRONTEND_PORT:-5173}"
BACKEND_PORT="${BACKEND_PORT:-8080}"
MINIO_PORT="${MINIO_PORT:-9000}"

upsert_env "FRONTEND_ORIGIN" "http://$BROWSER_ACCESS_HOST:$FRONTEND_PORT"
upsert_env "VITE_API_BASE" "http://$BROWSER_ACCESS_HOST:$BACKEND_PORT/api/v1"
upsert_env "S3_ENDPOINT" "http://$BROWSER_ACCESS_HOST:$MINIO_PORT"

echo "Updated $ENV_FILE with browser-accessible addresses:"
echo "FRONTEND_ORIGIN=http://$BROWSER_ACCESS_HOST:$FRONTEND_PORT"
echo "VITE_API_BASE=http://$BROWSER_ACCESS_HOST:$BACKEND_PORT/api/v1"
echo "S3_ENDPOINT=http://$BROWSER_ACCESS_HOST:$MINIO_PORT"
