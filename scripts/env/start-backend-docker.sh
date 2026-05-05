#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"
COMPOSE_FILE="$ROOT_DIR/scripts/env/docker-compose.dev.yml"

if [ ! -f "$ENV_FILE" ]; then
  echo "Missing .env. Create it with:"
  echo "cp scripts/env/dev.env.example .env"
  exit 1
fi

if [ ! -f "$ROOT_DIR/backend/go.mod" ]; then
  echo "Missing backend/go.mod. Scaffold backend before starting the Docker backend service."
  exit 1
fi

if [ -z "${BACKEND_LOG_HOST_DIR:-}" ]; then
  BACKEND_LOG_HOST_DIR="$(awk -F= '$1 == "BACKEND_LOG_HOST_DIR" { print substr($0, index($0, "=") + 1) }' "$ENV_FILE" | tail -n 1)"
fi
BACKEND_LOG_HOST_DIR="${BACKEND_LOG_HOST_DIR:-$ROOT_DIR/logs/backend}"
case "$BACKEND_LOG_HOST_DIR" in
  /*) ;;
  *) BACKEND_LOG_HOST_DIR="$ROOT_DIR/${BACKEND_LOG_HOST_DIR#./}" ;;
esac
mkdir -p "$BACKEND_LOG_HOST_DIR"
export BACKEND_LOG_HOST_DIR

bash "$ROOT_DIR/scripts/env/sync-wsl-network-env.sh"
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d postgres minio minio-init
docker compose --profile backend --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d backend
docker compose --profile backend --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps backend
