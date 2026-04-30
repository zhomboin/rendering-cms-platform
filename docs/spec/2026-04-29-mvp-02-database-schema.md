# MVP 阶段 02：数据库 Schema、sqlc 和连接

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

建立 PostgreSQL 作为 CMS 运行时数据源，定义文章、用户、评论、统计、文件和下载审计的基础表结构，并准备 sqlc 代码生成入口。

## 文件范围

- Create: `backend/internal/database/db.go`
- Create: `backend/migrations/000001_init.up.sql`
- Create: `backend/migrations/000001_init.down.sql`
- Create: `backend/sqlc.yaml`
- Create: `backend/sql/users.sql`
- Create: `backend/sql/articles.sql`
- Create: `backend/sql/analytics.sql`
- Create: `backend/sql/comments.sql`
- Create: `backend/sql/assets.sql`
- Create: `docs/sql/schema.md`

## 子任务

- [x] 创建 `backend/migrations/000001_init.up.sql`。
- [x] 启用 `pgcrypto` 扩展，用于 `gen_random_uuid()`。
- [x] 创建枚举：`user_role`、`article_status`、`comment_status`。
- [x] 创建 `users` 表，字段包含 `user_id`、`email`、`name`、`password_hash`、`role`、时间戳。
- [x] 创建 `articles` 表，字段包含 `article_id`、`slug`、`title`、`summary`、`body_mdx`、`status`、`tags`、`featured`、`cover_image_url`、`published_at`、`author_id`、时间戳。
- [x] 创建 `article_revisions` 表，保存每次草稿保存和发布记录。
- [x] 创建 `comments` 表，评论默认状态为 `pending`，只保存 `ip_hash`，不保存原始 IP。
- [x] 创建 `article_view_daily` 和 `site_view_daily`，采用日聚合统计。
- [x] 创建 `assets` 表，保存文件元数据和 `storage_key`。
- [x] 创建 `download_events` 表，记录下载审计。
- [x] 创建 `backend/migrations/000001_init.down.sql`，按依赖反向删除表和枚举。
- [x] 创建 `backend/sqlc.yaml`，输出包为 `backend/internal/database/dbgen`。
- [x] 创建各业务 SQL 文件占位入口：`users.sql`、`articles.sql`、`analytics.sql`、`comments.sql`、`assets.sql`。
- [x] 创建 `backend/internal/database/db.go`，封装 `pgxpool.New()`。
- [x] 创建 `docs/sql/schema.md`，用中文说明表职责和隐私规则。
- [x] 运行 `cd backend && go test ./...`。

## 完成记录

- 完成时间：2026-04-30。
- 已创建 MVP 核心 PostgreSQL schema、反向 migration、sqlc 配置和业务查询入口。
- 已通过 `sqlc generate` 生成 `backend/internal/database/dbgen`。
- 已补充 `database.Open()` 单元测试。
- 验证命令已通过：`cd backend && go test ./...`、`cd backend && sqlc generate`。

## 验收标准

- migration 包含 MVP 所需全部核心表。
- `down` migration 可以反向删除本阶段创建的对象。
- `sqlc.yaml` 指向 `migrations/` 和 `sql/`。
- schema 文档明确“不保存原始 IP”。

## 建议提交

```bash
git add backend/migrations backend/sqlc.yaml backend/sql backend/internal/database docs/sql/schema.md
git commit -m "feat: add cms database schema"
```
