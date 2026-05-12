# PostgreSQL 备份

本文档记录 Rendering CMS Platform 的 PostgreSQL 备份要求和手动备份流程。命令默认在 WSL Ubuntu 24.04 或 Linux 服务器中执行。

## 备份时机

- 每次生产 migration 前必须备份 PostgreSQL。
- 每次大版本发布前必须备份 PostgreSQL。
- 手动修改生产数据前必须备份 PostgreSQL。
- 发现异常写入、误删除或需要排查数据问题时，应先备份当前现场。

## 目录准备

备份文件建议统一放在 `backups/`，该目录不应提交到 Git。

```bash
mkdir -p backups
chmod 700 backups
```

## 手动备份

使用 `pg_dump` 基于 `DATABASE_URL` 导出 SQL 备份：

```bash
pg_dump "$DATABASE_URL" > "backups/rendering-cms-$(date +%Y%m%d-%H%M%S).sql"
```

如果希望压缩备份文件：

```bash
pg_dump "$DATABASE_URL" | gzip > "backups/rendering-cms-$(date +%Y%m%d-%H%M%S).sql.gz"
```

## 备份校验

备份完成后至少检查文件是否存在且非空：

```bash
ls -lh backups/rendering-cms-*.sql*
test -s "$(ls -t backups/rendering-cms-*.sql* | head -n 1)"
```

如果使用压缩备份，应检查 gzip 文件完整性：

```bash
gzip -t "$(ls -t backups/rendering-cms-*.sql.gz | head -n 1)"
```

## 保留策略

- 至少保留最近 7 天备份。
- 生产环境建议额外保留每周和每月关键备份。
- 删除旧备份前必须确认最近一次备份可用。
- 备份文件包含业务数据和后台用户信息，应限制访问权限，不要提交到 Git。

## 对象存储备份

PostgreSQL 只保存上传文件元数据和 object key，不保存文件本体。对象存储文件需要单独保护：

- Cloudflare R2、AWS S3 或 MinIO 应开启 bucket 版本、复制或生命周期策略。
- 迁移对象存储前应先导出 bucket 清单。
- 恢复数据库后必须确认 `assets.storage_key` 对应对象仍存在。

