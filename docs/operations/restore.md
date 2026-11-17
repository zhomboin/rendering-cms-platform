# PostgreSQL 恢复

本文档记录 Rendering CMS Platform 的 PostgreSQL 恢复流程和恢复后检查项。生产命令默认在 Ubuntu 服务器执行，应用目录为 `/opt/rendering-cms-platform`，生产 Docker 入口固定为 `deploy/docker-compose.prod.yml`。

## 恢复前准备

1. 按 [生产访问规范](production-access.md)确认目标、授权与回退方案。暂停自动部署，确认没有发布或回滚任务正在运行。
2. 按 [备份流程](backup.md#手动备份)备份当前数据库，并核对待恢复文件的来源、时间、校验值和权限。
3. 确认 R2 对象清单和读取权限可用。完整 SQL 备份应导入一个经批准的空库，避免与现有表及数据冲突。准备足够空间用于解压和导入。

## 恢复命令

进入部署目录，停止后端和其它写入目标库的进程。

```bash
cd /opt/rendering-cms-platform/deploy
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml stop backend
```

设置 `RESTORE_BACKUP` 为已核对的 SQL 或 gzip 文件绝对路径，`RESTORE_DB` 为 PostgreSQL 容器中的目标空库。压缩备份完整解压后再导入，普通 SQL 直接导入。

```bash
(
  set -euo pipefail
  umask 077
  test -s "$RESTORE_BACKUP"
  restore_sql="$RESTORE_BACKUP"
  if [[ "$RESTORE_BACKUP" == *.gz ]]; then
    gzip -t "$RESTORE_BACKUP"
    restore_sql=$(mktemp /var/lib/rendering-cms/backups/.restore.XXXXXX)
    trap 'rm -f -- "$restore_sql"' EXIT
    gzip -dc -- "$RESTORE_BACKUP" > "$restore_sql"
    test -s "$restore_sql"
  fi
  docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml exec -T postgres \
    sh -c 'psql -X -v ON_ERROR_STOP=1 --single-transaction --file=- -U "$POSTGRES_USER" -d "$1"' \
    sh "$RESTORE_DB" < "$restore_sql"
)
```

任一 SQL 出错会回滚本次导入。命令非零退出时保持后端停机，保留错误日志并排查。导入成功后核对目标库内容，确认生产 `DATABASE_URL` 指向已批准的库。将核对过的应用镜像标签设置为 `APP_IMAGE_TAG`，再恢复应用。

```bash
APP_IMAGE_TAG="${APP_IMAGE_TAG:?请设置已核对的应用镜像标签}" \
  docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml up -d --no-build --pull never backend frontend
```

需要覆盖原库并同步回退代码时，使用 [回滚脚本](auto-deploy.md#手动回滚)中经批准的 `--commit` 与具体 `--backup` 文件。脚本会停止后端并删除、重建目标数据库。SQL 导入失败时脚本退出并保持后端停机。

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
7. 检查首页访问趋势、文章与相册排行，并抽查相册照片、封面及 R2 访问。

## 对象存储一致性检查

数据库恢复后应抽样检查文件元数据与 R2 对象是否一致：

- `assets.storage_key` 对应对象能下载。
- 已软删除资源不会出现在公开下载入口。
- 下载审计能继续写入 `download_events`。

使用 AWS CLI 抽样检查 R2 对象是否存在：

```bash
cd /opt/rendering-cms-platform/deploy
set -a
. /etc/rendering-cms/production.env
set +a
aws s3api head-object \
  --endpoint-url "$S3_ENDPOINT" \
  --bucket "$S3_BUCKET" \
  --key "$RESTORE_OBJECT_KEY"
```

如果需要从离线备份恢复 R2 bucket，先保留当前 bucket 清单，再使用 `rclone sync` 或 AWS CLI 将备份目录写回 R2。恢复完成后再次抽样检查 `assets.storage_key`。

## 回滚处理

如果恢复后 smoke test 失败：

1. 保留后端日志和数据库错误信息。
2. 停止继续写入。
3. 判断是否重新恢复到更早备份。
4. 如果是 migration 引起的问题，先在临时库复现，再决定是否执行 down migration 或补丁 migration。
