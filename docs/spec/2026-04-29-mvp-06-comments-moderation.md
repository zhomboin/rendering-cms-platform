# MVP 阶段 06：评论提交和审核

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

实现匿名评论提交、默认待审核、公开只展示已通过评论，以及后台评论审核能力。

## 文件范围

- Create: `backend/internal/comments/service.go`
- Create: `backend/internal/comments/service_test.go`
- Create: `backend/internal/comments/handler.go`
- Modify: `backend/sql/comments.sql`
- Create: `frontend/src/features/comments/AdminCommentsPage.tsx`
- Create: `docs/apis/comments.md`

## 子任务

- [x] 在 `service_test.go` 编写 `TestNewCommentDefaultsToPending`。
- [x] 在 `service.go` 定义 `Comment` 结构，字段包含 `AuthorName`、`Body`、`Status`。
- [x] 在 `service.go` 实现 `NewComment(authorName, body string) Comment`，默认 `Status` 为 `pending`。
- [x] 在 `backend/sql/comments.sql` 增加创建评论 SQL，默认状态 `pending`。
- [x] 在 `backend/sql/comments.sql` 增加公开评论列表 SQL，只返回 `approved`。
- [x] 在 `backend/sql/comments.sql` 增加后台评论列表 SQL。
- [x] 在 `backend/sql/comments.sql` 增加评论审核 SQL，只允许 `approved` 或 `rejected`。
- [x] 在 `handler.go` 实现 `GET /api/v1/articles/{slug}/comments`。
- [x] 在 `handler.go` 实现 `POST /api/v1/articles/{slug}/comments`。
- [x] 提交评论时从请求上下文派生 `ip_hash`，不得保存原始 IP。
- [x] 在 `handler.go` 实现 `GET /api/v1/admin/comments`。
- [x] 在 `handler.go` 实现 `PATCH /api/v1/admin/comments/{id}`。
- [x] 创建 `AdminCommentsPage.tsx`，包含待审核、已通过、已拒绝状态区域。
- [x] 创建 `docs/apis/comments.md`。
- [x] 运行 `cd backend && go test ./internal/comments`。
- [x] 运行 `cd frontend && npm run build`。

## 验收标准

- 新评论默认 `pending`。
- 公开评论列表只展示 `approved`。
- 评论审核 API 必须要求后台认证。
- 文档明确“不保存原始 IP”。

## 建议提交

```bash
git add backend/internal/comments backend/sql/comments.sql frontend/src/features/comments docs/apis/comments.md
git commit -m "feat: add comment moderation foundation"
```

## 完成记录

- 实现公开评论提交和已审核评论列表，评论默认 `pending`。
- 后台评论列表和审核接口接入后台认证路由。
- 后端仅保存请求 IP 哈希，API 响应不暴露原始 IP 或 IP 哈希。
