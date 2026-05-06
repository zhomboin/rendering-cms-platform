#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/scripts/env/docker-compose.dev.yml"

fail() {
  echo "[fail] $*" >&2
  exit 1
}

[ -f "$ROOT_DIR/scripts/env/start-prerequisites.sh" ] || fail "start-prerequisites.sh is required"
[ -f "$ROOT_DIR/scripts/env/start-backend-docker.sh" ] || fail "start-backend-docker.sh is required"
[ -f "$ROOT_DIR/scripts/env/start-frontend-docker.sh" ] || fail "start-frontend-docker.sh is required"
[ ! -f "$ROOT_DIR/scripts/env/start-dev-services.sh" ] || fail "start-dev-services.sh duplicates start-prerequisites.sh and should be removed"

grep -q '^  backend:' "$COMPOSE_FILE" || fail "docker-compose.dev.yml must define backend service"
grep -q 'container_name: rendering-cms-backend' "$COMPOSE_FILE" || fail "backend container name is missing"
grep -q '8080:8080' "$COMPOSE_FILE" || fail "backend service must publish port 8080"
grep -q 'FRONTEND_ORIGINS: ${FRONTEND_ORIGINS:-http://127.0.0.1:3000,http://127.0.0.1:5173}' "$COMPOSE_FILE" || fail "backend service must allow multiple local frontend origins"
grep -q 'LOG_DIR: ${LOG_DIR:-/var/log/rendering-cms-platform}' "$COMPOSE_FILE" || fail "backend service must write logs to the mounted container log directory"
grep -q '${BACKEND_LOG_HOST_DIR:-../../logs/backend}:/var/log/rendering-cms-platform' "$COMPOSE_FILE" || fail "backend log directory must be bind-mounted to the host filesystem"
grep -q 'start-backend-docker.sh' "$ROOT_DIR/scripts/env/start-dev-stack.sh" || fail "start-dev-stack.sh must start backend"
grep -q -- '--profile backend' "$ROOT_DIR/scripts/env/start-backend-docker.sh" || fail "backend script must use backend profile"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
ENV_FILE="$TMP_DIR/.env" bash "$ROOT_DIR/scripts/env/sync-wsl-network-env.sh" >/dev/null
grep -q '^FRONTEND_ORIGIN=http://127.0.0.1:5173$' "$TMP_DIR/.env" || fail "sync script should default frontend origin to localhost"
grep -q '^FRONTEND_ORIGINS=http://127.0.0.1:3000,http://127.0.0.1:5173$' "$TMP_DIR/.env" || fail "sync script should default frontend origins to local dev ports"
grep -q '^VITE_API_BASE=http://127.0.0.1:8080/api/v1$' "$TMP_DIR/.env" || fail "sync script should default API base to localhost"
grep -q '^S3_ENDPOINT=http://127.0.0.1:9000$' "$TMP_DIR/.env" || fail "sync script should default S3 endpoint to localhost"

echo "dev scripts look consistent"
