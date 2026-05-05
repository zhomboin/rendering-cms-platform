# CMS 文章发布到 Rendering 博客访问方案

本文档定义 Rendering CMS Platform 中已发布文章如何在 Rendering 博客平台访问。该方案用于后续实现 Rendering 前台读取 CMS API 的改造。

## 背景

当前 CMS 后台的文章发布功能只完成 CMS 内部发布：

- 将 `articles.status` 更新为 `published`。
- 首次发布时写入 `published_at`。
- `articles.version` 自动加 `1`，并由数据库触发器写入 `article_logs`。
- CMS 自身公开前端可通过 `/articles` 和 `/articles/{slug}` 访问已发布文章。

当前发布功能不会：

- 写回 Rendering 静态博客仓库的 `content/posts/*.mdx`。
- 自动提交 Rendering 仓库。
- 自动触发 Rendering 博客重新构建或部署。
- 自动让文章出现在 `https://rendering.me/blog/<slug>`。

## 目标

- CMS 发布文章后，Rendering 博客平台可以通过 `/blog/<slug>` 访问该文章。
- Rendering 博客前台保持 SSR、SSG 或 ISR 能力。
- CMS 继续作为文章运行时数据源。
- Rendering 静态博客不直接连接 PostgreSQL。
- Rendering 静态博客不再依赖把 CMS 文章写回 `content/posts/` 才能展示新文章。

## 非目标

- 不让 CMS 后端直接写入 Rendering 静态博客仓库。
- 不在第一阶段实现 CMS 发布后自动创建 Git commit。
- 不在第一阶段实现复杂预览、定时发布、审批流或多版本回滚。
- 不把文章正文放到浏览器端再请求后渲染为主要方案。

## 推荐方案

推荐让 Rendering 博客前台在服务端读取 CMS 公开 API，并使用 Next.js ISR。

核心方式：

```ts
await fetch(`${CMS_API_BASE}/articles/${slug}`, {
  next: { revalidate: 60 },
});
```

这样：

- Rendering 页面仍然可以服务端渲染。
- 页面可以被 Next.js 缓存。
- CMS 发布后不需要完整重新构建 Rendering。
- 最多等待 `revalidate` 时间后，公开页面会刷新到 CMS 最新数据。

## 模式对比

| 模式 | SSR/SSG | CMS 发布后可见 | 优点 | 风险 |
|---|---|---|---|---|
| 构建期读取 CMS | 可用 | 需要重新构建 | 静态化程度最高 | 发布链路慢 |
| ISR 读取 CMS | 可用 | 等待 revalidate 或手动刷新 | 平衡性能和动态发布 | 依赖 CMS API 可用性 |
| 每次请求 SSR 读取 CMS | 可用 | 立即可见 | 数据最新 | CMS API 故障会影响页面 |
| 客户端读取 CMS | 首屏正文不可 SSR | 立即可见 | 实现简单 | SEO 和首屏体验较差 |

第一阶段选择 ISR 读取 CMS。

## Rendering 前台数据源策略

### 文章列表页

Rendering 的 `/blog` 应优先读取 CMS：

```http
GET /api/v1/articles
```

用途：

- 展示 CMS 中 `published` 状态文章。
- 按发布时间倒序排列。
- 作为博客首页或文章列表的数据源。

### 文章详情页

Rendering 的 `/blog/<slug>` 应读取 CMS：

```http
GET /api/v1/articles/{slug}
```

用途：

- 根据 `slug` 获取已发布文章。
- 只展示 `published` 状态文章。
- 未发布或不存在时返回 `404`，Rendering 前台应展示 not found 页面。

### 元数据生成

Rendering 的 `generateMetadata` 应使用同一个 CMS API 读取文章标题和摘要：

```ts
export async function generateMetadata({ params }) {
  const article = await getCMSArticle(params.slug);

  return {
    title: article.title,
    description: article.summary,
  };
}
```

这样可以保留文章详情页的 SEO 元数据能力。

## 环境变量

Rendering 博客平台应新增服务端环境变量：

```env
CMS_API_BASE=https://rendering.me/api/v1
```

本地开发示例：

```env
CMS_API_BASE=http://127.0.0.1:8080/api/v1
```

说明：

- 服务端渲染读取 CMS API 时优先使用 `CMS_API_BASE`。
- 不要只依赖 `NEXT_PUBLIC_CMS_API_BASE` 承载文章主内容读取。
- `NEXT_PUBLIC_` 变量仅用于浏览器端访问统计 Tracker 等公开上报场景。

## 生产访问路径

推荐生产部署使用同域反向代理：

```text
https://rendering.me/api/v1/* -> Rendering CMS Platform backend
```

优点：

- Rendering 前台服务端和浏览器都能用同一个公开 API 地址。
- 减少 CORS 配置复杂度。
- 后续访问统计上报也可以使用同域路径。

如果 CMS API 使用独立域名：

```text
https://cms.rendering.me/api/v1
```

则需要确保：

- Rendering 服务器可以访问该域名。
- CMS 后端 CORS 允许 `https://rendering.me`。
- SSR 请求不能使用只在本机可达的 `127.0.0.1`，除非 Rendering 与 CMS 在同一主机且该地址确实指向 CMS 服务。

## SSR 与 ISR 规则

Rendering 前台改读 CMS API 不会导致 SSR 不可用。需要遵守以下规则：

- 文章主内容读取在服务端完成，不放到客户端组件里作为唯一数据源。
- `generateMetadata` 使用服务端 API 读取文章元数据。
- `/blog/<slug>` 页面使用服务端组件或服务端数据函数读取 CMS。
- 第一阶段使用 `next: { revalidate: 60 }`，避免每次请求都打到 CMS。
- CMS API 请求失败时，Rendering 应区分 `404` 和服务异常。

建议行为：

- CMS 返回 `404`：Rendering 返回 not found。
- CMS 返回 `500` 或网络错误：Rendering 可以抛错进入错误页，或者回退到旧静态 MDX。
- CMS 返回文章数据：Rendering 渲染 CMS 文章正文。

## 兼容旧 MDX 内容

第一阶段建议保留旧 MDX 作为回退数据源：

```text
优先读取 CMS published 文章
  -> CMS 不存在该 slug 时
  -> 回退读取 Rendering content/posts/*.mdx
```

这样可以降低切换风险：

- 新文章可以从 CMS 发布后被 Rendering 访问。
- 旧文章即使尚未导入 CMS，也仍可由静态 MDX 提供。
- 当 CMS 数据完整且稳定后，再决定是否移除 MDX 运行时依赖。

注意：

- 如果某个 slug 同时存在于 CMS 和 MDX，优先展示 CMS 版本。
- 导入工具应保持 CMS slug 与 Rendering MDX 文件名一致。
- 回退只在 Rendering 前台读取层实现，CMS 后端不写回 MDX。

## 与访问统计的关系

发布链路和访问统计链路是两个独立能力：

- 发布链路：Rendering 前台读取 CMS `GET /api/v1/articles` 和 `GET /api/v1/articles/{slug}`。
- 统计链路：Rendering 前台上报 `POST /api/v1/articles/{slug}/views` 和 `POST /api/v1/analytics/site-views`。

文章详情页读取 CMS 成功后，仍应触发文章访问统计上报。

详细访问统计方案见：

```text
docs/operations/rendering-blog-analytics-integration.md
```

## 实施阶段

### 阶段 1：准备 CMS 数据源

- 确认 CMS `GET /api/v1/articles` 和 `GET /api/v1/articles/{slug}` 返回字段满足 Rendering 前台渲染。
- 使用 `cmd/import-mdx` 将 Rendering 现有 MDX 导入 CMS。
- 验证 CMS 中目标文章状态为 `published`。

### 阶段 2：Rendering 增加 CMS API 客户端

- 新增服务端 API client，例如 `lib/cms-content.js`。
- 读取 `CMS_API_BASE`。
- 实现 `getCMSPublishedPosts()`。
- 实现 `getCMSPostBySlug(slug)`。
- 对 `404`、网络错误和无效响应做明确处理。

### 阶段 3：改造文章列表页

- `/blog` 优先读取 CMS 文章列表。
- 保留旧 MDX 列表作为回退。
- 保持原有标签、摘要、发布时间和链接结构。

### 阶段 4：改造文章详情页

- `/blog/<slug>` 优先读取 CMS 文章详情。
- 使用 CMS `bodyMdx` 进入现有 MDX 渲染流程。
- `generateMetadata` 使用 CMS 标题和摘要。
- CMS 不存在时回退旧 MDX。

### 阶段 5：接入 ISR 与缓存刷新

- 第一阶段设置 `revalidate: 60`。
- 后续可以增加 CMS 发布后调用 Rendering revalidate webhook。
- webhook 可按 slug 精准刷新 `/blog/<slug>` 和 `/blog`。

### 阶段 6：收敛旧静态数据依赖

- 确认所有历史文章已导入 CMS。
- 对比 CMS 与 MDX 的 slug、标题、发布时间和标签。
- 决定是否继续保留 MDX 回退。

## 验证清单

CMS 侧：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
go test ./...
```

确认 CMS API 可读：

```bash
curl -fsS http://127.0.0.1:8080/api/v1/articles
curl -fsS http://127.0.0.1:8080/api/v1/articles/<slug>
```

Rendering 侧：

```bash
cd /path/to/Rendering
npm run check
```

浏览器验收：

- CMS 后台发布新文章。
- 打开 `https://rendering.me/blog/<slug>`。
- 确认页面显示 CMS 发布的标题、摘要、正文、标签和发布时间。
- 确认页面源代码中包含文章正文，避免正文只在客户端加载。
- 等待 `revalidate` 时间后，确认 CMS 修改能反映到 Rendering 页面。

## 风险与处理

- CMS API 不可用：保留 MDX 回退，或让 Rendering 错误页明确提示。
- CMS 数据未导入完整：优先执行 `cmd/import-mdx`，并检查 slug 对齐。
- 缓存导致发布不立即可见：第一阶段接受 60 秒延迟，后续增加 revalidate webhook。
- `127.0.0.1` 指向错误服务：生产使用反向代理地址或服务内网地址。
- MDX 渲染差异：复用 Rendering 当前 MDX 渲染组件，避免 CMS 前台和 Rendering 前台渲染不一致。
- SEO 元数据缺失：`generateMetadata` 必须读取 CMS 标题与摘要。

## 当前结论

当前 CMS 发布功能尚不会自动把文章发布到 Rendering 博客平台。后续应通过 Rendering 前台读取 CMS 公开 API 的方式打通 `/blog/<slug>` 访问链路，并优先采用 ISR 保持 SSR 能力和较好的发布时效。
