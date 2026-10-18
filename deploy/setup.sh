#!/usr/bin/env bash
#
# Rendering CMS Platform 自动部署一次性安装脚本（在服务器上以 root 执行）。
#
# 功能：
#   1. 创建目录布局：/etc/rendering-cms、/var/log/rendering-cms、
#      /var/lib/rendering-cms/backups、/var/lock/rendering-cms；
#   2. 迁移/安装环境配置（deploy/production.env -> /etc/rendering-cms/production.env，
#      安装 deploy.env 模板）；
#   3. 生成 systemd 单元到 /etc/systemd/system/（cms-rendering-deploy.service/.timer）；
#   4. 安装 logrotate 配置；
#   5. production.env 校验通过后启用 timer。
#
# 用法（在仓库根目录或任意位置执行均可）：
#   sudo bash deploy/setup.sh
#   sudo bash deploy/setup.sh --app-dir /opt/rendering-cms-platform --user root
#
# --user：指定 systemd 服务运行用户。非 root 用户必须已加入 docker 组，
#         且 /opt 仓库属主应为该用户。

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

APP_DIR="/opt/rendering-cms-platform"
SERVICE_USER="root"
while [ $# -gt 0 ]; do
  case "$1" in
    --app-dir) APP_DIR="$2"; shift 2 ;;
    --user) SERVICE_USER="$2"; shift 2 ;;
    -h|--help) sed -n '3,19p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "未知参数：$1" >&2; exit 2 ;;
  esac
done

CONFIG_DIR="/etc/rendering-cms"
LOG_DIR="/var/log/rendering-cms"
STATE_DIR="/var/lib/rendering-cms"
BACKUP_DIR="/var/lib/rendering-cms/backups"
LOCK_DIR="/var/lock/rendering-cms"
SERVICE_NAME="cms-rendering-deploy"

log() { printf '[setup] %s\n' "$*"; }

[ "$(id -u)" -eq 0 ] || { log "请用 root 执行：sudo bash $SCRIPT_DIR/setup.sh"; exit 1; }
command -v systemctl >/dev/null 2>&1 || { log "当前系统没有 systemd，无法安装 timer。"; exit 1; }
[ -d "$APP_DIR/.git" ] || { log "应用目录不是 Git 仓库：$APP_DIR（先按 docs/operations/deployment.md 完成首次部署）"; exit 1; }
[ -f "$SCRIPT_DIR/auto-deploy.sh" ] || { log "未找到 $SCRIPT_DIR/auto-deploy.sh，请在仓库内执行本脚本。"; exit 1; }

if [ "$SERVICE_USER" != "root" ]; then
  id "$SERVICE_USER" >/dev/null 2>&1 || { log "用户 $SERVICE_USER 不存在。"; exit 1; }
  if ! id -nG "$SERVICE_USER" | tr ' ' '\n' | grep -qx docker; then
    log "警告：用户 $SERVICE_USER 不在 docker 组，服务将无法执行 docker compose。"
    log "       执行：sudo usermod -aG docker $SERVICE_USER 后重新登录生效。"
  fi
fi

log "1/6 创建目录布局……"
install -d -m 0750 "$CONFIG_DIR" "$LOG_DIR" "$STATE_DIR"
install -d -m 0700 "$BACKUP_DIR"
install -d -m 0755 "$LOCK_DIR"
if [ "$SERVICE_USER" != "root" ]; then
  chown "$SERVICE_USER:$SERVICE_USER" "$CONFIG_DIR" "$LOG_DIR" "$STATE_DIR" "$BACKUP_DIR" "$LOCK_DIR"
fi

log "2/6 安装环境配置到 $CONFIG_DIR ……"
if [ -f "$SCRIPT_DIR/production.env" ] && [ ! -f "$CONFIG_DIR/production.env" ]; then
  # 迁移已有配置（包含密钥，权限收紧为 600）
  install -m 0600 "$SCRIPT_DIR/production.env" "$CONFIG_DIR/production.env"
  log "  已迁移 deploy/production.env -> $CONFIG_DIR/production.env"
elif [ ! -f "$CONFIG_DIR/production.env" ]; then
  install -m 0600 "$SCRIPT_DIR/production.env.example" "$CONFIG_DIR/production.env"
  log "  警告：已从模板生成 $CONFIG_DIR/production.env，请先填入真实生产配置！"
fi
if [ ! -f "$CONFIG_DIR/deploy.env" ]; then
  install -m 0644 "$SCRIPT_DIR/auto-deploy.env.example" "$CONFIG_DIR/deploy.env"
  log "  已安装 $CONFIG_DIR/deploy.env（可选调整项见文件内注释）"
fi

log "3/6 生成 systemd 单元到 /etc/systemd/system/ ……"
# systemd 单元只存在于服务器上（/etc/systemd/system/），代码仓库不保存单元文件；
# 此处按当前 APP_DIR / SERVICE_USER 现场生成。
cat > "/etc/systemd/system/$SERVICE_NAME.service" <<EOF
[Unit]
Description=Rendering CMS Platform 自动部署（拉取代码并按需发布）
Documentation=file://$APP_DIR/docs/operations/auto-deploy.md
After=network-online.target docker.service
Wants=network-online.target
Requires=docker.service
ConditionPathIsDirectory=$APP_DIR

[Service]
Type=oneshot
# 默认以 root 运行；Git 操作由 auto-deploy.sh 自动降权到仓库属主执行。
# 如需专用用户，用 deploy/setup.sh --user <name> 重新生成本单元。
User=$SERVICE_USER
Group=$SERVICE_USER
ExecStart=$APP_DIR/deploy/auto-deploy.sh
# 单次部署（构建镜像 + migration + 健康检查）的最长允许时间
TimeoutStartSec=30min
Nice=5

[Install]
WantedBy=multi-user.target
EOF

cat > "/etc/systemd/system/$SERVICE_NAME.timer" <<EOF
[Unit]
Description=Rendering CMS Platform 自动部署定时触发器

[Timer]
# 每小时整点检查一次远端提交（无更新时脚本秒退，不做任何变更）；
# 停机期间错过整点时，开机后补跑一次。
OnCalendar=hourly
Persistent=true
AccuracySec=1min
Unit=$SERVICE_NAME.service

[Install]
WantedBy=timers.target
EOF

chmod 0644 "/etc/systemd/system/$SERVICE_NAME.service" "/etc/systemd/system/$SERVICE_NAME.timer"
systemctl daemon-reload

log "4/6 安装 logrotate 配置……"
if [ -d /etc/logrotate.d ]; then
  install -m 0644 "$SCRIPT_DIR/logrotate/rendering-cms-deploy" /etc/logrotate.d/rendering-cms-deploy
else
  log "  系统没有 /etc/logrotate.d，跳过（日志清理请自行安排）。"
fi

log "5/6 校验 production.env ……"
ENV_READY=true
if grep -q 'replace-with' "$CONFIG_DIR/production.env" || grep -q '<[a-z-]*>' "$CONFIG_DIR/production.env"; then
  ENV_READY=false
  log "  警告：$CONFIG_DIR/production.env 仍包含占位值，暂不启用自动部署。"
  log "  请编辑该文件填入真实配置，然后执行："
  log "    sudo systemctl enable --now $SERVICE_NAME.timer"
else
  log "  配置看起来已就绪。"
fi

log "6/6 启用定时部署……"
if [ "$ENV_READY" = "true" ]; then
  systemctl enable --now "$SERVICE_NAME.timer"
  log "  已启用 $SERVICE_NAME.timer（每小时整点检查远端提交）。"
else
  log "  跳过启用 timer。"
fi

# 校验 Git 拉取凭据：以仓库属主身份做一次只读探测
REPO_OWNER="$(stat -c '%U' "$APP_DIR")"
log "检查 Git 远端访问（以 $REPO_OWNER 身份）……"
GIT_REMOTE="$(git -C "$APP_DIR" remote | head -n 1)"
GIT_BRANCH="$(git -C "$APP_DIR" rev-parse --abbrev-ref HEAD)"
if [ "$REPO_OWNER" = "root" ]; then
  git -C "$APP_DIR" ls-remote --heads "$GIT_REMOTE" >/dev/null 2>&1 \
    && log "  Git 远端访问正常（$GIT_REMOTE）。" \
    || log "  警告：root 无法访问 $GIT_REMOTE，请配置 deploy key 或改用仓库属主运行服务。"
else
  runuser -u "$REPO_OWNER" -- git -C "$APP_DIR" ls-remote --heads "$GIT_REMOTE" >/dev/null 2>&1 \
    && log "  Git 远端访问正常（$GIT_REMOTE，用户 $REPO_OWNER）。" \
    || log "  警告：用户 $REPO_OWNER 无法访问 $GIT_REMOTE，请为其配置 SSH deploy key。"
fi

log "完成。常用命令："
log "  手动触发一次部署：sudo systemctl start $SERVICE_NAME.service"
log "  查看部署日志：    journalctl -u $SERVICE_NAME.service -f"
log "                  或 /var/log/rendering-cms/latest.log"
log "  查看 timer 状态： systemctl list-timers | grep $SERVICE_NAME"
log "  停止自动部署：    sudo systemctl stop $SERVICE_NAME.timer"
