# 生产 Docker 部署方案

本文档是 Rendering CMS Platform 的生产 Docker 部署入口。命令默认在 Ubuntu 服务器中执行，使用 `docker compose`，不使用旧版 `docker-compose`。

## 部署拓扑

生产 Docker 版本包含以下服务：

- `frontend`：Nginx 容器，托管 Vite 构建产物，并把 `/api/v1/` 反向代理到后端。
- `backend`：Go 后端容器，监听容器内 `0.0.0.0:8080`。
- `postgres`：PostgreSQL 16 容器，数据保存在 Docker volume `rendering_cms_postgres_data`。
- `migrate`：一次性 migration 容器，仅在发布时手动执行。
- Cloudflare R2：托管上传文件本体，后端只保存文件元数据和 `storage_key`。

推荐公网入口：

```text
用户浏览器
  -> https://cms.rendering.me
  -> 服务器 Nginx/Caddy/负载均衡器
  -> 127.0.0.1:3001
  -> frontend 容器
  -> backend 容器
  -> postgres 容器

用户浏览器
  -> Cloudflare R2 预签名 URL
  -> R2 bucket
```

生产对象存储使用 Cloudflare R2。因为上传和下载 URL 是后端生成后返回给浏览器使用，`S3_ENDPOINT` 必须配置为 R2 S3 API 端点，例如 `https://<account-id>.r2.cloudflarestorage.com`。预签名 URL 不使用 R2 自定义域名。

## 文件入口

- `deploy/docker-compose.prod.yml`：生产 Compose 文件。
- `deploy/production.env.example`：生产环境变量模板。
- `deploy/auto-deploy.sh`：自动部署主脚本（systemd timer 定时触发）。
- `deploy/setup.sh`：自动部署一次性安装脚本（目录、systemd 单元、配置迁移）。
- `deploy/rollback.sh`：手动回滚脚本（代码回滚 + 可选数据库恢复）。
- systemd 单元 `cms-rendering-deploy.service` / `.timer`：不在仓库内，由 `deploy/setup.sh` 在服务器上生成到 `/etc/systemd/system/`。
- `deploy/nginx/frontend.conf`：前端 Nginx 容器配置。
- `deploy/nginx/rendering.me.conf`：服务器宿主机 Nginx 完整反向代理配置。
- `deploy/nginx/rendering-cms-proxy-headers.conf`：CMS 各 location 共用的可信代理头、超时和缓冲配置。
- `deploy/nginx/cloudflare-realip.conf`：Cloudflare 橙云代理下的真实 IP 还原配置（宿主机 Nginx http 级 include）。
- `backend/Dockerfile`：后端生产镜像。
- `frontend/Dockerfile`：前端生产镜像。
- `docs/operations/auto-deploy.md`：systemd timer 自动部署与回滚 SOP。
- `docs/operations/maintenance.md`：运维速查卡（定期检查清单、目录地图、异常第一反应）。
- `docs/operations/runbook.md`：固定运维 SOP。
- `docs/operations/production-access.md`：生产访问、登录和后台用户初始化 SOP。
- `docs/operations/backup.md`：备份要求。
- `docs/operations/restore.md`：恢复流程。

## 服务器准备

创建应用目录并拉取代码：

```bash
sudo mkdir -p /opt/rendering-cms-platform
sudo chown "$USER":"$USER" /opt/rendering-cms-platform
cd /opt/rendering-cms-platform
git clone <repo-url> .
```

确认 Docker 可用：

```bash
docker version
docker compose version
```

准备备份目录：

```bash
mkdir -p /var/lib/rendering-cms/backups
chmod 700 /var/lib/rendering-cms/backups
```

## 环境变量

复制生产环境模板到 `/etc/rendering-cms/`（自动部署环境统一从这里读取；旧位置 `deploy/production.env` 由 `deploy/setup.sh` 自动迁移）：

```bash
sudo mkdir -p /etc/rendering-cms
sudo cp /opt/rendering-cms-platform/deploy/production.env.example /etc/rendering-cms/production.env
sudo chmod 600 /etc/rendering-cms/production.env
```

### 必须修改的生产配置

生产部署时必须修改 `/etc/rendering-cms/production.env`。不要直接使用 `deploy/production.env.example` 中的占位值。

基础部署配置：

```env
APP_IMAGE_TAG=latest
PUBLIC_HTTP_BIND=127.0.0.1:3001
```

数据库配置：

```env
POSTGRES_DB=rendering_cms
POSTGRES_USER=rendering
POSTGRES_PASSWORD=replace-with-strong-database-password
DATABASE_URL=postgres://rendering:replace-with-strong-database-password@postgres:5432/rendering_cms?sslmode=disable
```

后端运行时与登录安全配置：

```env
APP_ENV=production
DEV_BOOTSTRAP_ADMIN=false
JWT_SECRET=replace-with-32-plus-character-secret
FRONTEND_ORIGIN=https://cms.rendering.me
FRONTEND_ORIGINS=https://cms.rendering.me,https://rendering.me,https://www.rendering.me
VITE_API_BASE=/api/v1
PUBLIC_READ_RATE_PER_SECOND=20
PUBLIC_READ_BURST=40
PUBLIC_SEARCH_RATE_PER_SECOND=5
PUBLIC_SEARCH_BURST=10
PUBLIC_MAX_IN_FLIGHT=128
PUBLIC_RATE_LIMIT_MAX_CLIENTS=10000
LOG_RETENTION_DAYS=30
ARTICLE_LOG_RETENTION_LIMIT=20
```

登录邮箱验证码 SMTP 配置：

```env
SMTP_HOST=smtp.example.com
SMTP_PORT=465
SMTP_USERNAME=cms@example.com
SMTP_PASSWORD=<smtp-授权码或密码>
SMTP_FROM=cms@example.com
LOGIN_MFA_ENABLED=true
LOGIN_MFA_DEV_LOG=false
```

Cloudflare R2 对象存储配置：

```env
S3_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com
S3_REGION=auto
S3_BUCKET=rendering-assets
S3_ACCESS_KEY_ID=<r2-access-key-id>
S3_SECRET_ACCESS_KEY=<r2-secret-access-key>
S3_USE_PATH_STYLE=false
S3_PUBLIC_BASE_URL=https://assets.rendering.me
S3_BLOG_IMAGE_PREFIX=blog
S3_ASSET_FILE_PREFIX=assets
```

配置要求：

- `POSTGRES_PASSWORD` 必须使用强密码；如果密码包含特殊字符，`DATABASE_URL` 中的密码段必须 URL 编码。
- `DATABASE_URL` 在容器网络内连接 `postgres:5432`，不要写成 `127.0.0.1`。
- `APP_ENV` 生产环境固定为 `production`。
- `DEV_BOOTSTRAP_ADMIN` 生产环境固定为 `false`，不要在生产启动 dev 默认管理员账号填充。
- `JWT_SECRET` 至少 32 字符，生产环境不得使用示例值。
- `FRONTEND_ORIGINS` 使用生产访问域名；多个来源使用英文逗号分隔。
- `VITE_API_BASE` 保持 `/api/v1`，让浏览器走同域 API 反代。
- `PUBLIC_*` 是 Go 服务的第二层保护。搜索默认额度低于普通文章读取；调整时应结合 Nginx `429`、应用 `429/503`、数据库 P95 和连接池使用率，不得直接取消硬客户端状态上限。
- `ARTICLE_LOG_RETENTION_LIMIT` 是每篇文章保留的修订快照数量（不含首版），默认 `20`。后端每天本地时间 `00:15` 自动清理超出保留策略的 `article_logs` 快照，清理量记录在 `pruned stale article logs` 日志行的 `deleted` 字段；设为 `0` 或非法值时使用默认值，清理任务不会删除任何文章的首版快照。
- `LOG_RETENTION_DAYS` 是后端按日日志文件（`backend_logs` 卷内）的保留天数，默认 `30`。后端在切换新一天日志文件时自动删除更早的文件；设为 `0` 或非法值时使用默认值。容器 stdout/stderr 日志由 Compose 统一限制为每容器 `10m × 3` 份，无需配置。
- `S3_ENDPOINT` 使用 R2 S3 API 端点，不要填写 R2 公开访问自定义域名。
- `S3_PUBLIC_BASE_URL` 使用图片公开访问域名，和 `S3_ENDPOINT` 分开配置；文章正文图片会使用该 URL。
- `S3_REGION` 使用 `auto`。
- `S3_USE_PATH_STYLE` 生产环境固定为 `false`，让 AWS SDK 使用 R2 需要的虚拟主机风格寻址。
- `S3_BLOG_IMAGE_PREFIX` 和 `S3_ASSET_FILE_PREFIX` 分别控制文章图片和普通资源文件的对象 key 前缀。
- `S3_ACCESS_KEY_ID` 和 `S3_SECRET_ACCESS_KEY` 使用 Cloudflare R2 专用访问密钥，权限限定到 `S3_BUCKET` 的对象读写。
- `SMTP_*` 用于登录邮箱验证码发信。`SMTP_HOST`、`SMTP_USERNAME`、`SMTP_PASSWORD` 配置齐全即启用验证码功能，未配置齐全时自动关闭（登录不触发验证码）；`SMTP_PASSWORD` 使用个人邮箱的 SMTP 授权码（如 QQ 邮箱、163 邮箱授权码或 Gmail 应用密码）。`SMTP_*` 与 `LOGIN_MFA_*` 变量由 `deploy/docker-compose.prod.yml` 的 backend `environment` 传入容器，新增此类变量时必须同步 compose 透传，否则 `production.env` 中的配置不会生效。
- `SMTP_PORT` 默认 `465`（隐式 TLS）；使用 587/25 时走 STARTTLS。
- `SMTP_FROM` 为发件人地址，留空时使用 `SMTP_USERNAME`。
- `LOGIN_MFA_ENABLED` 不设置时跟随 SMTP 配置；显式设为 `false` 可在 SMTP 已配置的情况下关闭验证码功能。
- `LOGIN_MFA_DEV_LOG` 生产环境必须保持 `false` 或不设置；开启会把验证码明文写入后端日志，仅限本地联调。
- 后端容器端口不得直接对公网暴露：登录 IP 基线与限流依赖 Nginx 可信代理覆写的 `X-Real-IP`，绕过 Nginx 直连会破坏该信任边界（攻击者可伪造来源 IP 匹配受害者基线以规避验证码触发）。
- R2 bucket 必须配置 CORS，允许生产前端来源对预签名 URL 发起 `PUT` 和 `GET`，并允许 `Content-Type` 请求头。

R2 bucket CORS JSON 示例：

```json
[
  {
    "AllowedOrigins": [
      "https://cms.rendering.me"
    ],
    "AllowedMethods": [
      "PUT",
      "GET",
      "HEAD"
    ],
    "AllowedHeaders": [
      "Content-Type"
    ],
    "ExposeHeaders": [
      "ETag"
    ],
    "MaxAgeSeconds": 3600
  }
]
```

如果后续生产前端新增域名，需要同步加入 `AllowedOrigins`，例如 `https://admin.example.com`。不要把 `https://rendering.me` 当作后台上传来源，除非实际上传页面运行在该域名下。

### R2 地址配置说明

生产环境有两类 R2 地址，不要混用：

- `S3_ENDPOINT`：R2 S3 API 端点，格式为 `https://<account-id>.r2.cloudflarestorage.com`。后端用它生成预签名上传和下载 URL。
- `S3_PUBLIC_BASE_URL`：图片公开访问域名，例如 `https://assets.rendering.me`。文章编辑器上传正文图片后，接口返回的 `publicUrl` 使用该域名。

文章图片和普通资源文件使用不同对象 key 前缀：

- `S3_BLOG_IMAGE_PREFIX=blog`：文章正文图片上传后保存为 `blog/YYYY/MM/<uuid>.<ext>`，并返回公开访问 URL。
- `S3_ASSET_FILE_PREFIX=assets`：后台普通资源上传后保存为 `assets/YYYY/MM/<uuid>.<ext>`，默认不返回公开 URL，通过下载预签名 URL 访问。

## 首次部署

构建镜像：

```bash
cd /opt/rendering-cms-platform/deploy
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml build
```

启动 PostgreSQL：

```bash
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml up -d postgres
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml ps
```

执行 migration：

```bash
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml --profile migrate run --rm migrate
```

启动应用：

```bash
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml up -d backend frontend
```

检查容器状态：

```bash
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml ps
```

健康检查：

```bash
curl -fsS http://127.0.0.1:3001/api/v1/health
```

期望响应：

```json
{
  "status": "ok"
}
```

## 启用自动部署

首次部署验证通过后，启用 systemd timer 自动部署（每小时整点检查远端提交并自动发布）：

```bash
cd /opt/rendering-cms-platform
sudo bash deploy/setup.sh
```

机制、失败自动回滚策略与手动回滚见 `docs/operations/auto-deploy.md`；日常巡检速查见 `docs/operations/maintenance.md`。

## HTTPS 入口

推荐在宿主机 Nginx 或 Caddy 上终止 HTTPS：

- 现有 Rendering 博客域名转发到 `127.0.0.1:3000`。
- CMS 前端/API 域名转发到 `127.0.0.1:3001`。

完整 Nginx 配置以 `deploy/nginx/rendering.me.conf` 为准。不要从本文档复制局部 Nginx 片段到生产环境，避免片段与完整配置发生漂移。

部署到服务器：

```bash
cd /opt/rendering-cms-platform
sudo cp deploy/nginx/rendering.me.conf /etc/nginx/conf.d/rendering.me.conf
sudo cp deploy/nginx/rendering-cms-proxy-headers.conf /etc/nginx/snippets/rendering-cms-proxy-headers.conf
sudo install -d -m 0755 /var/cache/nginx/rendering-cms
sudo chown www-data:www-data /var/cache/nginx/rendering-cms
sudo nginx -t
sudo systemctl reload nginx
```

执行 `nginx -t` 前，Rendering 仓库必须把博客 location snippet 安装到：

```text
/etc/nginx/snippets/rendering-locations.conf
```

## Cloudflare 橙云真实 IP

站点域名经 Cloudflare 橙云（Proxied）代理时，宿主机 Nginx 的 `$remote_addr` 是 Cloudflare 边缘节点 IP，不是访客真实 IP；登录 IP 基线、登录失败锁定和邮箱验证码触发都会失真。Cloudflare 会在每个代理请求上携带 `CF-Connecting-IP` 头（访客真实 IP），通过 Nginx realip 模块按可信来源还原：

```bash
cd /opt/rendering-cms-platform
sudo cp deploy/nginx/cloudflare-realip.conf /etc/nginx/conf.d/00-cloudflare-realip.conf
sudo nginx -t
sudo systemctl reload nginx
```

- 配置以 `deploy/nginx/cloudflare-realip.conf` 为准，在 http 级生效，博客与 CMS 站点同时受益；`$remote_addr` 被还原后，现有 `X-Real-IP` 传递链路（宿主机 Nginx → 前端容器 Nginx → 后端）无需任何改动。
- 仅信任来自 Cloudflare IP 段的 `CF-Connecting-IP` 头：直连源站伪造该头不会生效。可选加固是用防火墙把 80/443 入站限制到 Cloudflare IP 段，彻底杜绝直连。
- 源站证书使用 Cloudflare Origin CA（Full strict 模式标准做法），仅 Cloudflare 信任，浏览器直连会报证书错误，属预期行为。
- Cloudflare 调整 IP 段后需更新仓库内 `deploy/nginx/cloudflare-realip.conf`（来源 `https://www.cloudflare.com/ips-v4` 与 `/ips-v6`），随下次部署复制到服务器。
- 验证：访问 CMS 后 `sudo tail /var/log/nginx/access.log`，来源应显示访客公网 IP 而非 `172.67.x` / `104.21.x`；换网络登录后台应触发邮箱验证码。

该 snippet 负责博客 HTTPS server 内的 `/blog/`、`/en/blog/` 和唯一默认 `location /`，并分别使用 `rendering_article`、`rendering_general` 限流区。Rendering 仓库不得再安装声明相同 `listen` 或 `server_name` 的完整虚拟主机。安装后使用 `sudo nginx -T | grep -n 'server_name rendering.me'` 确认 HTTPS server 只有一个有效定义。

公开文章缓存只覆盖无 Authorization/Cookie 的列表和详情 GET。搜索、登录、统计、评论、后台和写入请求不进入公共缓存。发布后可用以下命令检查缓存头与状态：

```bash
curl -sSI https://cms.rendering.me/api/v1/articles
curl -sSI https://cms.rendering.me/api/v1/articles/aB3dE9
curl -sSI -H 'Authorization: Bearer invalid-test-token' https://cms.rendering.me/api/v1/articles
```

前两次连续请求应能观察到 `X-Cache-Status` 从 `MISS` 变为 `HIT`；带 Authorization 的请求必须为 `BYPASS` 或不命中缓存。

配置中的证书路径默认写成：

```text
/etc/nginx/ssl/rendering.me/fullchain.pem
/etc/nginx/ssl/rendering.me/privkey.pem
```

如果服务器上的 Cloudflare SSL 证书路径不同，先修改 `deploy/nginx/rendering.me.conf` 中的 `ssl_certificate` 和 `ssl_certificate_key`，再执行 `nginx -t`。

`deploy/nginx/rendering.me.conf` 当前覆盖 `rendering.me`、`www.rendering.me` 和 `cms.rendering.me`。如需调整域名、证书路径、上传大小限制或代理端口，统一修改该完整配置文件。

如果临时不使用宿主机反向代理，可以把 `PUBLIC_HTTP_BIND` 改为 `0.0.0.0:80` 暴露 CMS HTTP。但当前服务器已有 Rendering 博客使用 `3000`，CMS 默认必须保留在 `3001` 或其他未占用端口。R2 预签名 URL 面向浏览器，正式上传下载必须确保 R2 bucket CORS 允许生产前端来源。

## 发布更新

生产服务器已配置 `cms-rendering-deploy.timer` 时，以下流程由 `deploy/auto-deploy.sh` 每小时整点自动检查并执行，详见 `docs/operations/auto-deploy.md`。手动流程仍作为基线 SOP 保留；手动操作前建议先停止 timer：

```bash
sudo systemctl stop cms-rendering-deploy.timer
```

每次发布按固定顺序执行：

1. 拉取目标提交。
2. 构建镜像。
3. 备份 PostgreSQL。
4. 执行 migration。
5. 重启应用容器。
6. 执行 smoke test。

命令：

```bash
cd /opt/rendering-cms-platform
git pull --ff-only

cd deploy
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml build

set -a
. /etc/rendering-cms/production.env
set +a
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" \
  | gzip > "/var/lib/rendering-cms/backups/rendering-cms-$(date +%Y%m%d-%H%M%S).sql.gz"

docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml --profile migrate run --rm migrate
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml up -d backend frontend
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml ps
curl -fsS http://127.0.0.1:3001/api/v1/health
```

## 发布后验收

- 后台管理员可以登录。
- 配置 SMTP 后，从新 IP（或换网络）登录后台会收到邮箱验证码邮件，输入 6 位验证码后可以完成登录；同 IP 再次登录不再要求验证码。
- `LOGIN_MFA_DEV_LOG` 未开启，后端日志不出现验证码明文。
- 仪表盘、文章列表、评论列表和文件列表可以打开。
- 创建文章草稿并保存成功。
- 发布文章后，公开文章读取接口返回已发布内容。
- 提交评论后默认进入待审核状态。
- 审核评论后公开接口只返回已通过评论。
- 上传允许类型文件后，对象进入 Cloudflare R2 bucket。
- 预签名上传和下载 URL 指向 R2 S3 API 端点。
- 在文章编辑器上传图片后，接口返回的 `publicUrl` 指向 `S3_PUBLIC_BASE_URL` 下的 `blog/YYYY/MM/<uuid>.<ext>`。
- 下载链接可以生成，`download_events` 写入审计记录。
- 后端日志持续写入，容器健康状态为 `healthy`。

## 回滚原则

- 如果只涉及应用镜像问题，优先回退 Git 提交或 `APP_IMAGE_TAG`，重新 `build` 与 `up -d`。
- 如果 migration 已经执行，不能直接删除数据库 volume；先按 `docs/operations/restore.md` 评估恢复或补丁 migration。
- 如果对象存储写入异常，先停止后台上传入口，再检查 R2 bucket、CORS、S3 endpoint 和 R2 凭据。
- 回滚前必须保留当前容器日志和数据库备份。
- Nginx 限流误伤时，优先提高对应 location 的 `burst` 或临时移除单个 `limit_req`，不要回滚可信代理头。
- 条件请求兼容异常时可临时关闭后端 ETag 判断，但保留输入限制、列表瘦身和 `Cache-Control`。
- 应用层限流异常时可暂时停止向 Router 传入限流配置并依赖 Nginx，修复后恢复；不得删除 Nginx/CDN 第一层保护。
