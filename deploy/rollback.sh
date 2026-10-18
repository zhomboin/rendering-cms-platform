#!/usr/bin/env bash
#
# Rendering CMS Platform 手动回滚脚本。
#
# 默认只回滚代码到上一次成功部署的提交（读取 /var/lib/rendering-cms/deploy-state.env）；
# 指定 --backup 时会先停 backend、DROP/CREATE 数据库并导入指定备份，再回滚代码。
# 数据库恢复语义与 docs/operations/restore.md 一致：恢复前保留现场、停止写入。
#
# 用法：
#   deploy/rollback.sh                          # 回滚代码到上一次成功提交
#   deploy/rollback.sh --commit <sha>           # 回滚代码到指定提交
#   deploy/rollback.sh --backup latest          # 同时恢复最近一个数据库备份
#   deploy/rollback.sh --backup <备份文件>       # 同时恢复指定备份（.sql.gz 或 .sql）
#   deploy/rollback.sh --yes                    # 跳过确认（谨慎使用）

set -Eeuo pipefail
umask 027

DEPLOY_ENV_FILE="${DEPLOY_ENV_FILE:-/etc/rendering-cms/deploy.env}"
# shellcheck disable=SC1090
[ -r "$DEPLOY_ENV_FILE" ] && . "$DEPLOY_ENV_FILE"

: "${APP_DIR:=/opt/rendering-cms-platform}"
: "${COMPOSE_FILE:=deploy/docker-compose.prod.yml}"
: "${PRODUCTION_ENV_FILE:=/etc/rendering-cms/production.env}"
: "${STATE_DIR:=/var/lib/rendering-cms}"
: "${BACKUP_DIR:=/var/lib/rendering-cms/backups}"
: "${LOCK_DIR:=/var/lock/rendering-cms}"
: "${LOG_DIR:=/var/log/rendering-cms}"
: "${HEALTH_URL:=http://127.0.0.1:3001/api/v1/health}"
: "${HEALTH_RETRIES:=12}"
: "${HEALTH_INTERVAL:=5}"

STATE_FILE="$STATE_DIR/deploy-state.env"
LOCK_FILE="$LOCK_DIR/deploy.lock"
[[ "$COMPOSE_FILE" = /* ]] || COMPOSE_FILE="$APP_DIR/$COMPOSE_FILE"

# ---------------------------------------------------------------------------
# 参数解析（放在日志/锁初始化之前，--help 无需 root）
# ---------------------------------------------------------------------------
TARGET_COMMIT=""
BACKUP_FILE=""
ASSUME_YES=false
while [ $# -gt 0 ]; do
  case "$1" in
    --commit)
      [ $# -ge 2 ] || { echo "参数错误：--commit 需要提交号" >&2; exit 2; }
      TARGET_COMMIT="$2"; shift 2 ;;
    --backup)
      [ $# -ge 2 ] || { echo "参数错误：--backup 需要文件路径或 latest" >&2; exit 2; }
      BACKUP_FILE="$2"; shift 2 ;;
    --yes|-y) ASSUME_YES=true; shift ;;
    -h|--help)
      sed -n '3,14p' "$0" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) echo "未知参数：$1（使用 --help 查看用法）" >&2; exit 2 ;;
  esac
done

mkdir -p "$LOG_DIR" "$STATE_DIR" "$BACKUP_DIR" "$LOCK_DIR"
LOG_FILE="$LOG_DIR/rollback-$(date +%Y%m%d-%H%M%S).log"
exec > >(tee -a "$LOG_FILE") 2>&1
ln -sfn "$LOG_FILE" "$LOG_DIR/rollback-latest.log"

log() { printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"; }
trap 'log "意外错误：脚本第 ${LINENO} 行执行失败"' ERR

REPO_OWNER="$(stat -c '%U' "$APP_DIR" 2>/dev/null || echo root)"
git_cmd() {
  if [ "$(id -u)" -eq 0 ] && [ "$REPO_OWNER" != "root" ] && id "$REPO_OWNER" >/dev/null 2>&1; then
    runuser -u "$REPO_OWNER" -- git -C "$APP_DIR" "$@"
  else
    git -C "$APP_DIR" "$@"
  fi
}

compose() { docker compose --env-file "$PRODUCTION_ENV_FILE" -f "$COMPOSE_FILE" "$@"; }

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
  return 1
}

# ---------------------------------------------------------------------------
# 防并发锁（与 auto-deploy.sh 共用，回滚期间定时部署自动跳过）
# ---------------------------------------------------------------------------
exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  log "部署锁 $LOCK_FILE 被占用，请等当前部署/回滚结束后重试。"
  exit 1
fi

# ---------------------------------------------------------------------------
# 计算回滚目标
# ---------------------------------------------------------------------------
[ -d "$APP_DIR/.git" ] || { log "错误：$APP_DIR 不是 Git 仓库"; exit 1; }
[ -r "$PRODUCTION_ENV_FILE" ] || { log "错误：缺少 $PRODUCTION_ENV_FILE"; exit 1; }

LAST_SUCCESS="$(grep -m1 '^LAST_SUCCESS_COMMIT=' "$STATE_FILE" 2>/dev/null | cut -d= -f2- || true)"
PREV_SUCCESS="$(grep -m1 '^PREVIOUS_SUCCESS_COMMIT=' "$STATE_FILE" 2>/dev/null | cut -d= -f2- || true)"

if [ -z "$TARGET_COMMIT" ]; then
  if [ -n "$PREV_SUCCESS" ]; then
    TARGET_COMMIT="$PREV_SUCCESS"
  elif [ -n "$LAST_SUCCESS" ]; then
    TARGET_COMMIT="$(git_cmd rev-parse "${LAST_SUCCESS}^" 2>/dev/null || true)"
  fi
fi
[ -n "$TARGET_COMMIT" ] || { log "错误：无法确定回滚目标提交，请用 --commit <sha> 指定。"; exit 1; }
git_cmd rev-parse --verify -q "$TARGET_COMMIT^{commit}" >/dev/null \
  || { log "错误：提交不存在：$TARGET_COMMIT"; exit 1; }
TARGET_COMMIT="$(git_cmd rev-parse "$TARGET_COMMIT^{commit}")"

# 解析数据库备份
if [ -n "$BACKUP_FILE" ] && [ "$BACKUP_FILE" = "latest" ]; then
  BACKUP_FILE="$(ls -1t "$BACKUP_DIR"/rendering-cms-*.sql.gz "$BACKUP_DIR"/rendering-cms-*.sql 2>/dev/null | head -n 1 || true)"
  [ -n "$BACKUP_FILE" ] || { log "错误：$BACKUP_DIR 下没有可用的数据库备份。"; exit 1; }
fi
if [ -n "$BACKUP_FILE" ] && [ ! -f "$BACKUP_FILE" ]; then
  log "错误：备份文件不存在：$BACKUP_FILE"
  exit 1
fi

# ---------------------------------------------------------------------------
# 确认
# ---------------------------------------------------------------------------
log "回滚计划："
log "  当前 HEAD：$(git_cmd rev-parse HEAD)"
log "  目标提交：$TARGET_COMMIT"
log "  数据库恢复：$([ -n "$BACKUP_FILE" ] && echo "$BACKUP_FILE" || echo '不恢复（保持现状）')"
if [ -n "$BACKUP_FILE" ]; then
  log "  注意：数据库将被 DROP 后重建并导入该备份，当前数据会被覆盖！"
fi
if [ "$ASSUME_YES" != "true" ]; then
  read -r -p "确认执行回滚？(输入 yes 继续)：" answer
  [ "$answer" = "yes" ] || { log "已取消。"; exit 1; }
fi

# ---------------------------------------------------------------------------
# 执行
# ---------------------------------------------------------------------------
log "==== 开始手动回滚 ===="

# 建议暂停定时部署，避免回滚刚完成又被新提交顶掉
if systemctl is-active cms-rendering-deploy.timer >/dev/null 2>&1; then
  log "提示：回滚后建议执行 systemctl stop cms-rendering-deploy.timer 暂停自动部署，确认稳定后再启动。"
fi

if [ -n "$BACKUP_FILE" ]; then
  if [[ "$BACKUP_FILE" == *.gz ]]; then
    gzip -t "$BACKUP_FILE" || { log "错误：备份 gzip 校验失败，已中止。"; exit 1; }
  fi
  [ -s "$BACKUP_FILE" ] || { log "错误：备份文件为空，已中止。"; exit 1; }

  log "停止 backend，避免恢复期间继续写入……"
  compose stop backend

  log "重建空数据库（DROP + CREATE）……"
  # 变量统一在 postgres 容器内展开（容器已注入 POSTGRES_USER/POSTGRES_DB），宿主机不加载明文密码。
  compose exec -T postgres sh -c \
    'psql -U "$POSTGRES_USER" -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '\''$POSTGRES_DB'\'' AND pid <> pg_backend_pid()" >/dev/null'
  compose exec -T postgres sh -c \
    'psql -U "$POSTGRES_USER" -d postgres -c "DROP DATABASE IF EXISTS \"$POSTGRES_DB\""'
  compose exec -T postgres sh -c \
    'psql -U "$POSTGRES_USER" -d postgres -c "CREATE DATABASE \"$POSTGRES_DB\" OWNER \"$POSTGRES_USER\""'

  log "导入备份：$BACKUP_FILE ……"
  if [[ "$BACKUP_FILE" == *.gz ]]; then
    gzip -dc "$BACKUP_FILE" | compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
  else
    compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < "$BACKUP_FILE"
  fi
  log "数据库恢复完成。"
fi

log "回滚代码到 $TARGET_COMMIT ……"
git_cmd reset --hard "$TARGET_COMMIT"

log "重建镜像并启动应用……"
compose build
compose up -d backend frontend

wait_health || { log "回滚后健康检查失败，请查看容器日志：docker compose logs backend"; exit 1; }

# 更新状态文件，把本次回滚记录为当前成功状态
cat > "$STATE_FILE" <<EOF
LAST_SUCCESS_COMMIT=$TARGET_COMMIT
LAST_SUCCESS_AT=$(date -Is)
PREVIOUS_SUCCESS_COMMIT=${LAST_SUCCESS:-}
EOF

log "==== 回滚完成：$TARGET_COMMIT ===="
log "后续建议："
log "  1. 登录后台抽查文章、评论、文件功能；"
log "  2. 若刚才暂停了 timer，确认稳定后执行 systemctl start cms-rendering-deploy.timer；"
log "  3. 若回滚原因是某次提交本身有问题，请先在远端修复并推送，再恢复自动部署。"
