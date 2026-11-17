# Rendering 博客发布对接

CMS 管理文章的草稿、发布状态和修订，Rendering 负责公开页面、SEO 数据和 Pagefind 索引。CMS 接口以 [文章 API](../apis/articles.md)为准，两仓使用同一套 [Markdown 规则](../guides/markdown.md)。

## 内容消费

- Rendering 服务端用 `GET /api/v1/articles` 读取轻量列表，用 `GET /api/v1/articles/{slug}` 读取详情及 `bodyMdx`。公开链接、canonical 与上报使用 `canonicalSlug`，CMS 也支持通过 `articleName` 查找已发布文章。
- Rendering 合并 CMS 和本地 MDX。CMS 记录优先，匹配的本地 MDX 以英文名去重。只有本地 MDX 的文章可继续独立展示，CMS 请求失败时 Rendering 可读取本地 MDX。详情取不到 CMS 数据时也会尝试回退。
- 摘要和标签是可选字段。Rendering 以标题填充空摘要的页面描述，空标签按空数组展示。消费逻辑和回归见 [CMS 内容客户端](https://github.com/zhomboin/Rendering/blob/main/lib/cms-content.js) 与 [内容消费测试](https://github.com/zhomboin/Rendering/blob/main/lib/cms-content.test.js)。
- CMS 下架后，公开详情、搜索、评论和访问入口按已发布状态过滤。Rendering 还受页面缓存、索引和本地 MDX 数据源影响。验收时分别确认 CMS 短链、博客 canonical、站点地图、RSS 与索引，以及存在本地副本时的预期可见性。[Rendering 公开数据源](https://github.com/zhomboin/Rendering/blob/main/lib/public-content.js)

## 配置与索引刷新

在 Rendering 的 Web 环境配置中，将服务端 `CMS_API_BASE` 设置为 `https://cms.zhomboin.com/api/v1`，确保构建和运行时读取同一地址。客户端上报另用构建期的 `NEXT_PUBLIC_CMS_API_BASE)，见 [统计对接](rendering-blog-analytics-integration.md)。Rendering 对 CMS 文章读取设置 60 秒缓存刷新周期，另外使用可配置的超时及列表、详情响应大小上限。

Rendering 使用构建期 Pagefind。其运维文档记录 CMS 内容变更可触发重建，即使 Rendering 的 Git 提交未变化。触发器、认证、日志与重试以实际环境配置为准。普通 Git 定时任务在没有新提交时会退出。需要人工重建时，从 Rendering 运维入口执行经批准的强制部署。正在运行的应用通过健康检查后才会切换已暂存的搜索索引。[Rendering 部署与索引操作](https://github.com/zhomboin/Rendering/blob/main/docs/linux-systemd-deployment.zh-CN.md#cms-内容更新与重建)

## 验收与故障归属

1. 在 CMS 验证发布与下架、公开列表与详情、缓存响应和规范短链。
2. 到 Rendering 核对首页、详情、RSS、站点地图与 canonical。样本包括空摘要、空标签、CMS 专属文章及有本地 MDX 副本的文章。
3. 核对发布联动的日志、应用健康和 Pagefind 切换结果。用标题检索文章并打开正文。下架后再核对列表、详情和索引。
4. 有正文显示异常时按 [Markdown 规则](../guides/markdown.md)区分 CMS 预览、博客生成和索引结果。Rendering 消费问题在 Rendering 仓库处理。当前环境验收以 [V07 与 V11](../plans/current-tasks.md#待环境验收)为入口。
