# MVP 阶段 08：前端后台壳层和公开页面

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

建立公开阅读页面和后台管理页面的前端壳层，统一 API client、路由、登录页、文章页面、统计页面、评论页面和资源页面。

## 文件范围

- Create: `frontend/src/api/client.ts`
- Create: `frontend/src/features/auth/LoginPage.tsx`
- Create: `frontend/src/features/articles/ArticleListPage.tsx`
- Create: `frontend/src/features/articles/ArticleDetailPage.tsx`
- Create: `frontend/src/features/articles/AdminArticleListPage.tsx`
- Create: `frontend/src/features/articles/AdminArticleEditorPage.tsx`
- Modify: `frontend/src/features/analytics/AdminDashboardPage.tsx`
- Modify: `frontend/src/features/comments/AdminCommentsPage.tsx`
- Modify: `frontend/src/features/assets/AdminAssetsPage.tsx`
- Modify: `frontend/src/routes/index.tsx`
- Modify: `frontend/src/main.tsx`

## 子任务

- [ ] 在 `frontend/src/api/client.ts` 定义 `API_BASE`，默认值为 `http://127.0.0.1:8080/api/v1`。
- [ ] 在 `client.ts` 实现 `apiGet<T>(path: string)`。
- [ ] 在 `client.ts` 实现 `apiPost<T>(path: string, body: unknown)`。
- [ ] 在 `client.ts` 统一处理非 2xx 响应。
- [ ] 创建 `LoginPage.tsx`，包含邮箱、密码和登录按钮。
- [ ] 创建 `ArticleListPage.tsx`，用于公开文章列表。
- [ ] 创建 `ArticleDetailPage.tsx`，用于公开文章详情和评论展示入口。
- [ ] 创建 `AdminArticleListPage.tsx`，用于后台文章列表。
- [ ] 创建 `AdminArticleEditorPage.tsx`，包含标题、slug、摘要、标签、MDX 正文、保存草稿、发布按钮。
- [ ] 将 `AdminDashboardPage.tsx` 接入 API client。
- [ ] 将 `AdminCommentsPage.tsx` 接入评论审核 API 的页面结构。
- [ ] 将 `AdminAssetsPage.tsx` 接入上传和下载 API 的页面结构。
- [ ] 在 `routes/index.tsx` 声明公开路由：`/`、`/articles/:slug`。
- [ ] 在 `routes/index.tsx` 声明后台路由：`/admin/login`、`/admin`、`/admin/articles`、`/admin/articles/new`、`/admin/comments`、`/admin/assets`。
- [ ] 在 `main.tsx` 挂载 React Router 和 TanStack Query。
- [ ] 运行 `cd frontend && npm run build`。

## 验收标准

- 前端所有 MVP 页面都有路由入口。
- API base URL 不在各页面重复硬编码。
- 后台页面布局偏运营工具，不做营销式落地页。
- `npm run build` 通过。

## 建议提交

```bash
git add frontend/src
git commit -m "feat: add cms frontend shell"
```
