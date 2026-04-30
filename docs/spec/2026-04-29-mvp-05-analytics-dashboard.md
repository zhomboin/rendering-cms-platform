# MVP 阶段 05：统计 API 和后台看板

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

实现文章访问量和站点访问量的基础日聚合，并在后台看板展示今日访问量、近 7 天访问量和热门文章。

## 文件范围

- Create: `backend/internal/analytics/service.go`
- Create: `backend/internal/analytics/service_test.go`
- Create: `backend/internal/analytics/handler.go`
- Modify: `backend/sql/analytics.sql`
- Create: `frontend/src/features/analytics/AdminDashboardPage.tsx`
- Create: `docs/apis/analytics.md`

## 子任务

- [ ] 在 `service_test.go` 编写 `TestSummaryTotals`。
- [ ] 在 `service.go` 定义 `DailyView` 和 `TotalViews(days []DailyView) int`。
- [ ] 在 `backend/sql/analytics.sql` 增加文章日访问量 upsert SQL。
- [ ] 在 `backend/sql/analytics.sql` 增加站点日访问量 upsert SQL。
- [ ] 在 `backend/sql/analytics.sql` 增加今日访问量查询 SQL。
- [ ] 在 `backend/sql/analytics.sql` 增加近 7 天访问量查询 SQL。
- [ ] 在 `backend/sql/analytics.sql` 增加热门文章查询 SQL。
- [ ] 在 `handler.go` 实现 `POST /api/v1/articles/{slug}/views`。
- [ ] 在 `handler.go` 实现 `GET /api/v1/admin/analytics/summary`。
- [ ] 在前端创建 `AdminDashboardPage.tsx`。
- [ ] 看板展示今日访问量、近 7 天访问量、热门文章列表三个区域。
- [ ] 创建 `docs/apis/analytics.md`，记录统计写入和后台汇总 API。
- [ ] 运行 `cd backend && go test ./internal/analytics`。
- [ ] 运行 `cd frontend && npm run build`。

## 验收标准

- 文章访问写入 `article_view_daily`。
- 站点访问写入 `site_view_daily`。
- 后台 summary API 需要认证。
- 前端看板页面可被路由引用并通过构建。

## 建议提交

```bash
git add backend/internal/analytics backend/sql/analytics.sql frontend/src/features/analytics docs/apis/analytics.md
git commit -m "feat: add analytics dashboard foundation"
```
