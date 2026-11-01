#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
nginx_image="${NGINX_TEST_IMAGE:-nginx:1.28-alpine}"
command -v docker >/dev/null || { echo "需要 Docker 才能执行真实 NGINX 配置校验" >&2; exit 1; }
command -v openssl >/dev/null || { echo "需要 OpenSSL 生成临时测试证书" >&2; exit 1; }
docker info >/dev/null
mkdir -p "$repo_root/temp"
check_dir="$(mktemp -d "$repo_root/temp/nginx-check.XXXXXX")"
container_name="cms-nginx-check-$(basename "$check_dir")"
cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  rm -rf -- "$check_dir"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$check_dir/ssl"
openssl req -x509 -nodes -newkey rsa:2048 -days 1 -subj '/CN=nginx-test.invalid' \
  -keyout "$check_dir/ssl/privkey.pem" -out "$check_dir/ssl/fullchain.pem" >/dev/null 2>&1
printf '%s\n' 'location / { return 204; }' > "$check_dir/rendering-locations.conf"
printf '%s\n' 'events {}' 'http { include /etc/nginx/conf.d/test.conf; }' > "$check_dir/nginx.conf"

docker run --rm --name "$container_name" --add-host backend:127.0.0.1 \
  -v "$check_dir/nginx.conf:/etc/nginx/nginx.conf:ro" \
  -v "$repo_root/deploy/nginx/frontend.conf:/etc/nginx/conf.d/test.conf:ro" \
  "$nginx_image" nginx -t

docker run --rm --name "$container_name" \
  -v "$check_dir/nginx.conf:/etc/nginx/nginx.conf:ro" \
  -v "$repo_root/deploy/nginx/rendering.me.conf:/etc/nginx/conf.d/test.conf:ro" \
  -v "$repo_root/deploy/nginx/rendering-cms-proxy-headers.conf:/etc/nginx/snippets/rendering-cms-proxy-headers.conf:ro" \
  -v "$check_dir/rendering-locations.conf:/etc/nginx/snippets/rendering-locations.conf:ro" \
  -v "$check_dir/ssl:/etc/nginx/ssl/rendering.me:ro" \
  "$nginx_image" nginx -t
