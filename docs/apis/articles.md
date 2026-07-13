# 文章 API

本文档记录 MVP 阶段文章管理接口，以及供 Rendering 博客读取已发布文章的内容接口。所有接口前缀为 `/api/v1`。

## 数据模型

以下完整模型只用于后台文章管理接口，公开接口使用后续章节定义的最小字段集合：

```json
{
  "articleId": "uuid",
  "slug": "aB3dE9",
  "canonicalSlug": "aB3dE9",
  "articleName": "my-article",
  "title": "文章标题",
  "summary": "文章摘要",
  "bodyMdx": "## MDX 正文",
  "status": "draft",
  "version": 1,
  "tags": ["Go", "React"],
  "featured": false,
  "isFeatured": false,
  "featuredRank": 100,
  "featuredAt": null,
  "coverImageUrl": null,
  "publishedAt": null,
  "authorId": "uuid",
  "createdAt": "2026-04-30T00:00:00Z",
  "updatedAt": "2026-04-30T00:00:00Z",
  "resolvedBy": "slug"
}
```

`slug` 是系统生成的 6 位短链码，不由用户输入。格式必须满足：

```text
^[0-9A-Za-z]{6}$
```

`articleName` 是用户输入或导入工具保留的文章英文名，用于后台识别和旧内容对照，不作为公开访问地址。格式必须满足：

```text
^[a-z0-9]+(?:-[a-z0-9]+)*$
```

首页分区字段说明：

- `isFeatured`：是否进入 Rendering 首页“精选文章”候选集合；与历史字段 `featured` 保持同值返回。
- `featuredRank`：精选排序值，数字越小越靠前；未单独配置时默认为 `100`。
- `featuredAt`：可选精选设置时间，未设置时返回 `null`。
- `canonicalSlug`：实际 6 位短链码。文章列表和详情都返回该字段，Rendering 前台应优先使用它构造 `/blog/<canonicalSlug>`。

## Rendering 博客文章列表

```http
GET /api/v1/articles
```

说明：

- 只返回 `published` 状态文章。
- 按发布时间倒序排列。
- 该接口由 Rendering 博客服务端或前台读取，CMS 前端自身不提供公开文章列表页面。
- 返回 `isFeatured`、`featuredRank`、`featuredAt`，供 Rendering 首页拆分“精选文章”和“最近更新”。
- 首页精选应优先取 `isFeatured=true`，按 `featuredRank` 升序排列；最近更新可按 `publishedAt` 倒序并排除已进入精选的文章。
- 列表不返回正文和后台管理字段，Rendering 不得依赖列表响应中的 `bodyMdx`。

公开列表字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `slug` / `canonicalSlug` | string | 6 位 canonical 短链码 |
| `articleName` | string | 兼容期文章英文名 |
| `title` / `summary` | string | 标题与摘要 |
| `tags` | string[] | 标签 |
| `publishedAt` / `updatedAt` | string/null | 发布时间与更新时间 |
| `isFeatured` | boolean | 是否精选 |
| `featuredRank` | number | 精选排序 |
| `featuredAt` | string/null | 精选设置时间 |
| `coverImageUrl` | string/null | 封面地址 |

列表明确不返回 `bodyMdx`、`articleId`、`authorId`、`version` 和其他后台字段。

## Rendering 博客文章详情

```http
GET /api/v1/articles/{slug}
```

说明：

- 只返回 `published` 状态文章。
- 未发布或不存在时返回 `404`。
- 该接口用于 Rendering 博客的 `/blog/<slug>` 页面渲染。
- `{slug}` 必须使用 CMS 返回的 6 位短链码。
- 兼容期内，如果 `{slug}` 不是 6 位短链码，后端会尝试按 `articleName` 查找并返回对应文章。
- 响应中的 `canonicalSlug` 永远是实际短链；`resolvedBy` 取值为 `slug` 或 `articleName`。
- Rendering 博客如果收到 `resolvedBy: "articleName"`，必须把浏览器地址重定向或替换为 `/blog/<canonicalSlug>`。
- 非法标识符、未发布文章和不存在文章统一返回 `404`；标识符最多 128 个 Unicode 字符。
- 详情返回列表摘要字段，并额外返回完整 `bodyMdx` 和 `resolvedBy`。

## Rendering 博客文章搜索

```http
GET /api/v1/articles/search?q=keyword
```

说明：

- 基于 PostgreSQL full text search 搜索已发布文章。
- 搜索范围包含标题、摘要和 MDX 正文。
- 搜索关键词为空时返回 `400`。
- 去除首尾空白后，搜索关键词长度必须为 1–100 个 Unicode 字符；超出范围返回 `400` 且不查询数据库。
- 返回字段包含 `articleId`、`slug`、`articleName`、`title`、`summary` 和 `publishedAt`，不返回完整 `bodyMdx`。
- 搜索排序优先使用 `ts_rank` 相关度，其次按发布时间倒序排列。
- 搜索最多返回 20 条结果。Rendering 当前继续使用构建期 Pagefind，不直接调用该接口。

## 公开读取缓存与限流

列表和详情的成功响应包含：

```http
Cache-Control: public, max-age=60, stale-while-revalidate=300
Vary: Accept-Encoding
ETag: "<sha256>"
```

客户端再次请求时可发送 `If-None-Match`。ETag 与最终 JSON 内容匹配时返回 `304 Not Modified`，响应体为空。公开详情 `404` 使用 `Cache-Control: public, max-age=15`；服务端 `5xx` 使用 `Cache-Control: no-store`。

应用层和 Nginx 会分别限制公开读取与搜索压力。超过速率返回 `429 Too Many Requests` 并携带 `Retry-After`；超过应用并发上限时返回 `503 Service Unavailable` 和 `Retry-After: 1`。后台、登录和写入接口不使用公开读取桶，但在 Nginx 边界有独立额度。

| 状态码 | 语义 |
|---|---|
| `400` | 搜索词为空或超过 100 个 Unicode 字符 |
| `404` | 标识符非法、文章不存在或未发布 |
| `429` | 触发边界或应用速率限制 |
| `500` | 数据库或服务内部错误，不允许缓存 |

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
  "articleName": "my-article",
  "title": "文章标题",
  "summary": "文章摘要",
  "bodyMdx": "## 正文",
  "tags": ["Go"],
  "featured": false,
  "featuredRank": 100,
  "featuredAt": null,
  "coverImageUrl": ""
}
```

说明：

- 创建后默认状态为 `draft`。
- 后端自动生成 6 位短链码并写入 `slug`，请求体中的 `slug` 字段会被忽略。
- `articleName` 必填，用于保存用户命名的文章英文名。
- `featuredRank` 可选，未传时默认为 `100`。
- `featuredAt` 可选，必须使用 RFC3339 时间字符串；未传或为空时保存为空。
- 创建草稿时 `version` 默认为 `1`。
- 创建草稿成功后由数据库触发器自动写入 `article_logs`。

## 更新草稿

```http
PATCH /api/v1/admin/articles/{id}
Authorization: Bearer <jwt-token>
Content-Type: application/json
```

请求体同创建草稿，不需要传入 `slug`。

说明：

- 更新草稿不会修改已有短链码。
- 更新草稿可以修改 `articleName`。
- 只允许更新 `draft` 状态文章；如果文章已发布或归档，返回 `409`，避免通过“保存草稿”直接覆盖线上内容。
- 更新成功后 `articles.version` 自动加 `1`。
- 更新成功后由数据库触发器自动写入 `article_logs`。

## 发布文章

```http
POST /api/v1/admin/articles/{id}/publish
Authorization: Bearer <jwt-token>
```

说明：

- 将文章状态设置为 `published`。
- 首次发布时设置 `published_at`。
- 发布成功后 `articles.version` 自动加 `1`。
- 发布成功后由数据库触发器自动写入 `article_logs`。

## MDX 导入工具

导入工具用于把 Rendering 静态博客中的 `content/posts/*.mdx` 导入 CMS `articles` 表。

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
go run ./cmd/import-mdx \
  -source /path/to/Rendering/content/posts \
  -database-url "$DATABASE_URL"
```

生产 Docker 环境中，`DATABASE_URL` 里的 `postgres` 主机名只在 Docker 网络内可解析，不要直接在宿主机执行上面的 `go run`。生产导入应使用：

```bash
cd /opt/rendering-cms-platform
bash scripts/ops/import-mdx.sh \
  --source /srv/rendering/content/posts
```

先检查解析结果但不写入数据库：

```bash
cd /opt/rendering-cms-platform
bash scripts/ops/import-mdx.sh \
  --source /srv/rendering/content/posts \
  --dry-run
```

参数说明：

- `-source`：必填，Rendering 静态博客 `content/posts` 目录。
- `-database-url`：PostgreSQL 连接字符串；未传时读取 `DATABASE_URL`。
- `-author-email`：可选，指定导入文章的作者邮箱；未传时使用第一个 `admin` 或 `editor` 用户。
- `-dry-run`：只解析并输出文章状态、短链码和标题，不写数据库。

导入规则：

- `slug` 使用 MDX 文件名稳定派生出的 6 位短链码，不直接使用文件名作为访问地址。
- `articleName` 使用 MDX 文件名，保留历史文章的英文名。
- `title` 读取 front matter 的 `title`。
- `summary` 优先读取 `description`，其次读取 `summary`。
- `published_at` 读取 `publishedAt`。
- `tags` 读取 front matter 的 `tags` 列表。
- `featured` 读取 front matter 的 `featured`。
- `featuredRank` 读取 front matter 的 `featuredRank`，未配置时默认为 `100`。
- `featuredAt` 读取 front matter 的 `featuredAt`，未配置时为空。
- `draft: true` 的文章跳过导入。
- 非草稿文章写入或更新为 `published` 状态。
- 新导入文章的 `version` 默认为 `1`。
- 每次成功导入新文章或更新已有文章，都会由数据库触发器写入 `article_logs`。

## 版本日志规则

- `article_logs` 字段与 `articles` 一致。
- `article_logs` 使用 `article_id + version` 作为联合主键。
- 应用层不直接生成日志主键，也不直接维护文章版本号。
- 文章创建、后台更新、发布和 MDX 导入更新都必须通过 `articles` 表写入，由数据库触发器统一生成版本日志。
