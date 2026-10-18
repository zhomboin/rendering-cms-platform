#!/usr/bin/env bash
#
# Rendering CMS Platform 自动部署脚本。
#
# 由 cms-rendering-deploy.service（systemd oneshot）定时触发，也可手动执行。
# 流程与 docs/operations/deployment.md 的手动发布 SOP 保持一致：
#   1. flock 防并发；
#   2. git fetch 并比较本地与远端提交，无更新直接退出；
#   3. git pull --ff-only；
#   4. docker compose build；
#   5. 备份 PostgreSQL 到 /var/lib/rendering-cms/backups/；
#   6. 执行 migration（--profile migrate）；
#   7. docker compose up -d backend frontend；
#   8. 健康检查 HEALTH_URL；
#   9. 失败时按阶段自动回滚代码并重建旧版本镜像。
#
# 退出码：0 = 部署成功或无需部署；1 = 部署失败（含自动回滚后的失败）。
#
# 用法：
#   deploy/auto-deploy.sh           # 常规定时触发
#   deploy/auto-deploy.sh --force   # 无新提交也强制重新构建部署

set -Eeuo pipefail
umask 027

# ---------------------------------------------------------------------------
# 配置加载：/etc/rendering-cms/deploy.env 优先，未设置的项目使用默认值。
# ---------------------------------------------------------------------------
DEPLOY_ENV_FILE="${DEPLOY_ENV_FILE:-/etc/rendering-cms/deploy.env}"
# shellcheck disable=SC1090
[ -r "$DEPLOY_ENV_FILE" ] && . "$DEPLOY_ENV_FILE"

: "${APP_DIR:=/opt/rendering-cms-platform}"
: "${GIT_REMOTE:=origin}"
: "${GIT_BRANCH:=main}"
: "${COMPOSE_FILE:=deploy/docker-compose.prod.yml}"
: "${PRODUCTION_ENV_FILE:=/etc/rendering-cms/production.env}"
: "${LOG_DIR:=/var/log/rendering-cms}"
: "${STATE_DIR:=/var/lib/rendering-cms}"
: "${BACKUP_DIR:=/var/lib/rendering-cms/backups}"
: "${LOCK_DIR:=/var/lock/rendering-cms}"
: "${HEALTH_URL:=http://127.0.0.1:3001/api/v1/health}"
: "${HEALTH_RETRIES:=12}"
: "${HEALTH_INTERVAL:=5}"
: "${BACKUP_KEEP:=14}"
: "${AUTO_ROLLBACK:=true}"
: "${PRUNE_IMAGES:=true}"
: "${NOTIFY_WEBHOOK:=}"

FORCE_DEPLOY=false
[ "${1:-}" = "--force" ] && FORCE_DEPLOY=true

STATE_FILE="$STATE_DIR/deploy-state.env"
LOCK_FILE="$LOCK_DIR/deploy.lock"

# COMPOSE_FILE 支持相对 APP_DIR 的写法
[[ "$COMPOSE_FILE" = /* ]] || COMPOSE_FILE="$APP_DIR/$COMPOSE_FILE"

# ---------------------------------------------------------------------------
# 日志：同时写入 /var/log/rendering-cms/deploy-*.log 和 stdout（进 journald）。
# ---------------------------------------------------------------------------
mkdir -p "$LOG_DIR" "$STATE_DIR" "$BACKUP_DIR" "$LOCK_DIR"
LOG_FILE="$LOG_DIR/deploy-$(date +%Y%m%d-%H%M%S).log"
exec > >(tee -a "$LOG_FILE") 2>&1
ln -sfn "$LOG_FILE" "$LOG_DIR/latest.log"

log() { printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }

trap 'log "意外错误：脚本第 ${LINENO} 行执行失败"' ERR

# ---------------------------------------------------------------------------
# 工具函数
# ---------------------------------------------------------------------------

# 以 Git 仓库属主身份执行命令，避免 root 拉取后在仓库里留下 root 属主文件。
REPO_OWNER="$(stat -c '%U' "$APP_DIR" 2>/dev/null || echo root)"
as_repo_user() {
  if [ "$(id -u)" -eq 0 ] && [ "$REPO_OWNER" != "root" ] && id "$REPO_OWNER" >/dev/null 2>&1; then
    runuser -u "$REPO_OWNER" -- "$@"
  else
    "$@"
  fi
}

git_cmd() { as_repo_user git -C "$APP_DIR" "$@"; }

compose() { docker compose --env-file "$PRODUCTION_ENV_FILE" -f "$COMPOSE_FILE" "$@"; }

# 可选 webhook 通知；未配置时为空操作，不影响退出码。
notify() {
  local escaped
  [ -n "$NOTIFY_WEBHOOK" ] || return 0
  escaped="${1//\\/\\\\}"
  escaped="${escaped//\"/\\\"}"
  curl -fsS -m 10 -X POST -H 'Content-Type: application/json' \
    -d "{\"text\": \"$escaped\"}" "$NOTIFY_WEBHOOK" >/dev/null 2>&1 \
    || log "警告：通知 webhook 调用失败"
  return 0
}

die_preflight() { log "预检查失败：$1"; exit 1; }

postgres_running() {
  local id
  id="$(compose ps -q postgres 2>/dev/null || true)"
  [ -n "$id" ] || return 1
  [ "$(docker inspect -f '{{.State.Running}}' "$id" 2>/dev/null || true)" = "true" ]
}

backup_postgres() {
  local file
  file="$BACKUP_DIR/rendering-cms-$(date +%Y%m%d-%H%M%S).sql.gz"
  compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB"' | gzip > "$file"
  [ -s "$file" ] || { log "错误：备份文件为空：$file"; return 1; }
  gzip -t "$file" || { log "错误：备份文件 gzip 校验失败：$file"; return 1; }
  chmod 600 "$file"
  log "PostgreSQL 备份完成：$file（$(du -h "$file" | cut -f1)）"
}

prune_backups() {
  local old
  # shellcheck disable=SC2012
  mapfile -t old < <(ls -1t "$BACKUP_DIR"/rendering-cms-*.sql.gz 2>/dev/null | tail -n +"$((BACKUP_KEEP + 1))" || true)
  [ "${#old[@]}" -eq 0 ] && return 0
  rm -f -- "${old[@]}"
  log "已清理过期备份 ${#old[@]} 个（保留最近 $BACKUP_KEEP 个）"
}

wait_health() {
  local i
  for ((i = 1; i <= HEALTH_RETRIES; i++)); do
    if curl -fsS -m 5 "$HEALTH_URL" >/dev/null 2>&1; then
      log "健康检查通过：$HEALTH_URL"
      return 0
    fi
    log "健康检查未通过（第 $i/$HEALTH_RETRIES 次），${HEALTH_INTERVAL}s 后重试"
    sleep "$HEALTH_INTERVAL"
  done
  log "健康检查失败：连续 $HEALTH_RETRIES 次无法访问 $HEALTH_URL"
  return 1
}

write_state() {
  local new_commit="$1" prev_recorded
  prev_recorded="$(grep -m1 '^LAST_SUCCESS_COMMIT=' "$STATE_FILE" 2>/dev/null | cut -d= -f2- || true)"
  cat > "$STATE_FILE" <<EOF
LAST_SUCCESS_COMMIT=$new_commit
LAST_SUCCESS_AT=$(date -Is)
PREVIOUS_SUCCESS_COMMIT=${prev_recorded:-$PREV_COMMIT}
EOF
}

# 代码级回滚：reset 到上一提交，重建旧镜像并恢复容器。
# 只回退应用代码，绝不自动回滚数据库（见 docs/operations/auto-deploy.md）。
rollback_code() {
  log "开始代码回滚：git reset --hard $PREV_COMMIT"
  git_cmd reset --hard "$PREV_COMMIT" || return 1
  log "回滚后重新构建旧版本镜像"
  compose build || return 1
  compose up -d backend frontend || return 1
  wait_health || { log "回滚后健康检查仍未通过，请立即人工介入"; return 1; }
  log "代码回滚完成，服务已恢复到 $PREV_COMMIT"
}

deploy_failed() {
  local stage="$1"
  log "部署失败：$stage（目标提交 $NEW_COMMIT）"
  if [ "$AUTO_ROLLBACK" = "true" ]; then
    if rollback_code; then
      notify "Rendering CMS 自动部署失败已自动回滚：$stage（目标 $NEW_COMMIT），当前运行 $PREV_COMMIT"
    else
      notify "Rendering CMS 自动部署失败且回滚异常：$stage（目标 $NEW_COMMIT），请人工介入"
    fi
  else
    notify "Rendering CMS 自动部署失败（AUTO_ROLLBACK=false）：$stage（目标 $NEW_COMMIT）"
  fi
  log "==== 部署失败 ===="
  exit 1
}

# migration 失败的数据库可能处于 dirty 状态：只回退代码、保持现有容器运行，
# 不自动重建重启，避免旧后端撞上未完成的 schema 变更。
deploy_failed_migrate() {
  log "部署失败：migration 执行失败，数据库可能处于 dirty 状态。"
  log "已按安全策略仅回退代码、不重启容器；请按 docs/operations/restore.md 排查数据库。"
  git_cmd reset --hard "$PREV_COMMIT" || log "警告：代码回退失败，请手动 git reset --hard $PREV_COMMIT"
  notify "Rendering CMS 自动部署失败（migration，目标 $NEW_COMMIT）：数据库可能 dirty，需人工处理"
  log "==== 部署失败 ===="
  exit 1
}

# ---------------------------------------------------------------------------
# 主流程
# ---------------------------------------------------------------------------
log "==== Rendering CMS 自动部署开始（force=$FORCE_DEPLOY，PID=$$）===="

# 1. 防并发锁：手工执行与定时触发共用同一把锁
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  log "已有部署正在进行（锁 $LOCK_FILE 被占用），本次跳过。"
  exit 0
fi

# 2. 预检查
command -v docker >/dev/null 2>&1 || die_preflight "服务器未安装 docker"
docker compose version >/dev/null 2>&1 || die_preflight "docker compose 不可用"
command -v curl >/dev/null 2>&1 || die_preflight "服务器未安装 curl"
command -v flock >/dev/null 2>&1 || die_preflight "服务器缺少 flock（util-linux）"
[ -d "$APP_DIR/.git" ] || die_preflight "应用目录不是 Git 仓库：$APP_DIR"
[ -r "$PRODUCTION_ENV_FILE" ] || die_preflight "缺少生产环境变量文件：$PRODUCTION_ENV_FILE（参考 deploy/production.env.example）"
git_cmd remote get-url "$GIT_REMOTE" >/dev/null 2>&1 || die_preflight "Git remote '$GIT_REMOTE' 不存在，服务器无法拉取代码"

# 3. 拉取远端并比较提交
if ! git_cmd fetch "$GIT_REMOTE" "$GIT_BRANCH"; then
  log "git fetch 失败：请检查服务器 Git 凭据（deploy key / SSH key）与网络。"
  exit 1
fi

LOCAL_COMMIT="$(git_cmd rev-parse HEAD)"
REMOTE_COMMIT="$(git_cmd rev-parse "$GIT_REMOTE/$GIT_BRANCH")"

if [ "$LOCAL_COMMIT" = "$REMOTE_COMMIT" ] && [ "$FORCE_DEPLOY" = "false" ]; then
  log "远端无新提交（HEAD=$LOCAL_COMMIT），本次不部署。"
  exit 0
fi

# 4. 工作区必须干净（忽略未跟踪文件），避免覆盖服务器上的本地改动
if [ -n "$(git_cmd status --porcelain --untracked-files=no)" ]; then
  log "检测到 $APP_DIR 存在未提交的本地改动，为避免覆盖已中止部署。请手动处理后重试。"
  git_cmd status --short --untracked-files=no || true
  exit 1
fi

PREV_COMMIT="$LOCAL_COMMIT"
NEW_COMMIT="$REMOTE_COMMIT"

log "检测到新提交：$PREV_COMMIT -> $NEW_COMMIT"
log "待发布提交："
git_cmd log --oneline --no-decorate "$PREV_COMMIT..$NEW_COMMIT" || true

# 5. 按固定 SOP 依次执行：pull -> build -> backup -> migrate -> up -> health
git_cmd pull --ff-only "$GIT_REMOTE" "$GIT_BRANCH" || deploy_failed "git pull 失败（非快进或网络异常）"

compose build || deploy_failed "镜像构建失败"

if postgres_running; then
  backup_postgres || deploy_failed "数据库备份失败"
else
  log "警告：postgres 容器未运行，跳过本次备份（首次部署场景）。"
fi

compose --profile migrate run --rm migrate || deploy_failed_migrate

compose up -d backend frontend || deploy_failed "应用容器启动失败"

wait_health || deploy_failed "健康检查失败"

# 6. 记录状态、清理
write_state "$NEW_COMMIT"
prune_backups
if [ "$PRUNE_IMAGES" = "true" ]; then
  docker image prune -f >/dev/null 2>&1 || true
  log "已清理 dangling 镜像。"
fi

compose ps || true
notify "Rendering CMS 部署成功：$PREV_COMMIT -> $NEW_COMMIT"
log "==== 部署成功：$NEW_COMMIT ===="
