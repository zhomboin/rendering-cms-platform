# PostgreSQL 恢复

本文档记录 Rendering CMS Platform 的 PostgreSQL 恢复流程和恢复后检查项。命令默认在 WSL Ubuntu 24.04 或 Linux 服务器中执行。

## 恢复前准备

恢复操作会覆盖或改变目标数据库状态。执行前必须完成以下检查：

1. 确认目标环境和 `DATABASE_URL` 指向正确数据库。
2. 备份当前数据库现场。
3. 停止会写入数据库的后端服务或切到维护模式。
4. 确认备份文件来源、时间和完整性。
5. 确认对象存储 bucket 未被误清理。

## 恢复命令

普通 SQL 备份恢复：

```bash
psql "$DATABASE_URL" < backups/rendering-cms-latest.sql
```

gzip 压缩备份恢复：

```bash
gzip -dc backups/rendering-cms-latest.sql.gz | psql "$DATABASE_URL"
```

如果需要恢复到空库，先创建新的空数据库，再导入备份。不要在未备份当前现场的情况下直接清空生产库。

## 恢复后检查

恢复完成后按顺序执行 smoke test：

1. 检查健康检查接口：

   ```bash
   curl -fsS http://127.0.0.1:8080/api/v1/health
   ```

2. 登录后台，确认管理员账号可用。
3. 打开文章列表，确认草稿、已发布和归档文章数量符合预期。
4. 打开最近发布文章，确认 MDX 正文、标签、摘要和发布时间正常。
5. 检查评论列表，确认待审核、已通过、已拒绝状态正常。
6. 检查文件列表，确认文件元数据、状态和下载链接生成正常。
7. 检查统计页面，确认今日、近 7 天和热门文章数据可读取。

## 对象存储一致性检查

数据库恢复后应抽样检查文件元数据与对象存储是否一致：

- `assets.storage_key` 对应对象能下载。
- 已软删除资源不会出现在公开下载入口。
- 下载审计能继续写入 `download_events`。

## 回滚处理

如果恢复后 smoke test 失败：

1. 保留后端日志和数据库错误信息。
2. 停止继续写入。
3. 判断是否重新恢复到更早备份。
4. 如果是 migration 引起的问题，先在临时库复现，再决定是否执行 down migration 或补丁 migration。

