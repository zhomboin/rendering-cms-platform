# 评论 API

本文档记录 MVP 阶段评论提交、公开展示和后台审核接口。所有接口前缀为 `/api/v1`。

## 隐私规则

- 评论提交时后端只保存 `ip_hash`，不得保存原始 IP。
- 公开评论列表不返回邮箱、IP 哈希、User-Agent 等管理字段。
- 后台评论审核接口必须通过后台认证。

## 公开评论模型

```json
{
  "commentId": "uuid",
  "authorName": "访客昵称",
  "body": "评论正文",
  "createdAt": "2026-04-30T00:00:00Z"
}
```

## 后台评论模型

```json
{
  "commentId": "uuid",
  "articleId": "uuid",
  "articleSlug": "my-article",
  "articleTitle": "文章标题",
  "authorName": "访客昵称",
  "authorEmail": null,
  "body": "评论正文",
  "status": "pending",
  "userAgent": "Mozilla/5.0",
  "createdAt": "2026-04-30T00:00:00Z",
  "reviewedAt": null
}
```

`status` 取值：

- `pending`：待审核。
- `approved`：已通过，允许公开展示。
- `rejected`：已拒绝，不公开展示。

## 公开评论列表

```http
GET /api/v1/articles/{slug}/comments
```

说明：

- 只返回所属文章的 `approved` 评论。
- 文章必须是 `published` 状态。

## 提交评论

```http
POST /api/v1/articles/{slug}/comments
Content-Type: application/json
```

请求体：

```json
{
  "authorName": "访客昵称",
  "authorEmail": "reader@example.com",
  "body": "评论正文"
}
```

说明：

- `authorName` 和 `body` 必填。
- 新评论默认进入 `pending` 状态。
- 后端从请求上下文派生 IP 哈希，只保存哈希值。
- 未发布或不存在的文章返回 `404`。

## 后台评论列表

```http
GET /api/v1/admin/comments
Authorization: Bearer <jwt-token>
```

说明：

- 返回待审核、已通过、已拒绝的全部评论。
- 需要后台认证。

## 审核评论

```http
PATCH /api/v1/admin/comments/{id}
Authorization: Bearer <jwt-token>
Content-Type: application/json
```

请求体：

```json
{
  "status": "approved"
}
```

说明：

- `status` 只允许 `approved` 或 `rejected`。
- 审核成功后写入 `reviewed_at`。
