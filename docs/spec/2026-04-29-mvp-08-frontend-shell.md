# MVP 阶段 08：管理端前端壳层

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

建立 CMS 管理端前端壳层，统一 API client、路由、登录页、文章管理、统计页面、评论审核和资源管理页面。CMS 前端不承载公开文章阅读页面，公开展示由独立的 Rendering 博客项目读取 CMS 已发布内容后完成。

## 文件范围

- Create: `frontend/src/api/client.ts`
- Create: `frontend/src/pages/auth/LoginPage.tsx`
- Create: `frontend/src/pages/articles/ArticleListPage.tsx`
- Create: `frontend/src/pages/articles/ArticleEditorPage.tsx`
- Modify: `frontend/src/pages/dashboard/DashboardPage.tsx`
- Modify: `frontend/src/pages/comments/CommentsPage.tsx`
- Modify: `frontend/src/pages/assets/AssetsPage.tsx`
- Modify: `frontend/src/routes/index.tsx`
- Modify: `frontend/src/main.tsx`

## 子任务

- [x] 在 `frontend/src/api/client.ts` 定义 `API_BASE`，默认值为 `http://127.0.0.1:8080/api/v1`。
- [x] 在 `client.ts` 实现 `apiGet<T>(path: string)`。
- [x] 在 `client.ts` 实现 `apiPost<T>(path: string, body: unknown)`。
- [x] 在 `client.ts` 统一处理非 2xx 响应。
- [x] 创建 `LoginPage.tsx`，包含邮箱、密码和登录按钮。
- [x] 创建 `ArticleListPage.tsx`，用于后台文章列表。
- [x] 创建 `ArticleEditorPage.tsx`，包含标题、slug、摘要、标签、MDX 正文、保存草稿、发布按钮。
- [x] 将 `DashboardPage.tsx` 接入 API client。
- [x] 将 `CommentsPage.tsx` 接入评论审核 API 的页面结构。
- [x] 将 `AssetsPage.tsx` 接入上传和下载 API 的页面结构。
- [x] 在 `routes/index.tsx` 声明后台路由：`/admin/login`、`/admin`、`/admin/articles`、`/admin/articles/new`、`/admin/comments`、`/admin/assets`。
- [x] 在 `main.tsx` 挂载 React Router 和 TanStack Query。
- [x] 运行 `cd frontend && npm run build`。

## 验收标准

- 前端所有 CMS 管理页面都有路由入口。
- CMS 前端不提供公开文章列表和文章详情页面。
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
- 后台文章列表、文章编辑、统计、评论和资源页面均已接入 API client。
- `/` 默认跳转到后台仪表盘 `/admin`，后台页面由统一侧边栏壳层承载。
- CMS 前端已移除自身公开阅读路由；Rendering 博客项目负责公开文章展示。
- 2026-05-12 复核时，当前 WSL 环境中的 `npm` 解析到 Windows Node 路径，前端构建需要修复 WSL Node 工具链后重新执行。
