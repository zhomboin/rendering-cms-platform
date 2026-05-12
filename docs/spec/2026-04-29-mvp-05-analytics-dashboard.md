# MVP 阶段 05：统计 API 和后台看板

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

实现文章访问量和站点访问量的基础日聚合，并在后台看板展示今日访问量、近 7 天访问量和热门文章。

## 文件范围

- Create: `backend/internal/analytics/service.go`
- Create: `backend/internal/analytics/service_test.go`
- Create: `backend/internal/analytics/handler.go`
- Modify: `backend/sql/analytics.sql`
- Create: `frontend/src/pages/dashboard/DashboardPage.tsx`
- Create: `docs/apis/analytics.md`

## 子任务

- [x] 在 `service_test.go` 编写 `TestSummaryTotals`。
- [x] 在 `service.go` 定义 `DailyView` 和 `TotalViews(days []DailyView) int`。
- [x] 在 `backend/sql/analytics.sql` 增加文章日访问量 upsert SQL。
- [x] 在 `backend/sql/analytics.sql` 增加站点日访问量 upsert SQL。
- [x] 在 `backend/sql/analytics.sql` 增加今日访问量查询 SQL。
- [x] 在 `backend/sql/analytics.sql` 增加近 7 天访问量查询 SQL。
- [x] 在 `backend/sql/analytics.sql` 增加热门文章查询 SQL。
- [x] 在 `handler.go` 实现 `POST /api/v1/articles/{slug}/views`。
- [x] 在 `handler.go` 实现 `GET /api/v1/admin/analytics/summary`。
- [x] 在前端创建 `DashboardPage.tsx`。
- [x] 看板展示今日访问量、近 7 天访问量、热门文章列表三个区域。
- [x] 创建 `docs/apis/analytics.md`，记录统计写入和后台汇总 API。
- [x] 运行 `cd backend && go test ./internal/analytics`。
- [x] 运行 `cd frontend && npm run build`。

## 完成记录

- 完成时间：2026-04-30。
- 文章访问写入会同步更新当天 `article_view_daily` 和 `site_view_daily`。
- 后端服务启动后先清理过期 daily 数据，之后每天本地时间 `00:05` 将历史日期 daily 数据归档到 `article_view_history` 和 `site_view_history`。
- 归档使用 `DELETE ... RETURNING` 原子搬迁，并发访问写入的旧日期 daily 记录由下一次归档继续累加处理。
- 后台统计汇总接口挂载在 `/api/v1/admin/analytics/summary`，由后台认证中间件保护。
- 前端看板已接入 API client，展示今日访问、近 7 天访问和热门文章。
- 验证命令已通过：`cd backend && go test ./internal/analytics ./...`、`cd frontend && npm run build`。

## 验收标准

- 文章访问写入当天 `article_view_daily`。
- 站点访问写入当天 `site_view_daily`。
- 历史统计应从 `article_view_history` 和 `site_view_history` 读取，必要时合并当天 daily 数据。
- daily 到 history 的归档不能使用“先 insert history 再 delete daily”的两步 SQL。
- 后台 summary API 需要认证。
- 前端看板页面可被路由引用并通过构建。

## 建议提交

```bash
git add backend/internal/analytics backend/sql/analytics.sql frontend/src/pages/dashboard docs/apis/analytics.md
git commit -m "feat: add analytics dashboard foundation"
```
