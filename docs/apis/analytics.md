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

## 数据规则

- 统计数据按日聚合。
- `article_view_daily` 使用 `(article_id, view_date)` 作为主键。
- `site_view_daily` 使用 `view_date` 作为主键。
- MVP 不保存原始 IP 地址。
