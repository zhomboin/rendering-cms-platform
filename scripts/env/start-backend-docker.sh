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

bash "$ROOT_DIR/scripts/env/sync-wsl-network-env.sh"
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d postgres minio minio-init
docker compose --profile backend --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d backend
docker compose --profile backend --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps backend
