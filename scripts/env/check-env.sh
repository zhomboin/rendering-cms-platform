#!/usr/bin/env bash
set -u

missing=0

check_command() {
  name="$1"
  command="$2"
  if command -v "$command" >/dev/null 2>&1; then
    printf '[ok] %s: ' "$name"
    "$command" --version 2>/dev/null || "$command" version 2>/dev/null || true
  else
    printf '[missing] %s: %s not found\n' "$name" "$command"
    missing=1
  fi
}

check_go() {
  if command -v go >/dev/null 2>&1; then
    printf '[ok] Go: '
    go version
  else
    printf '[missing] Go: go not found\n'
    missing=1
  fi
}

check_docker_compose() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    printf '[ok] Docker Compose: '
    docker compose version
  else
    printf '[missing] Docker Compose: docker compose not available\n'
    missing=1
  fi
}

check_go
check_command "Node.js" node
check_command "npm" npm
check_command "Docker" docker
check_docker_compose
check_command "PostgreSQL client" psql
check_command "sqlc" sqlc
check_command "golang-migrate" migrate

if [ "$missing" -ne 0 ]; then
  echo
  echo "Some required tools are missing. See docs/operations/development-environment.md."
  exit 1
fi

echo
echo "Development environment looks ready."
