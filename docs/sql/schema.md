# 数据库 Schema

MVP 使用 PostgreSQL 作为唯一运行时数据源。文章、评论、统计、文件元数据和后台用户都进入数据库管理；上传文件本体存储在 S3 兼容对象存储中，数据库只保存元数据和 `storage_key`。

## 扩展与枚举

- `pgcrypto`：用于 `gen_random_uuid()` 生成 UUID 主键。
- `user_role`：后台用户角色，当前包含 `admin`、`editor`。
- `article_status`：文章状态，当前包含 `draft`、`published`、`archived`。
- `comment_status`：评论审核状态，当前包含 `pending`、`approved`、`rejected`。

## 核心表

- `users`：后台用户表，保存邮箱、名称、密码哈希、角色和时间戳。
- `articles`：文章主表，保存 slug、标题、摘要、MDX 正文、状态、标签、精选标记、封面地址、发布时间和作者。
- `article_revisions`：文章修订历史表，每次草稿保存和发布都应写入一条记录。
- `comments`：评论表，评论默认状态为 `pending`，后台审核后进入 `approved` 或 `rejected`。
- `article_view_daily`：文章日访问量聚合表，以 `article_id` 和 `view_date` 作为联合主键。
- `site_view_daily`：站点日访问量聚合表，以 `view_date` 作为主键。
- `assets`：上传文件元数据表，保存文件名、内容类型、字节数、对象存储 key、可选公开地址、创建人和创建时间。
- `download_events`：文件下载审计表，记录资产、访问端哈希、User-Agent 和发生时间。

## 关联关系

- `articles.author_id` 引用 `users.user_id`。
- `article_revisions.article_id` 引用 `articles.article_id`，文章删除时级联删除修订记录。
- `article_revisions.created_by` 引用 `users.user_id`。
- `comments.article_id` 引用 `articles.article_id`，文章删除时级联删除评论。
- `article_view_daily.article_id` 引用 `articles.article_id`，文章删除时级联删除统计聚合。
- `assets.created_by` 引用 `users.user_id`。
- `download_events.asset_id` 引用 `assets.asset_id`，资产删除时级联删除下载审计。

## 隐私规则

- 不保存原始 IP 地址。
- 评论表 `comments` 只保存 `ip_hash`。
- 下载审计表 `download_events` 只保存 `ip_hash`。
- 如后续需要风控、限流或去重，必须在进入数据库前完成 IP 哈希处理。
- `author_email` 属于评论提交者隐私信息，只用于后台审核和必要联系，不应在公开接口中默认返回。

## sqlc 入口

- 配置文件：`backend/sqlc.yaml`。
- schema 来源：`backend/migrations/`。
- 查询来源：`backend/sql/`。
- 生成包路径：`backend/internal/database/dbgen`。
