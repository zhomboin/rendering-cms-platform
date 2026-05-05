# MVP 阶段 04：文章 API、发布流和 MDX 导入

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

实现文章的公开读取、后台草稿编辑、发布和版本日志基础能力，并提供从原静态博客 MDX 文件导入文章的工具入口。

## 文件范围

- Create: `backend/internal/articles/service.go`
- Create: `backend/internal/articles/service_test.go`
- Create: `backend/internal/articles/handler.go`
- Create: `backend/cmd/import-mdx/main.go`
- Modify: `backend/sql/articles.sql`
- Create: `docs/apis/articles.md`

## 子任务

- [x] 在 `service_test.go` 编写 `TestValidateSlug`。
- [x] 在 `service.go` 实现 `ValidSlug(slug string) bool`，规则为 `^[a-z0-9]+(?:-[a-z0-9]+)*$`。
- [x] 在 `backend/sql/articles.sql` 增加创建草稿文章 SQL。
- [x] 在 `backend/sql/articles.sql` 增加更新草稿文章 SQL。
- [x] 在 `backend/sql/articles.sql` 增加发布文章 SQL，设置 `status='published'` 和 `published_at`。
- [x] 通过数据库触发器支持写入 `article_logs`。
- [x] 在 `backend/sql/articles.sql` 增加公开文章列表 SQL，只返回 `published`。
- [x] 在 `backend/sql/articles.sql` 增加公开文章详情 SQL，只返回 `published`。
- [x] 在 `handler.go` 实现 `GET /api/v1/articles`。
- [x] 在 `handler.go` 实现 `GET /api/v1/articles/{slug}`。
- [x] 在 `handler.go` 实现 `GET /api/v1/admin/articles`。
- [x] 在 `handler.go` 实现 `POST /api/v1/admin/articles`。
- [x] 在 `handler.go` 实现 `PATCH /api/v1/admin/articles/{id}`。
- [x] 在 `handler.go` 实现 `POST /api/v1/admin/articles/{id}/publish`。
- [x] 创建 `backend/cmd/import-mdx/main.go`，支持 `-source` 参数扫描 `*.mdx`。
- [x] 导入工具支持解析 Rendering 静态博客 front matter。
- [x] 导入工具支持把非草稿 MDX 写入或更新到 `articles` 表。
- [x] 导入工具导入成功后由数据库触发器写入 `article_logs`。
- [x] 创建 `docs/apis/articles.md`，记录公开和后台文章 API。
- [x] 运行 `cd backend && go test ./internal/articles`。
- [x] 运行 `cd backend && go test ./cmd/import-mdx`。

## 完成记录

- 完成时间：2026-04-30。
- 公开文章接口只读取 `published` 状态文章。
- 后台文章接口挂载在 `/api/v1/admin/articles`，由后台认证中间件保护。
- 创建草稿、更新草稿和发布文章都会由数据库触发器写入 `article_logs`。
- MDX 导入工具支持 `-source`、`-database-url`、`-author-email` 和 `-dry-run` 参数。
- MDX 导入工具会跳过 `draft: true` 的文章，并把非草稿文章导入为 `published` 状态。
- 验证命令已通过：`cd backend && go test ./internal/articles ./cmd/import-mdx ./...`。
- 已使用 Rendering 静态博客真实 `content/posts/*.mdx` dry-run 验证 21 篇文章可解析。

## 验收标准

- 公开 API 只返回已发布文章。
- 后台 API 必须要求认证。
- 保存草稿和发布文章必须写入 `article_logs`，且 `articles.version` 必须随更新递增。
- slug 校验有测试覆盖。

## 建议提交

```bash
git add backend/internal/articles backend/cmd/import-mdx backend/sql/articles.sql docs/apis/articles.md
git commit -m "feat: add article publishing foundation"
```
