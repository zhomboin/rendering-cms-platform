# 统计 API

本文档记录 MVP 阶段文章访问写入和后台统计汇总接口。所有接口前缀为 `/api/v1`。

## 写入文章访问

```http
POST /api/v1/articles/{slug}/views
```

说明：

- 公开接口。
- `{slug}` 是 CMS 返回的 6 位短链码，格式为 `^[0-9A-Za-z]{6}$`。
- 兼容期内，如果 `{slug}` 不是 6 位短链码，后端会按 `articleName` 解析到对应文章后写入同一篇文章的访问量。
- Rendering 博客正式实现必须使用短链 `slug` 上报，不应长期使用 `articleName`。
- 仅对已发布文章写入访问统计。
- 每次调用使 `article_view_daily` 中当日文章访问量加 `1`。
- 同时使 `site_view_daily` 中当日站点访问量加 `1`。
- 后端会先写入 `analytics_events`，并按 `event_type + ip_hash + article_id + event_date` 做每日去重；重复事件返回 `204`，但不重复累加 daily 统计。
- `article_view_daily` 只保存当天实时计数，历史日期统计应在每日归档后进入 `article_view_history`。
- Rendering 静态博客文章详情页接入时，必须使用 CMS 返回的短链码上报文章访问。

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
- 后端会先写入 `analytics_events`，并按 `event_type + ip_hash + event_date` 做每日去重；重复事件返回 `204`，但不重复累加 daily 统计。
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
      "slug": "aB3dE9",
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
      "slug": "aB3dE9",
      "title": "Hello World",
      "todayViews": 12,
      "periodViews": 86,
      "totalViews": 324,
      "publishedAt": "2026-03-18T00:00:00Z"
    }
  ]
}
```

## 后台访问趋势

```http
GET /api/v1/admin/analytics/trend?days=30
Authorization: Bearer <jwt-token>
```

说明：

- 返回最近 N 天站点访问趋势和文章访问趋势。
- `days` 允许值为 `7`、`30`、`90`，未传或传入其它值时使用 `30`。
- `site` 返回连续日期序列，无访问量日期返回 `0`。
- `articles` 返回最近 N 天内存在访问量的已发布文章按日趋势。

响应：

```json
{
  "days": 30,
  "site": [
    {
      "date": "2026-05-12",
      "views": 42
    }
  ],
  "articles": [
    {
      "date": "2026-05-12",
      "slug": "aB3dE9",
      "title": "Hello World",
      "views": 12
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
- 增强阶段新增 `analytics_events` 作为访问事件明细表，并用于公开统计接口去重；当前趋势接口仍优先读取 daily/history 聚合表。
- `analytics_events` 通过唯一索引限制同一 `event_type + ip_hash + article_id + event_date` 重复计数；站点访问使用空文章 ID 参与同一规则。

## Rendering 博客对接

详细方案见 [Rendering 博客访问统计对接方案](../operations/rendering-blog-analytics-integration.md)。
