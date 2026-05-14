#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DEPLOY_DIR="$REPO_ROOT/deploy"
ENV_FILE="$DEPLOY_DIR/production.env"
COMPOSE_FILE="$DEPLOY_DIR/docker-compose.prod.yml"
GO_IMAGE="${GO_IMAGE:-golang:1.26-alpine}"
SOURCE_DIR=""
AUTHOR_EMAIL=""
DRY_RUN=0

usage() {
  cat <<'USAGE'
Usage:
  scripts/ops/import-mdx.sh --source /srv/rendering/content/posts [--author-email admin@rendering.me] [--dry-run]

说明：
  生产 DATABASE_URL 中的 postgres 主机名只在 Docker 网络内可解析。
  本脚本会在 rendering_cms Docker 网络内启动临时 Go 容器执行导入工具。
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source)
      SOURCE_DIR="${2:-}"
      shift 2
      ;;
    --author-email)
      AUTHOR_EMAIL="${2:-}"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$SOURCE_DIR" ]]; then
  echo "missing --source" >&2
  usage >&2
  exit 2
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "missing production env file: $ENV_FILE" >&2
  exit 1
fi

if [[ ! -d "$SOURCE_DIR" ]]; then
  echo "source directory does not exist: $SOURCE_DIR" >&2
  exit 1
fi

SOURCE_ABS="$(realpath "$SOURCE_DIR")"

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "DATABASE_URL is required in $ENV_FILE" >&2
  exit 1
fi

cd "$DEPLOY_DIR"
docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE" ps postgres >/dev/null

args=(go run ./cmd/import-mdx -source /mdx-posts -database-url "$DATABASE_URL")
if [[ -n "$AUTHOR_EMAIL" ]]; then
  args+=(-author-email "$AUTHOR_EMAIL")
fi
if [[ "$DRY_RUN" -eq 1 ]]; then
  args+=(-dry-run)
fi

docker run --rm \
  --network rendering_cms \
  -e DATABASE_URL="$DATABASE_URL" \
  -v "$REPO_ROOT:/workspace:ro" \
  -v "$SOURCE_ABS:/mdx-posts:ro" \
  -v rendering_cms_go_pkg_mod:/go/pkg/mod \
  -v rendering_cms_go_build_cache:/root/.cache/go-build \
  -w /workspace/backend \
  "$GO_IMAGE" \
  "${args[@]}"
