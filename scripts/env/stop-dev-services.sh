#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"
COMPOSE_FILE="$ROOT_DIR/scripts/env/docker-compose.dev.yml"

if [ -f "$ENV_FILE" ]; then
  docker compose --profile backend --profile frontend --env-file "$ENV_FILE" -f "$COMPOSE_FILE" down
else
  docker compose --profile backend --profile frontend -f "$COMPOSE_FILE" down
fi
