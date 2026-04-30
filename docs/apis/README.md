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
