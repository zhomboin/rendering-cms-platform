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

- [x] 在 `frontend/src/api/client.ts` 定义 `API_BASE`，默认值为 `http://127.0.0.1:8080/api/v1`。
- [x] 在 `client.ts` 实现 `apiGet<T>(path: string)`。
- [x] 在 `client.ts` 实现 `apiPost<T>(path: string, body: unknown)`。
- [x] 在 `client.ts` 统一处理非 2xx 响应。
- [x] 创建 `LoginPage.tsx`，包含邮箱、密码和登录按钮。
- [x] 创建 `ArticleListPage.tsx`，用于公开文章列表。
- [x] 创建 `ArticleDetailPage.tsx`，用于公开文章详情和评论展示入口。
- [x] 创建 `AdminArticleListPage.tsx`，用于后台文章列表。
- [x] 创建 `AdminArticleEditorPage.tsx`，包含标题、slug、摘要、标签、MDX 正文、保存草稿、发布按钮。
- [x] 将 `AdminDashboardPage.tsx` 接入 API client。
- [x] 将 `AdminCommentsPage.tsx` 接入评论审核 API 的页面结构。
- [x] 将 `AdminAssetsPage.tsx` 接入上传和下载 API 的页面结构。
- [x] 在 `routes/index.tsx` 声明公开路由：`/`、`/articles/:slug`。
- [x] 在 `routes/index.tsx` 声明后台路由：`/admin/login`、`/admin`、`/admin/articles`、`/admin/articles/new`、`/admin/comments`、`/admin/assets`。
- [x] 在 `main.tsx` 挂载 React Router 和 TanStack Query。
- [x] 运行 `cd frontend && npm run build`。

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

## 完成记录

- API client 使用 axios 实例封装，统一读取 `VITE_API_BASE`，默认回退到 `http://127.0.0.1:8080/api/v1`，并统一处理非 2xx 响应。
- axios 请求拦截器统一设置后台登录 token，响应拦截器遇到后台 `401` 时清理本地 token 并跳转登录页。
- 具体业务 API 统一维护在 `frontend/src/api/` 下的领域文件中，页面不直接调用底层 `apiGet`、`apiPost`、`apiPatch`、`apiClient`、`fetch` 或 `axios` 方法。
- 登录成功后保存 Bearer token，后台 API 请求自动携带 `Authorization`。
- 未登录访问 `/admin` 及其子路由时跳转到 `/admin/login`，登录成功后回到原目标后台页面。
- 公开文章列表、文章详情、评论提交、后台文章列表、文章编辑、统计、评论和资源页面均已接入 API client。
- `/` 默认跳转到后台仪表盘 `/admin`，后台页面由统一侧边栏壳层承载。
- `/articles` 作为公开文章列表入口，`/articles/:slug` 作为详情页入口。
