# 生产运维 SOP

本文档是 Rendering CMS Platform 生产环境的固定运维 SOP。命令默认在 Ubuntu 服务器执行，生产 Docker 入口固定为 `deploy/docker-compose.prod.yml`。

## 基本约定

- 应用目录：`/opt/rendering-cms-platform`。
- Compose 目录：`deploy/`。
- 生产环境变量：`deploy/production.env`，权限必须为 `600`。
- 生产入口端口：默认由 `frontend` 容器绑定 `127.0.0.1:3000`。
- 公网 HTTPS：由宿主机 Nginx、Caddy 或负载均衡器转发到 `127.0.0.1:3000`。
- 备份目录：`backups/`，不得提交到 Git。

## 日常巡检

每日巡检：

```bash
cd /opt/rendering-cms-platform/deploy
docker compose --env-file production.env -f docker-compose.prod.yml ps
curl -fsS http://127.0.0.1:3000/api/v1/health
docker compose --env-file production.env -f docker-compose.prod.yml logs --tail=100 backend
df -h
```

检查项：

- `postgres`、`backend`、`frontend` 均处于运行状态。
- `backend` 健康检查为 `healthy`。
- `/api/v1/health` 返回 `{"status":"ok"}`。
- 磁盘空间充足，备份目录和 Docker volume 所在磁盘没有接近满盘。
- 对象存储 bucket 可访问，上传和下载凭据没有过期。
- 评论待审核队列没有异常堆积。
- 登录失败次数没有异常升高。

## 发布 SOP

发布前确认当前提交：

```bash
cd /opt/rendering-cms-platform
git status --short --branch
git log -1 --oneline
```

拉取目标提交：

```bash
git pull --ff-only
```

构建镜像：

```bash
cd /opt/rendering-cms-platform/deploy
docker compose --env-file production.env -f docker-compose.prod.yml build
```

备份 PostgreSQL：

```bash
set -a
. ./production.env
set +a
mkdir -p ../backups
chmod 700 ../backups
docker compose --env-file production.env -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" \
  | gzip > "../backups/rendering-cms-$(date +%Y%m%d-%H%M%S).sql.gz"
gzip -t "$(ls -t ../backups/rendering-cms-*.sql.gz | head -n 1)"
```

执行 migration：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml --profile migrate run --rm migrate
```

重启应用：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml up -d backend frontend
docker compose --env-file production.env -f docker-compose.prod.yml ps
```

发布后 smoke test：

```bash
curl -fsS http://127.0.0.1:3000/api/v1/health
```

人工验收：

- 登录后台。
- 创建并保存文章草稿。
- 发布文章并检查公开读取。
- 提交评论并完成审核。
- 上传小文件并生成下载链接。
- 打开统计页面并确认数据可读取。
- 查看后端日志，确认有新的 `http_request` 记录。

## 停止和启动

停止应用容器，保留数据库：

```bash
cd /opt/rendering-cms-platform/deploy
docker compose --env-file production.env -f docker-compose.prod.yml stop backend frontend
```

启动应用容器：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml up -d backend frontend
```

停止全部容器，保留 volume：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml stop
```

不要在生产环境执行 `docker compose down -v`，该命令会删除 PostgreSQL 数据 volume。

## 日志查看

查看后端日志：

```bash
cd /opt/rendering-cms-platform/deploy
docker compose --env-file production.env -f docker-compose.prod.yml logs -f backend
```

查看前端 Nginx 日志：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml logs -f frontend
```

导出最近日志：

```bash
mkdir -p ../tmp
docker compose --env-file production.env -f docker-compose.prod.yml logs --since=1h backend frontend > ../tmp/rendering-cms-logs-$(date +%Y%m%d-%H%M%S).log
```

## 备份 SOP

手动备份：

```bash
cd /opt/rendering-cms-platform/deploy
set -a
. ./production.env
set +a
mkdir -p ../backups
chmod 700 ../backups
docker compose --env-file production.env -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" \
  | gzip > "../backups/rendering-cms-$(date +%Y%m%d-%H%M%S).sql.gz"
gzip -t "$(ls -t ../backups/rendering-cms-*.sql.gz | head -n 1)"
```

保留策略：

- 至少保留最近 7 天备份。
- 大版本发布前的备份单独保留。
- 删除旧备份前必须确认最新备份可用。
- 对象存储文件本体不在 PostgreSQL 备份中，bucket 需要单独开启版本、复制或生命周期保护。

## 恢复 SOP

恢复前必须：

1. 确认 `DATABASE_URL` 指向生产数据库。
2. 先备份当前现场。
3. 停止 `backend`，避免恢复过程中继续写入。
4. 校验待恢复备份文件完整性。

停止后端：

```bash
cd /opt/rendering-cms-platform/deploy
docker compose --env-file production.env -f docker-compose.prod.yml stop backend
```

恢复 gzip 备份：

```bash
set -a
. ./production.env
set +a
gzip -dc ../backups/rendering-cms-latest.sql.gz \
  | docker compose --env-file production.env -f docker-compose.prod.yml exec -T postgres \
      psql -U "$POSTGRES_USER" "$POSTGRES_DB"
```

恢复后启动并验收：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml up -d backend frontend
curl -fsS http://127.0.0.1:3000/api/v1/health
```

更多恢复细节见 `docs/operations/restore.md`。

## 常见故障处理

### 健康检查失败

```bash
cd /opt/rendering-cms-platform/deploy
docker compose --env-file production.env -f docker-compose.prod.yml ps
docker compose --env-file production.env -f docker-compose.prod.yml logs --tail=200 backend
docker compose --env-file production.env -f docker-compose.prod.yml logs --tail=100 postgres
```

排查顺序：

1. 检查 `backend` 是否能连接 `postgres`。
2. 检查 `JWT_SECRET`、`DATABASE_URL`、`FRONTEND_ORIGINS` 和 `S3_*` 是否存在。
3. 检查 migration 是否已经执行。
4. 检查对象存储凭据是否导致后端启动失败。

### 前端可打开但 API 失败

排查顺序：

1. 请求 `curl -fsS http://127.0.0.1:3000/api/v1/health`。
2. 查看 `frontend` Nginx 日志。
3. 查看 `backend` 日志。
4. 确认 `deploy/nginx/frontend.conf` 中 `/api/v1/` 代理仍指向 `backend:8080`。
5. 如果通过其他域名访问，确认 `FRONTEND_ORIGINS` 包含该完整来源。

### 登录失败次数异常

排查顺序：

1. 检查后端日志中的登录失败来源。
2. 检查 `login_attempts` 是否短时间高频增长。
3. 在宿主机 Nginx、Caddy 或网络层临时封禁异常来源。
4. 保留日志后再决定是否调整限流策略。

### 文件上传或下载失败

排查顺序：

1. 检查 `S3_ENDPOINT`、`S3_REGION`、`S3_BUCKET`、`S3_ACCESS_KEY_ID`、`S3_SECRET_ACCESS_KEY`。
2. 检查 bucket CORS、对象权限和生命周期策略。
3. 检查上传大小是否超过 `20MB`。
4. 检查文件类型是否在允许列表内。
5. 检查 `download_events` 是否能写入。

## 禁止操作

- 不要执行 `docker compose down -v`。
- 不要删除 `rendering_cms_postgres_data` volume。
- 不要把 `deploy/production.env`、备份文件或对象存储密钥提交到 Git。
- 不要在未备份生产数据库的情况下执行 migration。
- 不要直接修改生产数据库数据，除非已经完成备份并记录操作原因。

## 关联文档

- `docs/operations/deployment.md`
- `docs/operations/backup.md`
- `docs/operations/restore.md`
- `docs/operations/observability.md`
