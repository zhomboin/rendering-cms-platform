# Rendering 博客访问统计对接

Rendering 上报访问，CMS 按日去重、聚合并在后台显示。接口、返回码与日期口径以 [统计 API](../apis/analytics.md)、[相册 API](../apis/photos.md)和 [首页 API](../apis/dashboard.md)为准。

## 上报链路

| 页面 | Rendering 上报方式 | CMS 计数 |
| --- | --- | --- |
| 文章详情 | 浏览器向 CMS 发送 `POST /api/v1/articles/{slug}/views`，使用 CMS 的 `canonicalSlug` | 已发布文章及站点日计数各加一 |
| 站点页面 | 浏览器向 CMS 发送 `POST /api/v1/analytics/site-views` | 站点日计数加一 |
| 相册详情 | 浏览器向 Rendering 同域 `/api/photos/{slug}/views` 上报，由服务端转发到 CMS | 已发布相册及站点日计数各加一 |

文章和站点由 [客户端上报组件](https://github.com/zhomboin/Rendering/blob/main/components/cms-analytics-tracker.tsx)发起带 `keepalive` 的请求。相册通过 [Rendering 相册服务端路由](https://github.com/zhomboin/Rendering/blob/main/lib/photo-api.js)转发。文章或相册详情应使用对应上报入口，避免再次发送站点上报。

CMS 对同一天、相同来源哈希及相同事件目标去重。文章和相册上报的关联计数与事件一起提交。成功与去重均返回 `204`。相册代理当前未转发访客 IP，CMS 按服务器侧来源进行去重。真实相册计数口径随 V04、V08 验收。首页展示日去重计数，计数含义见 [首页说明](../guides/dashboard.md)。

## 环境配置

- Rendering 浏览器端的文章和站点上报使用构建期变量 `NEXT_PUBLIC_CMS_API_BASE=https://cms.zhomboin.com/api/v1`。修改后重新构建 Rendering。
- Rendering 相册服务端使用 `CMS_API_BASE=https://cms.zhomboin.com/api/v1`，与其他公开数据读取配置一致。
- CMS 的 `FRONTEND_ORIGINS` 包含实际发起跨域上报的博客来源。生产配置见 [部署说明](deployment.md#环境变量)，本地用 `http://127.0.0.1:8080/api/v1` 并核对对应的前端开发来源。
- CMS 使用受信前置代理覆写的客户端地址计算哈希，可信代理配置见 [部署说明](deployment.md#cloudflare-橙云真实-ip)。

## 环境验收

使用获准的测试页面确认文章、站点、相册各调用一次预期入口。用同一测试来源重复上报核对去重，验证不存在或已下架内容返回 `404`，检查上报失败时博客仍可阅读。后台核对站点、文章、相册和 7／30／90 天趋势，确认相册代理来源对统计的影响。结果记入 [当前任务](../plans/current-tasks.md#待环境验收)，归档口径见 [数据库说明](../sql/schema.md#访问统计归档规则)。
