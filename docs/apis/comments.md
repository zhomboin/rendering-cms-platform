# 评论 API

本文档记录 MVP 阶段 Rendering 博客评论提交、已审核评论读取和 CMS 后台审核接口。所有接口前缀为 `/api/v1`。

## 隐私规则

- 评论提交时后端只保存 `ip_hash`，不得保存原始 IP。
- Rendering 博客评论列表不返回邮箱、IP 哈希、User-Agent 等管理字段。
- 后台评论审核接口必须通过后台认证。

## Rendering 博客评论模型

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
  "articleSlug": "aB3dE9",
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
- `approved`：已通过，允许被 Rendering 博客展示。
- `rejected`：已拒绝，不在 Rendering 博客展示。

## Rendering 博客评论列表

```http
GET /api/v1/articles/{slug}/comments
```

说明：

- 只返回所属文章的 `approved` 评论。
- 文章必须是 `published` 状态。
- `{slug}` 是 CMS 返回的 6 位短链码，格式为 `^[0-9A-Za-z]{6}$`。
- 兼容期内，如果 `{slug}` 不是 6 位短链码，后端会按 `articleName` 解析到对应文章。

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
- `{slug}` 是 CMS 返回的 6 位短链码。
- 兼容期内支持传入 `articleName`，但 Rendering 博客正式实现应使用短链 `slug`。
- 后端从请求上下文派生 IP 哈希，只保存哈希值。
- 未发布或不存在的文章返回 `404`。
- 同一 IP 哈希 1 分钟内最多提交 3 条评论，超限时返回 `429 Too Many Requests`。

## 限流规则

同一 IP 哈希 1 分钟内最多提交 3 条评论。超限时返回 `429 Too Many Requests`。

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
