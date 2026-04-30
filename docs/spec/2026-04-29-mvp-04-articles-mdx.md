# MVP 阶段 04：文章 API、发布流和 MDX 导入

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

实现文章的公开读取、后台草稿编辑、发布和修订记录基础能力，并提供从原静态博客 MDX 文件导入文章的工具入口。

## 文件范围

- Create: `backend/internal/articles/service.go`
- Create: `backend/internal/articles/service_test.go`
- Create: `backend/internal/articles/handler.go`
- Create: `backend/cmd/import-mdx/main.go`
- Modify: `backend/sql/articles.sql`
- Create: `docs/apis/articles.md`

## 子任务

- [ ] 在 `service_test.go` 编写 `TestValidateSlug`。
- [ ] 在 `service.go` 实现 `ValidSlug(slug string) bool`，规则为 `^[a-z0-9]+(?:-[a-z0-9]+)*$`。
- [ ] 在 `backend/sql/articles.sql` 增加创建草稿文章 SQL。
- [ ] 在 `backend/sql/articles.sql` 增加更新草稿文章 SQL。
- [ ] 在 `backend/sql/articles.sql` 增加发布文章 SQL，设置 `status='published'` 和 `published_at`。
- [ ] 在 `backend/sql/articles.sql` 增加写入 `article_revisions` SQL。
- [ ] 在 `backend/sql/articles.sql` 增加公开文章列表 SQL，只返回 `published`。
- [ ] 在 `backend/sql/articles.sql` 增加公开文章详情 SQL，只返回 `published`。
- [ ] 在 `handler.go` 实现 `GET /api/v1/articles`。
- [ ] 在 `handler.go` 实现 `GET /api/v1/articles/{slug}`。
- [ ] 在 `handler.go` 实现 `GET /api/v1/admin/articles`。
- [ ] 在 `handler.go` 实现 `POST /api/v1/admin/articles`。
- [ ] 在 `handler.go` 实现 `PATCH /api/v1/admin/articles/{id}`。
- [ ] 在 `handler.go` 实现 `POST /api/v1/admin/articles/{id}/publish`。
- [ ] 创建 `backend/cmd/import-mdx/main.go`，支持 `-source` 参数扫描 `*.mdx`。
- [ ] 导入工具第一版输出待导入文件列表，后续接入数据库写入。
- [ ] 创建 `docs/apis/articles.md`，记录公开和后台文章 API。
- [ ] 运行 `cd backend && go test ./internal/articles`。
- [ ] 运行 `cd backend && go test ./cmd/import-mdx`。

## 验收标准

- 公开 API 只返回已发布文章。
- 后台 API 必须要求认证。
- 保存草稿和发布文章必须写入 `article_revisions`。
- slug 校验有测试覆盖。

## 建议提交

```bash
git add backend/internal/articles backend/cmd/import-mdx backend/sql/articles.sql docs/apis/articles.md
git commit -m "feat: add article publishing foundation"
```
