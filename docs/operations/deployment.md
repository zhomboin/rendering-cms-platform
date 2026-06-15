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
- `deploy/nginx/frontend.conf`：前端 Nginx 容器配置。
- `deploy/nginx/rendering.me.conf`：服务器宿主机 Nginx 完整反向代理配置。
- `backend/Dockerfile`：后端生产镜像。
- `frontend/Dockerfile`：前端生产镜像。
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

### 必须修改的生产配置

生产部署时必须修改 `deploy/production.env`。不要直接使用 `deploy/production.env.example` 中的占位值。

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
- `S3_ENDPOINT` 使用 R2 S3 API 端点，不要填写 R2 公开访问自定义域名。
- `S3_PUBLIC_BASE_URL` 使用图片公开访问域名，和 `S3_ENDPOINT` 分开配置；文章正文图片会使用该 URL。
- `S3_REGION` 使用 `auto`。
- `S3_USE_PATH_STYLE` 生产环境固定为 `false`，让 AWS SDK 使用 R2 需要的虚拟主机风格寻址。
- `S3_BLOG_IMAGE_PREFIX` 和 `S3_ASSET_FILE_PREFIX` 分别控制文章图片和普通资源文件的对象 key 前缀。
- `S3_ACCESS_KEY_ID` 和 `S3_SECRET_ACCESS_KEY` 使用 Cloudflare R2 专用访问密钥，权限限定到 `S3_BUCKET` 的对象读写。
- R2 bucket 必须配置 CORS，允许生产前端来源对预签名 URL 发起 `PUT` 和 `GET`，并允许 `Content-Type` 请求头。

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
curl -fsS http://127.0.0.1:3001/api/v1/health
```

期望响应：

```json
{
  "status": "ok"
}
```

## HTTPS 入口

推荐在宿主机 Nginx 或 Caddy 上终止 HTTPS：

- 现有 Rendering 博客域名转发到 `127.0.0.1:3000`。
- CMS 前端/API 域名转发到 `127.0.0.1:3001`。

完整 Nginx 配置以 `deploy/nginx/rendering.me.conf` 为准。不要从本文档复制局部 Nginx 片段到生产环境，避免片段与完整配置发生漂移。

部署到服务器：

```bash
cd /opt/rendering-cms-platform
sudo cp deploy/nginx/rendering.me.conf /etc/nginx/conf.d/rendering.me.conf
sudo nginx -t
sudo systemctl reload nginx
```

配置中的证书路径默认写成：

```text
/etc/nginx/ssl/rendering.me/fullchain.pem
/etc/nginx/ssl/rendering.me/privkey.pem
```

如果服务器上的 Cloudflare SSL 证书路径不同，先修改 `deploy/nginx/rendering.me.conf` 中的 `ssl_certificate` 和 `ssl_certificate_key`，再执行 `nginx -t`。

`deploy/nginx/rendering.me.conf` 当前覆盖 `rendering.me`、`www.rendering.me` 和 `cms.rendering.me`。如需调整域名、证书路径、上传大小限制或代理端口，统一修改该完整配置文件。

如果临时不使用宿主机反向代理，可以把 `PUBLIC_HTTP_BIND` 改为 `0.0.0.0:80` 暴露 CMS HTTP。但当前服务器已有 Rendering 博客使用 `3000`，CMS 默认必须保留在 `3001` 或其他未占用端口。R2 预签名 URL 面向浏览器，正式上传下载必须确保 R2 bucket CORS 允许生产前端来源。

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
curl -fsS http://127.0.0.1:3001/api/v1/health
```

## 发布后验收

- 后台管理员可以登录。
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
