# 生产 Docker 部署方案

本文档是 Rendering CMS Platform 的生产 Docker 部署入口。命令默认在 Ubuntu 服务器中执行，使用 `docker compose`，不使用旧版 `docker-compose`。

## 部署拓扑

生产 Docker 版本包含以下服务：

- `frontend`：Nginx 容器，托管 Vite 构建产物，并把 `/api/v1/` 反向代理到后端。
- `backend`：Go 后端容器，监听容器内 `0.0.0.0:8080`。
- `postgres`：PostgreSQL 16 容器，数据保存在 Docker volume `rendering_cms_postgres_data`。
- `migrate`：一次性 migration 容器，仅在发布时手动执行。

推荐公网入口：

```text
用户浏览器
  -> https://cms.example.com
  -> 服务器 Nginx/Caddy/负载均衡器
  -> 127.0.0.1:3000
  -> frontend 容器
  -> backend 容器
  -> postgres 容器
```

生产对象存储推荐使用 Cloudflare R2、AWS S3 或同类托管 S3 兼容服务。MinIO 更适合作为本地开发依赖，不作为默认生产入口。

## 文件入口

- `deploy/docker-compose.prod.yml`：生产 Compose 文件。
- `deploy/production.env.example`：生产环境变量模板。
- `deploy/nginx/frontend.conf`：前端 Nginx 容器配置。
- `backend/Dockerfile`：后端生产镜像。
- `frontend/Dockerfile`：前端生产镜像。
- `docs/operations/runbook.md`：固定运维 SOP。
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
mkdir -p backups
chmod 700 backups
```

## 环境变量

复制生产环境模板：

```bash
cd /opt/rendering-cms-platform/deploy
cp production.env.example production.env
chmod 600 production.env
```

必须修改以下值：

```env
APP_IMAGE_TAG=latest
PUBLIC_HTTP_BIND=127.0.0.1:3000

POSTGRES_DB=rendering_cms
POSTGRES_USER=rendering
POSTGRES_PASSWORD=replace-with-strong-database-password
DATABASE_URL=postgres://rendering:replace-with-strong-database-password@postgres:5432/rendering_cms?sslmode=disable

JWT_SECRET=replace-with-32-plus-character-secret
FRONTEND_ORIGIN=https://cms.example.com
FRONTEND_ORIGINS=https://cms.example.com
VITE_API_BASE=/api/v1

S3_ENDPOINT=https://example.r2.cloudflarestorage.com
S3_REGION=auto
S3_BUCKET=rendering-assets
S3_ACCESS_KEY_ID=replace-me
S3_SECRET_ACCESS_KEY=replace-me
```

配置要求：

- `POSTGRES_PASSWORD` 必须使用强密码；如果密码包含特殊字符，`DATABASE_URL` 中的密码段必须 URL 编码。
- `DATABASE_URL` 在容器网络内连接 `postgres:5432`，不要写成 `127.0.0.1`。
- `JWT_SECRET` 至少 32 字符，生产环境不得使用示例值。
- `FRONTEND_ORIGINS` 使用生产访问域名；多个来源使用英文逗号分隔。
- `VITE_API_BASE` 保持 `/api/v1`，让浏览器走同域 API 反代。
- `S3_*` 必须指向真实生产对象存储，bucket 权限和 CORS 需要单独在对象存储侧配置。

## 首次部署

构建镜像：

```bash
cd /opt/rendering-cms-platform/deploy
docker compose --env-file production.env -f docker-compose.prod.yml build
```

启动 PostgreSQL：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml up -d postgres
docker compose --env-file production.env -f docker-compose.prod.yml ps
```

执行 migration：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml --profile migrate run --rm migrate
```

启动应用：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml up -d backend frontend
```

检查容器状态：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml ps
```

健康检查：

```bash
curl -fsS http://127.0.0.1:3000/api/v1/health
```

期望响应：

```json
{
  "status": "ok"
}
```

## HTTPS 入口

推荐在宿主机 Nginx 或 Caddy 上终止 HTTPS，再转发到 `127.0.0.1:3000`。

Nginx 示例：

```nginx
server {
  listen 80;
  server_name cms.example.com;
  return 301 https://$host$request_uri;
}

server {
  listen 443 ssl http2;
  server_name cms.example.com;

  ssl_certificate /etc/letsencrypt/live/cms.example.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/cms.example.com/privkey.pem;

  client_max_body_size 20m;

  location / {
    proxy_pass http://127.0.0.1:3000;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }
}
```

如果临时不使用宿主机反向代理，可以把 `PUBLIC_HTTP_BIND` 改为 `0.0.0.0:80` 暴露 HTTP。但生产正式访问必须配置 HTTPS。

## 发布更新

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
docker compose --env-file production.env -f docker-compose.prod.yml build

set -a
. ./production.env
set +a
docker compose --env-file production.env -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" \
  | gzip > "../backups/rendering-cms-$(date +%Y%m%d-%H%M%S).sql.gz"

docker compose --env-file production.env -f docker-compose.prod.yml --profile migrate run --rm migrate
docker compose --env-file production.env -f docker-compose.prod.yml up -d backend frontend
docker compose --env-file production.env -f docker-compose.prod.yml ps
curl -fsS http://127.0.0.1:3000/api/v1/health
```

## 发布后验收

- 后台管理员可以登录。
- 仪表盘、文章列表、评论列表和文件列表可以打开。
- 创建文章草稿并保存成功。
- 发布文章后，公开文章读取接口返回已发布内容。
- 提交评论后默认进入待审核状态。
- 审核评论后公开接口只返回已通过评论。
- 上传允许类型文件后，对象进入 S3 兼容存储。
- 下载链接可以生成，`download_events` 写入审计记录。
- 后端日志持续写入，容器健康状态为 `healthy`。

## 回滚原则

- 如果只涉及应用镜像问题，优先回退 Git 提交或 `APP_IMAGE_TAG`，重新 `build` 与 `up -d`。
- 如果 migration 已经执行，不能直接删除数据库 volume；先按 `docs/operations/restore.md` 评估恢复或补丁 migration。
- 如果对象存储写入异常，先停止后台上传入口，再检查 bucket 权限、CORS 和凭据。
- 回滚前必须保留当前容器日志和数据库备份。
