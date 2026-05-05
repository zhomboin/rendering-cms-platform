# CMS 平台技术架构建议

## 结论

建议将统计、评论、上传下载、后台文章编辑发布作为一个独立新项目开发，后端使用 Go，前端使用 React + TypeScript。

当前 `Rendering` 仓库已经是一个完成态静态博客：本地 MDX、构建期搜索、公开阅读体验、轻量部署脚本是它的核心优势。新需求需要登录、数据库、对象存储、后台管理、写入 API、数据备份和权限控制，架构性质已经不同。新项目可以复用当前博客的视觉语言、公开页面结构和 MDX 内容，但不建议继续在当前仓库里叠加后台系统。

## 是否可以作为单独项目开发

可以，而且这是推荐路径。

推荐拆分方式：

- 保留当前仓库作为静态博客基线和内容迁移来源。
- 新建 `rendering-cms-platform` 项目。
- 新项目采用前后端分离：
  - `backend/`：Go API 服务。
  - `frontend/`：React + TypeScript 管理端与公开站点。
  - `docs/`：接口、数据模型、部署、运维文档。
- 初期通过 MDX 导入工具把当前 `content/posts` 迁移到新项目数据库。
- 新项目稳定后，再决定是否由新项目接管公开站点域名。

## 推荐技术栈

### 后端

- Go 1.22+
- `Gin` 或 `Chi` 作为 HTTP Router
- `PostgreSQL` 作为主数据库
- `sqlc` + `pgx` 作为数据访问层
- `golang-migrate/migrate` 管理 SQL migration
- `JWT` 或安全 Cookie Session 做后台登录态
- `bcrypt` 做密码哈希
- `aws-sdk-go-v2` 连接 S3 兼容对象存储，例如 Cloudflare R2、AWS S3、MinIO
- `zap` 或 `slog` 做结构化日志

推荐优先选择 `Chi + pgx + sqlc + migrate`。这套组合比 ORM 更透明，适合这个项目的文章、评论、统计、文件审计等明确业务表。

### 前端

- React
- TypeScript
- Vite
- React Router
- TanStack Query
- Ant Design 或 shadcn/ui 二选一
- MDX 编辑器可选：
  - 简单版：textarea + 实时预览
  - 增强版：CodeMirror / Monaco Editor

如果后台偏运营工具，优先 Ant Design；如果希望延续当前博客的定制视觉语言，优先 shadcn/ui + Tailwind。

### 数据库

- PostgreSQL
- 主要表：
  - `users`
  - `articles`
  - `article_logs`
  - `comments`
  - `article_view_daily`
  - `article_view_history`
  - `site_view_daily`
  - `site_view_history`
  - `assets`
  - `download_events`

### 对象存储

- Cloudflare R2、AWS S3 或 MinIO
- 数据库只保存文件元数据和 object key
- 上传和下载使用预签名 URL
- 下载时写入审计记录
- 不把上传文件写入 Git 或前端 `public/`

## 项目结构建议

```text
rendering-cms-platform/
  backend/
    cmd/server/
    internal/auth/
    internal/articles/
    internal/comments/
    internal/analytics/
    internal/assets/
    internal/storage/
    internal/database/
    internal/http/
    migrations/
    sql/
  frontend/
    src/api/
    src/routes/
    src/pages/
    src/components/
    src/features/articles/
    src/features/comments/
    src/features/analytics/
    src/features/assets/
  docs/
    apis/
    sql/
    operations/
```

## API 边界

后端统一提供 `/api/v1/*`：

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout`
- `GET /api/v1/articles`
- `POST /api/v1/admin/articles`
- `PATCH /api/v1/admin/articles/{id}`
- `POST /api/v1/admin/articles/{id}/publish`
- `GET /api/v1/articles/{slug}`
- `POST /api/v1/articles/{slug}/views`
- `GET /api/v1/admin/analytics/summary`
- `GET /api/v1/articles/{slug}/comments`
- `POST /api/v1/articles/{slug}/comments`
- `PATCH /api/v1/admin/comments/{id}`
- `POST /api/v1/admin/assets/upload-url`
- `GET /api/v1/admin/assets/{id}/download-url`

公开接口只暴露已发布文章、已审核评论和访问统计写入。后台接口全部要求管理员登录态。

## 内容模型

- 数据库存储文章标题、摘要、标签、状态、发布时间和 MDX 正文。
- 保留 MDX 作为正文格式，继续复用当前内容写作习惯。
- 文章状态至少包含：
  - `draft`
  - `published`
  - `archived`
- `articles.version` 默认值为 `1`，每次更新后版本号自动加 `1`。
- 每次插入或更新 `articles` 时由数据库触发器写入 `article_logs`。
- `article_logs` 字段与 `articles` 一致，使用 `article_id + version` 作为联合主键，不再单独生成日志主键。
- 当前仓库的 `content/posts/*.mdx` 只作为导入来源和备份格式，不作为新项目运行时内容源。

## 统计模型

- 第一版使用日聚合，不记录每一次明细访问。
- 文章访问量写入 `article_view_daily`，该表只保存当天实时计数。
- 站点访问量写入 `site_view_daily`，该表只保存当天实时计数。
- 每天统计完成后，将文章和站点当日访问量分别归档到 `article_view_history` 与 `site_view_history`。
- 后台首页展示今日访问量、近 7 天访问量、热门文章。
- 后端记录 IP 哈希和 User-Agent 只用于基础去重或风控时再扩展；第一版可以只做简单计数。

## 评论模型

- 匿名评论可以提交，但默认进入 `pending`。
- 管理员审核后才公开。
- 不存储原始 IP，只存储哈希。
- 第一版不做嵌套评论，降低复杂度。
- 可以增加简单限流：同一 IP 哈希 1 分钟内最多提交 3 次。

## 文件上传下载

- 前端向 Go 后端请求上传 URL。
- Go 后端校验文件名、类型、大小并创建 `assets` 记录。
- Go 后端返回 S3/R2 预签名上传 URL。
- 前端直传对象存储。
- 下载时前端请求 Go 后端生成预签名下载 URL。
- Go 后端写入 `download_events`。

第一版允许类型：

- `image/png`
- `image/jpeg`
- `image/webp`
- `application/pdf`
- `text/plain`
- `application/zip`

最大文件大小建议先设为 `20MB`。

## 搜索

第一阶段建议直接使用 PostgreSQL 搜索：

- 简单版：标题、摘要、正文 `ILIKE`
- 增强版：PostgreSQL full text search

等内容量和查询需求上来后，再考虑 Meilisearch 或 Typesense。

## 部署

推荐部署单元：

- Go 后端：独立 systemd service 或容器
- React 前端：静态构建后由 Nginx/Caddy 托管
- PostgreSQL：独立实例或托管数据库
- 对象存储：R2/S3/MinIO

部署流程：

1. 备份 PostgreSQL。
2. 执行 SQL migration。
3. 构建并发布 Go 后端。
4. 构建并发布 React 前端。
5. 重启后端服务。
6. 执行健康检查。

## 不推荐方案

不建议：

- 在当前静态仓库内直接追加 Go 后端。
- 让 Go 后端直接写入当前仓库的 `content/posts`。
- 上传文件到 Git 或前端 `public/`。
- 用 JSON 文件保存评论或统计。
- 无认证保护地暴露后台 API。
- 继续用构建期 Pagefind 承担 CMS 动态搜索。

这些方案短期看起来简单，但会在部署、并发、备份、安全和搜索一致性上很快失控。

## 推荐实施顺序

1. 新建 `rendering-cms-platform` 仓库或目录。
2. 搭建 Go 后端骨架、配置、健康检查和数据库连接。
3. 建立 SQL migration、`sqlc` 查询和核心数据表。
4. 搭建 React + TypeScript 前端骨架和后台壳层。
5. 实现登录、鉴权和管理员初始化。
6. 编写 MDX 导入工具，迁移现有文章。
7. 实现公开文章读取和后台文章编辑发布。
8. 实现文章统计和后台看板。
9. 实现评论提交与审核。
10. 实现文件上传、下载和审计。
11. 在测试环境验证完整发布链路。
12. 决定是否切换生产域名到新项目。
