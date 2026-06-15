# PostgreSQL 恢复

本文档记录 Rendering CMS Platform 的 PostgreSQL 恢复流程和恢复后检查项。生产命令默认在 Ubuntu 服务器执行，应用目录为 `/opt/rendering-cms-platform`，生产 Docker 入口固定为 `deploy/docker-compose.prod.yml`。

## 恢复前准备

恢复操作会覆盖或改变目标数据库状态。执行前必须完成以下检查：

1. 确认目标环境是生产服务器，且 `deploy/production.env` 指向正确数据库。
2. 备份当前数据库现场。
3. 停止会写入数据库的后端服务或切到维护模式。
4. 确认备份文件来源、时间和完整性。
5. 确认 R2 bucket、R2 访问密钥和对象清单可用。

## 恢复命令

生产环境的 `DATABASE_URL` 默认使用 Docker 网络内的 `postgres:5432`，不要在宿主机直接用该地址执行 `psql`。恢复前先停止后端，避免恢复过程中继续写入：

```bash
cd /opt/rendering-cms-platform/deploy
docker compose --env-file production.env -f docker-compose.prod.yml stop backend
```

普通 SQL 备份恢复：

```bash
set -a
. ./production.env
set +a
docker compose --env-file production.env -f docker-compose.prod.yml exec -T postgres \
  psql -U "$POSTGRES_USER" "$POSTGRES_DB" \
  < ../backups/rendering-cms-latest.sql
```

gzip 压缩备份恢复：

```bash
set -a
. ./production.env
set +a
gzip -dc ../backups/rendering-cms-latest.sql.gz \
  | docker compose --env-file production.env -f docker-compose.prod.yml exec -T postgres \
      psql -U "$POSTGRES_USER" "$POSTGRES_DB"
```

恢复完成后重新启动应用：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml up -d backend frontend
```

本地开发环境如果 `DATABASE_URL` 指向宿主机可访问的 `127.0.0.1:5432`，可以直接执行：

```bash
gzip -dc backups/rendering-cms-latest.sql.gz | psql "$DATABASE_URL"
```

如果需要恢复到空库，先创建新的空数据库，再导入备份。不要在未备份当前现场的情况下直接清空生产库。

## 恢复后检查

恢复完成后按顺序执行 smoke test：

1. 检查健康检查接口：

   ```bash
   curl -fsS http://127.0.0.1:3001/api/v1/health
   ```

2. 登录后台，确认管理员账号可用。
3. 打开文章列表，确认草稿、已发布和归档文章数量符合预期。
4. 打开最近发布文章，确认 MDX 正文、标签、摘要和发布时间正常。
5. 检查评论列表，确认待审核、已通过、已拒绝状态正常。
6. 检查文件列表，确认文件元数据、状态和下载链接生成正常。
7. 检查统计页面，确认今日、近 7 天和热门文章数据可读取。

## 对象存储一致性检查

数据库恢复后应抽样检查文件元数据与 R2 对象是否一致：

- `assets.storage_key` 对应对象能下载。
- 已软删除资源不会出现在公开下载入口。
- 下载审计能继续写入 `download_events`。

使用 AWS CLI 抽样检查 R2 对象是否存在：

```bash
cd /opt/rendering-cms-platform/deploy
set -a
. ./production.env
set +a
aws s3api head-object \
  --endpoint-url "$S3_ENDPOINT" \
  --bucket "$S3_BUCKET" \
  --key "assets/<uuid>/<filename>"
```

如果需要从离线备份恢复 R2 bucket，先保留当前 bucket 清单，再使用 `rclone sync` 或 AWS CLI 将备份目录写回 R2。恢复完成后再次抽样检查 `assets.storage_key`。

## 回滚处理

如果恢复后 smoke test 失败：

1. 保留后端日志和数据库错误信息。
2. 停止继续写入。
3. 判断是否重新恢复到更早备份。
4. 如果是 migration 引起的问题，先在临时库复现，再决定是否执行 down migration 或补丁 migration。
