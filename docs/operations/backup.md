# PostgreSQL 备份

本文档记录 Rendering CMS Platform 的 PostgreSQL 备份要求和手动备份流程。生产命令默认在 Ubuntu 服务器执行，应用目录为 `/opt/rendering-cms-platform`，生产 Docker 入口固定为 `deploy/docker-compose.prod.yml`。

## 备份时机

- 每次生产 migration 前必须备份 PostgreSQL。
- 每次大版本发布前必须备份 PostgreSQL。
- 手动修改生产数据前必须备份 PostgreSQL。
- 发现异常写入、误删除或需要排查数据问题时，应先备份当前现场。

## 目录准备

备份文件建议统一放在应用目录下的 `backups/`，该目录不应提交到 Git。

```bash
cd /opt/rendering-cms-platform
mkdir -p backups
chmod 700 backups
```

## 手动备份

生产环境的 `DATABASE_URL` 默认使用 Docker 网络内的 `postgres:5432`，不要在宿主机直接用该地址执行 `pg_dump`。应通过 Compose 进入 `postgres` 服务导出：

```bash
cd /opt/rendering-cms-platform/deploy
set -a
. ./production.env
set +a
mkdir -p ../backups
chmod 700 ../backups
docker compose --env-file production.env -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" \
  > "../backups/rendering-cms-$(date +%Y%m%d-%H%M%S).sql"
```

如果希望压缩备份文件：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" \
  | gzip > "../backups/rendering-cms-$(date +%Y%m%d-%H%M%S).sql.gz"
```

本地开发环境如果 `DATABASE_URL` 指向宿主机可访问的 `127.0.0.1:5432`，可以直接执行：

```bash
pg_dump "$DATABASE_URL" | gzip > "backups/rendering-cms-$(date +%Y%m%d-%H%M%S).sql.gz"
```

## 备份校验

备份完成后至少检查文件是否存在且非空：

```bash
ls -lh ../backups/rendering-cms-*.sql*
test -s "$(ls -t ../backups/rendering-cms-*.sql* | head -n 1)"
```

如果使用压缩备份，应检查 gzip 文件完整性：

```bash
gzip -t "$(ls -t ../backups/rendering-cms-*.sql.gz | head -n 1)"
```

## 保留策略

- 至少保留最近 7 天备份。
- 生产环境建议额外保留每周和每月关键备份。
- 删除旧备份前必须确认最近一次备份可用。
- 备份文件包含业务数据和后台用户信息，应限制访问权限，不要提交到 Git。

## 对象存储备份

PostgreSQL 只保存上传文件元数据和 object key，不保存文件本体。当前生产对象存储暂时使用服务器本机 MinIO，因此 PostgreSQL 备份不能替代 MinIO 数据备份。

- MinIO 数据保存在 Docker volume `rendering_cms_minio_data`。
- 生产服务器应对该 volume 所在磁盘做快照或文件级备份。
- 如后续迁移到 Cloudflare R2、AWS S3 或其他托管对象存储，迁移前应先导出 bucket 清单。
- 恢复数据库后必须确认 `assets.storage_key` 对应对象仍存在。

使用 `mc mirror` 导出当前 bucket：

```bash
cd /opt/rendering-cms-platform/deploy
set -a
. ./production.env
set +a
MINIO_BACKUP_DIR="minio-$(date +%Y%m%d-%H%M%S)"
mkdir -p "../backups/$MINIO_BACKUP_DIR"
docker run --rm --network rendering_cms \
  -e MINIO_ROOT_USER \
  -e MINIO_ROOT_PASSWORD \
  -e MINIO_BUCKET \
  -e MINIO_BACKUP_DIR \
  -v "$(cd ../backups && pwd):/backups" \
  minio/mc:latest \
  sh -c 'mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" && mc mirror "local/$MINIO_BUCKET" "/backups/$MINIO_BACKUP_DIR/$MINIO_BUCKET"'
```

如果使用服务器磁盘快照，应同时覆盖 Docker volume `rendering_cms_postgres_data` 和 `rendering_cms_minio_data`，并记录快照时间点。
