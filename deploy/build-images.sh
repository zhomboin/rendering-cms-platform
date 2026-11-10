#!/usr/bin/env bash

# 仅构建进程使用代理；不向应用容器注入代理环境。
compose_build() (
  export COMPOSE_BAKE=false DOCKER_BUILDKIT=1 COMPOSE_PARALLEL_LIMIT=1
  export BUILDX_NO_DEFAULT_ATTESTATIONS=1

  local proxy="${BUILD_PROXY_URL:-}"
  if [[ -z "$proxy" ]]; then
    compose build
    return
  fi

  case "$proxy" in
    http://* | https://*) ;;
    *)
      log "错误：BUILD_PROXY_URL 必须使用 HTTP 或 HTTPS。"
      return 1
      ;;
  esac
  if [[ ! -r "${BUILD_PROXY_COMPOSE_FILE:-}" ]]; then
    log "错误：启用构建代理时必须提供可读的 BUILD_PROXY_COMPOSE_FILE。"
    return 1
  fi

  export HTTP_PROXY="$proxy" HTTPS_PROXY="$proxy"
  export http_proxy="$proxy" https_proxy="$proxy"
  export NO_PROXY="${BUILD_NO_PROXY:-localhost,127.0.0.1,::1}"
  export no_proxy="$NO_PROXY"
  export CMS_BUILD_PROXY="$proxy" CMS_BUILD_NO_PROXY="$NO_PROXY"
  # 覆盖文件位于仓库外，回退到未包含该文件的提交时仍然可用。
  compose -f "$BUILD_PROXY_COMPOSE_FILE" build
)
