# Rendering 博客访问统计对接方案

本文档定义 Rendering 静态博客与 Rendering CMS Platform 的访问统计对接方案。目标是先实现各篇文章访问量统计和 Rendering 博客整体访问量统计，并保持两个项目的边界清晰。

## 目标

- Rendering 静态博客继续负责公开站点渲染和内容访问。
- Rendering CMS Platform 负责访问事件接收、PostgreSQL 聚合写入和后台统计展示。
- 每篇文章按 `slug` 统计每日访问量。
- 整个 Rendering 博客按日期统计站点总访问量。
- 统计上报失败不得影响静态博客页面访问。

## 非目标

- 不在 Rendering 静态博客仓库中保存运行时统计 JSON。
- 不让 Rendering 静态博客直接连接 PostgreSQL。
- 不让 CMS 后端写回 Rendering 静态博客的 `content/posts/`。
- 第一阶段不实现访客唯一去重、实时在线人数、来源分析和设备分析。
- 第一阶段不接入第三方统计平台。

## 当前能力

CMS 当前已经具备文章访问写入和后台汇总展示能力：

```http
POST /api/v1/articles/{slug}/views
```

该接口当前行为：

- 按 `slug` 查询 CMS 数据库中的已发布文章。
- 向 `article_view_daily` 写入当日文章访问量。
- 同时向 `site_view_daily` 写入当日站点访问量。
- 返回 `204 No Content`。

后台统计汇总接口：

```http
GET /api/v1/admin/analytics/summary
Authorization: Bearer <jwt-token>
```

该接口当前返回：

- 今日站点访问量。
- 近 7 天站点访问量。
- 热门文章列表。

CMS 已补齐以下 Rendering 静态博客对接接口：

```http
POST /api/v1/analytics/site-views
GET /api/v1/admin/analytics/articles?days=7
```

其中站点访问接口用于非文章页面 PV 写入，后台文章统计列表接口用于展示每篇文章的今日访问量、最近 N 天访问量和总访问量。

## 核心前提

Rendering 静态博客文章 URL 使用：

```text
/blog/<slug>
```

CMS 统计接口按 `slug` 关联文章。因此，在 Rendering 静态博客开始上报文章访问前，必须先确保 CMS 数据库中存在相同 `slug` 的已发布文章记录。

第一阶段通过 MDX 导入工具完成这件事：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
go run ./cmd/import-mdx \
  -source /path/to/Rendering/content/posts \
  -database-url "$DATABASE_URL"
```

生产 Docker 环境不要在宿主机直接执行 `go run` 连接 `postgres`，因为 `postgres` 只在 Docker 网络内可解析。服务器上应执行：

```bash
cd /opt/rendering-cms-platform
bash scripts/ops/import-mdx.sh \
  --source /srv/rendering/content/posts
```

导入工具需要把 Rendering 静态博客中的 `content/posts/*.mdx` 导入 CMS `articles` 表，并保证：

- `slug` 与 MDX 文件名一致。
- `title`、`summary`、`tags`、`body_mdx` 来自 MDX front matter 和正文。
- `status` 设置为 `published`。
- `published_at` 使用原文章发布时间。
- `author_id` 指向管理员或系统导入用户。
- 每次导入或更新文章时由数据库触发器写入 `article_logs`，并同步维护 `articles.version`。

如需先检查解析结果而不写入数据库，可以使用：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
go run ./cmd/import-mdx -source /path/to/Rendering/content/posts -dry-run
```

生产 Docker 环境 dry-run：

```bash
cd /opt/rendering-cms-platform
bash scripts/ops/import-mdx.sh \
  --source /srv/rendering/content/posts \
  --dry-run
```

## 数据流

文章详情访问流程：

```text
访客打开 Rendering /blog/<slug>
  -> Rendering 客户端 Tracker 静默上报
  -> POST /api/v1/articles/<slug>/views
  -> CMS 校验文章存在且已发布
  -> article_view_daily 当日访问量 +1
  -> site_view_daily 当日访问量 +1
  -> 后台统计页面读取聚合结果
```

非文章页面访问流程：

```text
访客打开 Rendering 首页、文章列表、关于页等非文章页面
  -> Rendering 客户端 Tracker 静默上报
  -> POST /api/v1/analytics/site-views
  -> site_view_daily 当日访问量 +1
  -> 后台统计页面读取聚合结果
```

文章详情页只调用文章访问接口，不额外调用站点访问接口，避免站点总访问量重复计数。

## CMS 接口设计

### 现有文章访问接口

```http
POST /api/v1/articles/{slug}/views
```

用途：

- 记录单篇文章访问量。
- 同步增加站点总访问量。

请求体：

```json
{}
```

成功响应：

```http
204 No Content
```

失败响应：

```json
{
  "error": "文章不存在"
}
```

实现要求：

- 仅公开已发布文章允许写入访问统计。
- 不要求登录。
- 不保存原始 IP 地址。
- 第一阶段不做强去重，每次有效上报计为一次访问。

### 站点访问接口

```http
POST /api/v1/analytics/site-views
```

用途：

- 记录 Rendering 博客整体访问量。
- 用于首页、文章列表页、标签页、关于页等非文章页面。

请求体：

```json
{
  "path": "/blog",
  "referrer": "https://example.com"
}
```

字段说明：

- `path`：访问路径，用于后续扩展页面级统计。第一阶段可选写入日志或暂不持久化。
- `referrer`：来源页面。第一阶段可忽略或仅用于后续扩展预留。

成功响应：

```http
204 No Content
```

实现要求：

- 不要求登录。
- 第一阶段只更新当天 `site_view_daily`。
- 不保存原始 IP 地址。
- 参数不合法时仍应尽量不影响站点访问；服务端可以返回 `204` 并忽略无效扩展字段。

### 文章统计列表接口

```http
GET /api/v1/admin/analytics/articles?days=7
Authorization: Bearer <jwt-token>
```

用途：

- 在后台展示各个文档的访问量。
- 支持按最近 N 天查看文章访问表现。

响应示例：

```json
{
  "days": 7,
  "articles": [
    {
      "slug": "mysql-mvcc-read-view-explained",
      "title": "MySQL MVCC Read View Explained",
      "todayViews": 12,
      "periodViews": 86,
      "totalViews": 324,
      "publishedAt": "2026-03-18T00:00:00Z"
    }
  ]
}
```

实现要求：

- 需要 `admin` 或 `editor` 角色。
- `days` 默认 `7`，允许范围建议为 `1` 到 `90`。
- `periodViews` 统计最近 `days` 天。
- `totalViews` 统计该文章全部历史访问量。
- 排序默认按 `periodViews desc, todayViews desc, published_at desc`。

## Rendering 静态博客改造

### 环境变量

Rendering 静态博客新增公开环境变量：

```env
NEXT_PUBLIC_CMS_API_BASE=https://cms.rendering.me/api/v1
```

本地开发示例：

```env
NEXT_PUBLIC_CMS_API_BASE=http://127.0.0.1:8080/api/v1
```

### 客户端 Tracker

在 Rendering 静态博客新增客户端上报组件，建议命名为：

```text
components/cms-analytics-tracker.tsx
```

文章详情页调用：

```tsx
<CmsAnalyticsTracker kind="article" slug={post.slug} />
```

非文章页面调用：

```tsx
<CmsAnalyticsTracker kind="site" path={pathname} />
```

上报策略：

- 优先使用 `navigator.sendBeacon`。
- 不支持 `sendBeacon` 时使用 `fetch`，并设置 `keepalive: true`。
- 上报失败只在开发环境输出调试信息，生产环境静默失败。
- 只在浏览器端执行，不参与服务端渲染。

伪代码：

```ts
const endpoint =
  kind === 'article'
    ? `${baseUrl}/articles/${encodeURIComponent(slug)}/views`
    : `${baseUrl}/analytics/site-views`;

navigator.sendBeacon(endpoint, new Blob([JSON.stringify(payload)], {
  type: 'application/json',
}));
```

### 接入位置

文章详情页：

- 推荐在 `components/pages/blog-detail-page.tsx` 中拿到 `post.slug` 后渲染文章 Tracker。
- 文章详情页不要再渲染站点 Tracker。

站点级页面：

- 首页、文章列表页、关于页、英文壳层页面可以渲染站点 Tracker。
- 如果放在全局 layout，需要排除 `/blog/<slug>`，避免文章详情页重复增加站点访问量。

## 部署方案

### 当前生产路径

当前生产部署默认通过 CMS 独立域名暴露公共 API：

```text
https://cms.rendering.me/api/v1
```

Rendering 静态博客配置：

```env
NEXT_PUBLIC_CMS_API_BASE=https://cms.rendering.me/api/v1
```

CMS 后端必须允许 Rendering 站点来源。生产环境使用来源白名单：

```text
FRONTEND_ORIGINS=https://cms.rendering.me,https://rendering.me,https://www.rendering.me
```

本地开发时允许：

```text
FRONTEND_ORIGINS=http://127.0.0.1:3000,http://127.0.0.1:5173
```

`FRONTEND_ORIGIN` 仅保留给旧配置兼容；如果同时设置 `FRONTEND_ORIGINS` 和 `FRONTEND_ORIGIN`，以后端实际读取的 `FRONTEND_ORIGINS` 为准。

### 可选同域路径

如果后续希望减少 CORS 复杂度，可以在 Rendering 博客宿主机上额外把同域路径反向代理到 CMS：

```text
https://rendering.me/api/v1/* -> Rendering CMS Platform backend
```

启用该方案后，Rendering 静态博客可以改用：

```env
NEXT_PUBLIC_CMS_API_BASE=https://rendering.me/api/v1
```

## 数据库与迁移

第一阶段使用以下统计表：

- `article_view_daily`
- `article_view_history`
- `site_view_daily`
- `site_view_history`

`article_view_daily` 和 `site_view_daily` 只保存当天实时访问量。后端服务启动后会先执行一次过期 daily 数据清理，之后每天本地时间 `00:05` 执行归档，把 `view_date < current_date` 的 daily 数据搬迁到对应 history 表。

归档必须使用 `DELETE ... RETURNING` 原子搬迁，不能使用“先写 history 再 delete daily”的两步 SQL。原因是用户访问可能和归档任务并发：如果访问写入夹在两步 SQL 之间，后续 delete 可能误删尚未归档的计数。

归档过程中如果仍有延迟访问写入旧日期 daily 表，该记录会保留到下一次归档继续累加到 history 表，不会覆盖已有 history 记录。

新增文章统计列表接口查询历史区间时，应优先读取 history 表，并按需要合并当天 daily 表。

如果后续要做页面级统计或来源分析，再新增聚合表，例如：

```text
page_view_daily(path, view_date, views)
referrer_view_daily(referrer_host, view_date, views)
```

这些扩展不进入第一阶段。

## 分阶段实施

### 阶段 1：打通文章统计

- 完成 MDX 导入工具，使 Rendering 文章 slug 进入 CMS `articles` 表。
- 在 Rendering 文章详情页加入文章访问 Tracker。
- 验证访问 `/blog/<slug>` 后，CMS `article_view_daily` 和 `site_view_daily` 同步增加。
- 验证每日归档任务可把历史日期 daily 数据写入 `article_view_history` 和 `site_view_history`，并清理对应 daily 记录。
- 后台看板能展示热门文章和站点访问量。

### 阶段 2：打通站点总访问统计

- CMS 已新增 `POST /api/v1/analytics/site-views`。
- Rendering 首页、文章列表页、关于页等非文章页面加入站点访问 Tracker。
- 验证非文章页面访问只增加 `site_view_daily`，不增加 `article_view_daily`。

### 阶段 3：展示各文档访问量

- CMS 已新增 `GET /api/v1/admin/analytics/articles?days=7`。
- CMS 后台统计页面已增加文章访问量列表。
- 已支持展示今日访问量、最近 N 天访问量、总访问量。

### 阶段 4：质量收敛

- 增加基础限频或去重策略。
- 增加 CORS 白名单配置。
- 增加接口测试和导入工具测试。
- 补充部署文档和回滚说明。

## 验证清单

CMS 后端验证：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
go test ./...
```

CMS 前端验证：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
npm run build
```

Rendering 静态博客验证：

```bash
cd /path/to/Rendering
npm run check
```

接口验证：

```bash
curl -i -X POST http://127.0.0.1:8080/api/v1/articles/<slug>/views
curl -i -X POST http://127.0.0.1:8080/api/v1/analytics/site-views \
  -H 'Content-Type: application/json' \
  -d '{"path":"/blog"}'
```

数据库验证：

```bash
docker exec rendering-cms-postgres psql -U rendering -d rendering_cms -c "
select * from site_view_daily order by view_date desc limit 7;
"

docker exec rendering-cms-postgres psql -U rendering -d rendering_cms -c "
select a.slug, a.title, v.view_date, v.views
from article_view_daily v
join articles a on a.article_id = v.article_id
order by v.view_date desc, v.views desc
limit 20;
"

docker exec rendering-cms-postgres psql -U rendering -d rendering_cms -c "
select * from site_view_history order by view_date desc limit 7;
"

docker exec rendering-cms-postgres psql -U rendering -d rendering_cms -c "
select a.slug, a.title, v.view_date, v.views
from article_view_history v
join articles a on a.article_id = v.article_id
order by v.view_date desc, v.views desc
limit 20;
"
```

## 风险与处理

- CMS 中缺少文章 slug：文章访问接口会返回 `404`，需要先跑 MDX 导入。
- CORS 配置不正确：浏览器上报会失败，生产推荐走同域反向代理。
- 文章详情页重复上报：需要确保文章页只调用文章访问接口。
- 机器人访问导致数据偏高：第一阶段接受该偏差，后续再加 user-agent 过滤或限频。
- 用户快速刷新导致计数偏高：第一阶段按 PV 计数，后续再做短窗口去重。
- CMS API 不可用：Rendering 页面不能受影响，Tracker 必须静默失败。

## 后续扩展

- 按路径统计非文章页面访问量。
- 按 referrer host 统计来源。
- 按文章标签汇总访问量。
- 按小时维度统计趋势。
- 增加访问去重策略，例如 `date + slug + ip_hash + user_agent_hash`。
- 后台支持导出访问统计 CSV。

## 相关文档

- [CMS 文章发布到 Rendering 博客访问方案](./rendering-blog-publishing-integration.md)
