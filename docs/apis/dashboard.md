# 首页数据驾驶舱 API

首页使用三个只读后台接口和[访问趋势接口](analytics.md)。所有接口前缀为 /api/v1，要求有效登录令牌及 admin 或 editor 角色。没有公开读取入口，响应不允许缓存。

## 通用约定

- 三个接口均接受 days，允许 7、30、90；未传、空值或其它值使用 30。
- 统计窗口包含数据库当前日期，开始日期为当前日期向前 days 减一天；排除未来日期。时间戳使用带时区的标准日期格式。
- 响应直接返回对象，不额外套 data。列表无记录时为空数组，数量无记录时为 0；查询失败返回 500 和中文错误，不用 0 代替读取失败。
- 访问计数沿用现有每日按 IP 哈希、事件类型和内容去重规则，不能解释为标准 PV 或独立访客 UV；不同内容及日期的计数不能当作去重人数相加。
- 后台汇总不返回文章正文、邮箱、IP 哈希、User-Agent、存储 key 或下载签名，不触发下载审计。
- 文章与站点统计合并历史表和日表：历史表只读取今天之前的数据，日表包含归档后到达的增量。相册读取相册日表。

## 运营概况

GET /admin/dashboard/overview

| 字段 | 内容与范围 |
| --- | --- |
| days、startDate、endDate | 实际窗口天数、开始日期和结束日期 |
| generatedAt | 汇总生成时间 |
| metrics.todayViews | 今日站点访问计数 |
| metrics.periodViews | 所选区间站点访问计数，包含文章、相册及站点上报 |
| metrics.articleViews | 所选区间当前已发布文章访问计数，与文章趋势范围一致 |
| metrics.albumViews | 所选区间相册访问计数，保留已下架相册的计数 |
| metrics.newComments | 所选区间新增评论数量，包含所有审核状态 |
| metrics.pendingComments | 当前待审核评论数量，不随区间变化 |
| content | 当前文章各状态数量、相册各状态数量、有效照片数量及已发布文章缺封面数量 |
| resources | 有效资源数量、图片数量和资源登记大小（字节） |
| recentArticles | 最近更新文章摘要，最多 5 项，含私有状态，仅后台可读 |
| drafts | 最近更新草稿摘要，最多 3 项，不含私有文章 |
| pendingComments | 最新待审核评论摘要，最多 3 项，正文最多 120 个字符 |
| recentImages | 最近登记且有有效公开图片地址的图片资源，最多 6 项 |

content 的数量字段为 publishedArticles、draftArticles、privateArticles、archivedArticles、publishedAlbums、draftAlbums、photoCount、missingCoverArticles。资源字段为 assetCount、imageCount、totalBytes；有效资源要求状态 active 且删除时间为空。登记大小来源于数据库元数据，不代表对象存储实际用量，也不能证明直传文件已经完整到达对象存储。

文章摘要包含 articleId、slug、title、status、coverImageUrl、tags、updatedAt、publishedAt。评论摘要包含 commentId、articleId、articleTitle、authorName、body、createdAt。图片摘要包含 assetId、filename、publicUrl、byteSize、createdAt。

## 热门文章

GET /admin/dashboard/articles

响应包含 days 和 articles。仅返回当前已发布文章，按区间计数降序、发布时间降序和文章 ID 排序，最多 5 项；无访问记录的已发布文章也可进入榜单。

每项包含 articleId、slug、title、coverImageUrl、tags、periodViews、totalViews、publishedAt。periodViews 为区间计数，totalViews 为截至统计日期的累计计数。封面地址缺失或无效时为空字符串，tags 缺失时为空数组，未设置发布时间时为 null。

## 相册表现

GET /admin/dashboard/albums

响应包含 days 和 albums，最多 4 项。已发布相册优先，再按区间计数降序、更新时间降序和相册 ID 排序；草稿可用于继续整理相册。

每项包含 albumId、slug、title、status、coverUrl、photoCount、periodViews、updatedAt。照片数量只计算有效资源；封面优先使用相册指定照片，缺失时取排序最前的有效公开照片，没有可用地址则为空字符串。

## 实现与验证入口

- [后台处理与映射](../../backend/internal/dashboard/handler.go)
- [只读聚合查询](../../backend/sql/dashboard.sql)
- [接口与权限测试](../../backend/internal/dashboard/handler_test.go)
- [PostgreSQL 边界验证](../../backend/internal/dashboard/integration_test.go)
- [前端类型与请求](../../frontend/src/api/dashboard.ts)

本地仅运行本次改动影响的用例。数据库集成测试通过 TEST_DATABASE_URL 显式连接专用测试库，创建和清理隔离 schema；未配置时跳过，不能视为数据库验收通过。GitHub 的[全量验证](../../.github/workflows/verify.yml)配置专用 PostgreSQL 并执行全量后端测试、查询生成检查、前端构建和全部前端回归。生产数据和图片访问效果需部署后另行验收。
