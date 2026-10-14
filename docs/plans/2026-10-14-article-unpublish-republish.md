# 文章下架恢复草稿与再次发布实施计划

**目标：** 让已发布文章可以下架（下架即恢复为草稿、立即从公开接口消失），在后台编辑器中修改后再次发布。

**已确认决策：**

- 下架为一步操作：`published -> draft`，不启用 `archived` 中间态。
- 再次发布时 `published_at` 重置为本次发布时间，文章按新发布时间参与公开列表排序。

**架构：** 后端新增下架接口，发布接口增加“仅草稿可发布”前置校验；修订历史继续由数据库触发器自动写入，不需要 migration。前端在文章列表和编辑器提供下架入口，编辑器只读锁定覆盖所有非草稿状态。

**技术栈：** Go、Chi、pgx/sqlc、PostgreSQL、React + TypeScript + Vite。

## 执行状态（2026-10-14）

- [x] `UnpublishArticle` SQL 查询（`where status = 'published'` 前置校验）已实现并通过 `sqlc generate`。
- [x] `PublishArticle` SQL 增加 `where status = 'draft'` 前置校验，`published_at` 改为每次发布重置为 `now()`。
- [x] `POST /api/v1/admin/articles/{id}/unpublish` handler 已实现：404/409 语义与并发竞态兜底。
- [x] `publishArticle` handler 加固：读取文章区分 404，非草稿返回 409。
- [x] 后端测试覆盖：下架成功/409/404/竞态 409，发布成功/409/404。
- [x] 前端 `unpublishAdminArticle` API 封装已实现。
- [x] 文章列表页已发布行新增「下架」操作（Popconfirm 确认）。
- [x] 编辑器只读锁定扩展到所有非草稿状态，已发布文章提供「下架为草稿」按钮，本地草稿在下架后保留合并。
- [x] `docs/apis/articles.md`、`docs/sql/schema.md`、`docs/operations/rendering-blog-publishing-integration.md` 已同步。
- [x] `go test ./...`、`go vet ./...`、`npm run build` 全部通过。
- [ ] 预发布/生产环境执行端到端验收：发布 → 下架 → 公开接口 404 → 修改 → 再次发布 → `published_at` 重置。

## 状态机口径

```text
draft --发布 publish--> published
published --下架 unpublish--> draft（可修改后再次发布）
archived：保留值，无接口可进入或离开
```

## 行为变更说明

- 发布接口此前对任何状态的文章都会执行（重复发布白白增加 `version` 并写日志）；现在仅草稿可发布，非草稿返回 `409`。
- 发布接口此前保留最早的 `published_at`（`coalesce`）；现在每次发布都重置为当前时间。
- 更新草稿接口行为不变：非草稿一律 `409`。

## 边界与已知影响

- 下架后公开详情返回 `404`（15 秒负缓存），成功响应边缘缓存最多 60 秒后过期。
- Rendering 静态页面和 Pagefind 索引需触发重建后才会移除，触发路径与发布一致。
- 已审核评论与历史访问统计保留；评论随文章下架不可见，再次发布后恢复。
- 仪表盘热门文章可能继续展示已下架文章的历史访问数据，如需按状态过滤另开任务。
