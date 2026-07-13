# API 文档索引

本目录记录 Rendering CMS Platform MVP 的接口约定。所有业务接口默认使用 `/api/v1` 前缀。

- [认证 API](./auth.md)
- [文章 API](./articles.md)
- [统计 API](./analytics.md)
- [评论 API](./comments.md)
- [文件 API](./assets.md)

## 通用规则

- 后台接口路径使用 `/api/v1/admin/*`。
- 后台接口必须携带 `Authorization: Bearer <jwt-token>`。
- 错误响应使用 JSON：

```json
{
  "error": "错误说明"
}
```

- 评论、下载审计和访问统计不得保存原始 IP；如需识别访问端，只保存哈希值。
- 触发速率限制时返回 `429` 并携带 `Retry-After`；公共并发容量耗尽时可能返回 `503`。
- 公开 GET 可使用 `Cache-Control`、`ETag`、`If-None-Match` 和 `304`；后台和写入接口不得进入公共缓存。
