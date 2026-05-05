# 统计 API

本文档记录 MVP 阶段文章访问写入和后台统计汇总接口。所有接口前缀为 `/api/v1`。

## 写入文章访问

```http
POST /api/v1/articles/{slug}/views
```

说明：

- 公开接口。
- 仅对已发布文章写入访问统计。
- 每次调用使 `article_view_daily` 中当日文章访问量加 `1`。
- 同时使 `site_view_daily` 中当日站点访问量加 `1`。
- `article_view_daily` 只保存当天实时计数，历史日期统计应在每日归档后进入 `article_view_history`。
- Rendering 静态博客文章详情页接入时，必须先确保 CMS `articles` 表中存在相同 `slug` 的已发布文章。

成功响应：

```http
204 No Content
```

## 写入站点访问

```http
POST /api/v1/analytics/site-views
```

说明：

- 公开接口。
- 用于 Rendering 首页、文章列表页、关于页等非文章页面访问统计。
- 每次调用使 `site_view_daily` 中当日站点访问量加 `1`。
- 不关联具体文章，不写入 `article_view_daily`。
- `site_view_daily` 只保存当天实时计数，历史日期统计应在每日归档后进入 `site_view_history`。
- 请求体扩展字段暂不持久化；字段无效时仍按一次站点访问计数并返回 `204`。

请求体：

```json
{
  "path": "/blog",
  "referrer": "https://example.com"
}
```

成功响应：

```http
204 No Content
```

## 后台统计汇总

```http
GET /api/v1/admin/analytics/summary
Authorization: Bearer <jwt-token>
```

说明：

- 需要 `admin` 或 `editor` 角色。
- 返回今日访问量、近 7 天站点访问量和热门文章列表。
- 近 7 天站点访问量和热门文章统计应合并历史表与当天 daily 表，避免每日归档后丢失历史数据。

响应：

```json
{
  "todayViews": 128,
  "last7Days": [
    {
      "date": "2026-04-30",
      "views": 128
    }
  ],
  "hotArticles": [
    {
      "rank": 1,
      "slug": "hello-world",
      "title": "Hello World",
      "views": 42
    }
  ]
}
```

## 后台文章访问量列表

```http
GET /api/v1/admin/analytics/articles?days=7
Authorization: Bearer <jwt-token>
```

说明：

- 需要 `admin` 或 `editor` 角色。
- 用于展示各个文档的访问量。
- `days` 默认 `7`，允许范围为 `1` 到 `90`；小于 `1` 时按 `1` 处理，大于 `90` 时按 `90` 处理，非数字时使用默认值 `7`。
- `todayViews` 读取当天 `article_view_daily`。
- `periodViews` 合并最近 `days` 天的 `article_view_history` 和 `article_view_daily`。
- `totalViews` 合并全部 `article_view_history` 和 `article_view_daily`。
- 默认按 `periodViews desc, todayViews desc, publishedAt desc` 排序。

响应：

```json
{
  "days": 7,
  "articles": [
    {
      "slug": "hello-world",
      "title": "Hello World",
      "todayViews": 12,
      "periodViews": 86,
      "totalViews": 324,
      "publishedAt": "2026-03-18T00:00:00Z"
    }
  ]
}
```

## 数据规则

- 统计数据按日聚合。
- `article_view_daily` 使用 `(article_id, view_date)` 作为主键，只保存当天访问量。
- `article_view_history` 使用 `(article_id, view_date)` 作为主键，保存每日归档后的文章历史访问量。
- `site_view_daily` 使用 `view_date` 作为主键，只保存当天访问量。
- `site_view_history` 使用 `view_date` 作为主键，保存每日归档后的站点历史访问量。
- 后端服务启动后会先执行一次过期 daily 数据清理，之后每天本地时间 `00:05` 执行归档。
- 每日归档任务只处理 `view_date < current_date` 的 daily 数据，不处理当天实时计数。
- 归档 SQL 必须使用 `DELETE ... RETURNING` 把 daily 数据原子搬迁到 history 表，并在冲突时累加到已有 history 记录。
- 如果归档过程中仍有延迟访问写入旧日期 daily 表，该记录不会被本次删除；下一次归档会继续累加到 history 表，避免计数丢失。
- MVP 不保存原始 IP 地址。

## Rendering 博客对接

详细方案见 [Rendering 博客访问统计对接方案](../operations/rendering-blog-analytics-integration.md)。
