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

导入工具用于把 Rendering 静态博客中的 `content/posts/*.mdx` 导入 CMS `articles` 表。

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
go run ./cmd/import-mdx \
  -source /path/to/Rendering/content/posts \
  -database-url "$DATABASE_URL"
```

参数说明：

- `-source`：必填，Rendering 静态博客 `content/posts` 目录。
- `-database-url`：PostgreSQL 连接字符串；未传时读取 `DATABASE_URL`。
- `-author-email`：可选，指定导入文章的作者邮箱；未传时使用第一个 `admin` 或 `editor` 用户。
- `-dry-run`：只解析并输出文章状态、slug 和标题，不写数据库。

导入规则：

- `slug` 使用 MDX 文件名。
- `title` 读取 front matter 的 `title`。
- `summary` 优先读取 `description`，其次读取 `summary`。
- `published_at` 读取 `publishedAt`。
- `tags` 读取 front matter 的 `tags` 列表。
- `draft: true` 的文章跳过导入。
- 非草稿文章写入或更新为 `published` 状态。
- 每次成功导入或更新文章都会写入 `article_revisions`。
