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

if [ ! -f "$ROOT_DIR/frontend/package.json" ]; then
  echo "Missing frontend/package.json. Scaffold frontend before starting the Docker frontend service."
  exit 1
fi

bash "$ROOT_DIR/scripts/env/sync-wsl-network-env.sh"

docker compose --profile frontend --env-file "$ENV_FILE" -f "$COMPOSE_FILE" up -d frontend
docker compose --profile frontend --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps frontend
