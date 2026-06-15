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

PostgreSQL 只保存上传文件元数据和 object key，不保存文件本体。生产对象存储使用 Cloudflare R2，因此 PostgreSQL 备份不能替代 R2 对象清单和对象数据备份。

- R2 bucket 名称由 `S3_BUCKET` 指定。
- 每次重大发布或对象存储迁移前，应导出 bucket 对象清单，至少包含 object key、大小和更新时间。
- 如需跨供应商备份，可使用 `rclone` 或 AWS CLI 配置 R2 endpoint，将 R2 bucket 同步到另一处对象存储或离线备份目录。
- 恢复数据库后必须确认 `assets.storage_key` 对应对象仍存在。

使用 AWS CLI 导出 R2 对象清单示例：

```bash
cd /opt/rendering-cms-platform/deploy
set -a
. ./production.env
set +a
aws s3api list-objects-v2 \
  --endpoint-url "$S3_ENDPOINT" \
  --bucket "$S3_BUCKET" \
  --output json \
  > "../backups/r2-objects-$(date +%Y%m%d-%H%M%S).json"
```

使用 `rclone` 同步 R2 bucket 示例：

```bash
rclone sync "r2:$S3_BUCKET" "../backups/r2-$S3_BUCKET-$(date +%Y%m%d-%H%M%S)" --progress
```

执行跨存储备份前应先在服务器上配置只读或最小权限的 R2 访问凭据。若使用服务器磁盘快照，只能覆盖 PostgreSQL volume，不能替代 R2 对象备份。
