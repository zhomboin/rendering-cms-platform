# 文章 API

本文档记录 MVP 阶段文章公开读取与后台管理接口。所有接口前缀为 `/api/v1`。

## 数据模型

文章基础响应字段：

```json
{
  "articleId": "uuid",
  "slug": "my-article",
  "title": "文章标题",
  "summary": "文章摘要",
  "bodyMdx": "## MDX 正文",
  "status": "draft",
  "tags": ["Go", "React"],
  "featured": false,
  "coverImageUrl": null,
  "publishedAt": null,
  "authorId": "uuid",
  "createdAt": "2026-04-30T00:00:00Z",
  "updatedAt": "2026-04-30T00:00:00Z"
}
```

`slug` 格式必须满足：

```text
^[a-z0-9]+(?:-[a-z0-9]+)*$
```

## 公开文章列表

```http
GET /api/v1/articles
```

说明：

- 只返回 `published` 状态文章。
- 按发布时间倒序排列。

## 公开文章详情

```http
GET /api/v1/articles/{slug}
```

说明：

- 只返回 `published` 状态文章。
- 未发布或不存在时返回 `404`。

## 后台文章列表

```http
GET /api/v1/admin/articles
Authorization: Bearer <jwt-token>
```

说明：

- 返回全部状态文章。
- 需要 `admin` 或 `editor` 角色。

## 创建草稿

```http
POST /api/v1/admin/articles
Authorization: Bearer <jwt-token>
Content-Type: application/json
```

请求体：

```json
{
  "slug": "my-article",
  "title": "文章标题",
  "summary": "文章摘要",
  "bodyMdx": "## 正文",
  "tags": ["Go"],
  "featured": false,
  "coverImageUrl": ""
}
```

说明：

- 创建后默认状态为 `draft`。
- 创建草稿时必须写入 `article_revisions`。

## 更新草稿

```http
PATCH /api/v1/admin/articles/{id}
Authorization: Bearer <jwt-token>
Content-Type: application/json
```

请求体同创建草稿。

说明：

- 更新后必须写入 `article_revisions`。

## 发布文章

```http
POST /api/v1/admin/articles/{id}/publish
Authorization: Bearer <jwt-token>
```

说明：

- 将文章状态设置为 `published`。
- 首次发布时设置 `published_at`。
- 发布时必须写入 `article_revisions`。

## MDX 导入工具

当前导入工具只扫描并输出待导入 `.mdx` 文件列表，后续阶段再接入数据库写入。

```bash
cd backend
go run ./cmd/import-mdx -source /path/to/content/posts
```
