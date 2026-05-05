# 数据库 Schema

MVP 使用 PostgreSQL 作为唯一运行时数据源。文章、评论、统计、文件元数据和后台用户都进入数据库管理；上传文件本体存储在 S3 兼容对象存储中，数据库只保存元数据和 `storage_key`。

## 扩展与枚举

- `pgcrypto`：用于 `gen_random_uuid()` 生成 UUID 主键。
- `user_role`：后台用户角色，当前包含 `admin`、`editor`。
- `article_status`：文章状态，当前包含 `draft`、`published`、`archived`。
- `comment_status`：评论审核状态，当前包含 `pending`、`approved`、`rejected`。

## 核心表

- `users`：后台用户表，保存邮箱、名称、密码哈希、角色和时间戳。
- `articles`：文章主表，保存 slug、标题、摘要、MDX 正文、状态、标签、精选标记、封面地址、发布时间、作者和版本号。
- `article_logs`：文章版本日志表，字段与 `articles` 保持一致，以 `article_id` 和 `version` 作为联合主键。
- `comments`：评论表，评论默认状态为 `pending`，后台审核后进入 `approved` 或 `rejected`。
- `article_view_daily`：文章当日访问量聚合表，只保留当天写入中的访问量，以 `article_id` 和 `view_date` 作为联合主键。
- `article_view_history`：文章历史日访问量表，每天统计完成后把当日访问量归档到该表。
- `site_view_daily`：站点当日访问量聚合表，只保留当天写入中的访问量，以 `view_date` 作为主键。
- `site_view_history`：站点历史日访问量表，每天统计完成后把当日访问量归档到该表。
- `assets`：上传文件元数据表，保存文件名、内容类型、字节数、对象存储 key、可选公开地址、创建人和创建时间。
- `download_events`：文件下载审计表，记录资产、访问端哈希、User-Agent 和发生时间。

## 关联关系

- `articles.author_id` 引用 `users.user_id`。
- `article_logs.article_id` 引用 `articles.article_id`，文章删除时级联删除版本日志。
- `comments.article_id` 引用 `articles.article_id`，文章删除时级联删除评论。
- `article_view_daily.article_id` 引用 `articles.article_id`，文章删除时级联删除统计聚合。
- `article_view_history.article_id` 引用 `articles.article_id`，文章删除时级联删除历史统计。
- `assets.created_by` 引用 `users.user_id`。
- `download_events.asset_id` 引用 `assets.asset_id`，资产删除时级联删除下载审计。

## 隐私规则

- 不保存原始 IP 地址。
- 评论表 `comments` 只保存 `ip_hash`。
- 下载审计表 `download_events` 只保存 `ip_hash`。
- 如后续需要风控、限流或去重，必须在进入数据库前完成 IP 哈希处理。
- `author_email` 属于评论提交者隐私信息，只用于后台审核和必要联系，不应在公开接口中默认返回。

## 文章版本规则

- `articles.version` 默认值为 `1`。
- 每次插入 `articles` 时自动写入一条 `article_logs`。
- 每次更新文章内容字段、状态字段或发布字段时，`articles.version` 自动加 `1`，并写入对应版本的 `article_logs`。
- `article_logs` 不再使用独立日志主键，必须使用 `article_id + version` 唯一定位一条文章历史记录。

## 访问统计归档规则

- `article_view_daily` 和 `site_view_daily` 只承担当天实时计数。
- 后端服务启动后先执行一次过期 daily 数据清理，之后每天本地时间 `00:05` 执行归档。
- 归档只处理 `view_date < current_date` 的 daily 数据，不处理当天实时计数。
- 归档必须使用 `DELETE ... RETURNING` 原子搬迁 daily 数据到 history 表，并在 history 已存在同日记录时累加 `views`。
- 如果访问请求和归档任务并发，归档后才写入旧日期 daily 表的记录会保留到下一次归档继续累加，不允许用“先写 history 再 delete daily”的两步 SQL。
- 历史查询、趋势统计和长期总量应优先读取 history 表，并按需要合并当天 daily 表。

## sqlc 入口

- 配置文件：`backend/sqlc.yaml`。
- schema 来源：`backend/migrations/`。
- 查询来源：`backend/sql/`。
- 生成包路径：`backend/internal/database/dbgen`。
