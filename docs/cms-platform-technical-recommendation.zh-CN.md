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
  - `frontend/`：React + TypeScript 管理端。
  - `docs/`：接口、数据模型、部署、运维文档。
- 初期通过 MDX 导入工具把当前 `content/posts` 迁移到新项目数据库。
- 新项目稳定后，由原 `Rendering` 博客项目读取 CMS 已发布内容并继续承载公开站点。

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
    src/pages/articles/
    src/pages/comments/
    src/pages/dashboard/
    src/pages/assets/
    src/pages/auth/
    src/components/
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

内容读取接口只提供给 Rendering 博客消费，用于读取已发布文章、已审核评论和写入访问统计。CMS 前端自身不承载公开内容页面，后台接口全部要求管理员登录态。

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

## 相册模型

- 相册存 `photo_albums`（slug、中英文标题与导语、状态、标签、封面照片引用、发布时间），照片存 `album_photos`（alt、说明、服务端提取的像素宽高、拍摄时间、相册内顺序），文件本体元数据复用 `assets` 表。
- 相册状态只有 `draft` 与 `published`；仅 `published` 相册对外可见，发布要求至少一张照片，`published_at` 首次发布写入并保持稳定。
- 相册 slug 由管理员指定（小写字母、数字、中划线），发布后不可修改，保证博客路由 `/photos/{slug}` 稳定。
- 照片上传走后端代理链路（区别于通用文件的预签名直传）：服务端完成魔数校验、像素尺寸提取、EXIF GPS 剥离后写入对象存储，对象携带长缓存头且内容不可变；接口契约见 `docs/apis/photos.md`。

## 统计模型

- 第一版使用日聚合，不记录每一次明细访问。
- 文章访问量写入 `article_view_daily`，该表只保存当天实时计数。
- 相册访问量写入 `album_view_daily`，与文章访问共用每日 IP 哈希去重（`analytics_events` 唯一索引），同时计入站点访问量。
- 站点访问量写入 `site_view_daily`，该表只保存当天实时计数。
- 后端服务启动后先清理过期 daily 数据，之后每天本地时间 `00:05` 将历史日期访问量分别归档到 `article_view_history` 与 `site_view_history`。
- 归档使用 `DELETE ... RETURNING` 原子搬迁 daily 数据，并在 history 已存在同日记录时累加访问量，避免并发访问和归档任务同时发生时丢失计数。
- 后台首页为浅色数据驾驶舱，默认展示近 30 天，可切换 7／30／90 天；涵盖访问趋势、带封面的文章排行、相册表现、内容状态、待办与资源摘要。后台只读汇总契约见 [首页数据驾驶舱 API](apis/dashboard.md)，访问计数采用现有每日去重口径，不作为标准 PV 或 UV。
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

生产部署已按以下形态落地（2026-10）：

- 生产环境为单机 Docker Compose：`frontend`（Nginx 托管 Vite 产物并反代 API）、`backend`（Go 容器）、`postgres`（PostgreSQL 16）、`migrate`（一次性 migration profile）。
- 生产配置集中在服务器 `/etc/rendering-cms/production.env`，不在代码仓库内。
- 发布由 systemd timer 自动化：`cms-rendering-deploy.timer` 每小时整点触发 `deploy/auto-deploy.sh`，自动拉取代码、比较提交、按固定顺序发布（pull → build → 备份 → migration → 重启 → 健康检查），失败自动回滚代码。
- 对象存储使用 Cloudflare R2，宿主机 Nginx 终止 HTTPS。

部署入口文档：`docs/operations/deployment.md`（首次部署与手动 SOP）、`docs/operations/auto-deploy.md`（自动部署机制）、`docs/operations/maintenance.md`（日常巡检速查）。

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

以下顺序已全部落地投产（2026-10），保留作为项目演进记录；当前执行入口见 `docs/README.md`。

1. 新建 `rendering-cms-platform` 仓库或目录。
2. 搭建 Go 后端骨架、配置、健康检查和数据库连接。
3. 建立 SQL migration、`sqlc` 查询和核心数据表。
4. 搭建 React + TypeScript 前端骨架和后台壳层。
5. 实现登录、鉴权和管理员初始化。
6. 编写 MDX 导入工具，迁移现有文章。
7. 实现 Rendering 博客文章读取接口和后台文章编辑发布。
8. 实现文章统计和后台看板。
9. 实现评论提交与审核。
10. 实现文件上传、下载和审计。
11. 在测试环境验证完整发布链路。
12. 决定是否切换生产域名到新项目。
