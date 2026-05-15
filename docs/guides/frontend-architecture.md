# 前端目录结构与代码开发规范

本文档定义 `frontend/` 的 React + TypeScript 目录结构、模块边界和代码开发规则。后续前端重构、功能新增和代码评审都以本文档为准。

## 参考依据

- React 官方旧版 FAQ 说明 React 不强制唯一目录结构，但常见做法包括按 feature 或 route 分组，同时建议避免过深目录嵌套。
- React 官方新版文档说明组件是 UI 构建单元，大文件难以浏览时应拆出独立组件文件。
- React TypeScript 文档要求包含 JSX 的文件使用 `.tsx`，组件 props 应使用 `type` 或 `interface` 描述。
- Vite 静态资源文档区分源码内被引用的资源和 `public/` 中按原文件名服务的资源。
- TanStack Query 文档建议 query function 返回明确类型，query key 应唯一描述数据，并把变化变量纳入 query key。
- axios 官方文档支持通过 `axios.create` 创建实例，并用 request/response interceptors 统一处理请求和响应。

参考链接：

- https://legacy.reactjs.org/docs/faq-structure.html
- https://react.dev/learn/describing-the-ui
- https://react.dev/learn/typescript
- https://vite.dev/guide/assets
- https://tanstack.com/query/latest/docs/framework/react/guides/query-functions
- https://tanstack.com/query/latest/docs/framework/react/guides/query-keys
- https://tanstack.com/query/v5/docs/framework/react/typescript
- https://axios-http.com/docs/interceptors

## 目录结构

```text
frontend/
  src/
    api/
      client.ts
      auth-token.ts
      auth.ts
      articles.ts
      analytics.ts
      comments.ts
      assets.ts
    app/
      providers.tsx
      theme.ts
    layouts/
      AdminLayout.tsx
    pages/
      articles/ArticleEditorPage.tsx
      articles/ArticleListPage.tsx
      assets/AssetsPage.tsx
      auth/LoginPage.tsx
      comments/CommentsPage.tsx
      dashboard/DashboardPage.tsx
    routes/
      index.tsx
    main.tsx
    types/
      vite-env.d.ts
```

## 目录职责

- `src/main.tsx`：浏览器挂载入口，只负责创建 React root 并渲染应用。
- `src/app/`：应用级装配，包括 Provider、主题配置和全局上下文，不放业务页面。
- `src/routes/`：全局路由声明，只组合 layout 和 page，不写业务请求逻辑。
- `src/layouts/`：跨路由布局，例如后台侧边栏和顶部栏。
- `src/pages/`：路由级页面组件。页面按 URL 和用户任务分组，不直接放在 `features/`。
- `src/api/`：所有后端 API、axios 实例、拦截器、token 存取和接口类型。
- `src/components/`：跨业务复用 UI 组件。只有两个及以上页面复用时才放入该目录。

## 模块边界

- 页面可以导入 `src/api/`、`src/layouts/`、`src/components/` 和自身目录内的局部组件。
- 页面不得导入其他页面目录内的私有组件或工具。
- `src/api/client.ts` 只能被 `src/api/*.ts` 调用，页面不得直接导入。
- `src/app/` 可以导入 `src/routes/` 和全局 provider，不导入具体页面内部实现。
- `src/routes/` 可以导入 `src/pages/` 和 `src/layouts/`，不调用 API。

## API 规则

- axios 实例、base URL、超时、凭据、请求拦截器和响应拦截器统一放在 `src/api/client.ts`。
- 登录 token 存取统一放在 `src/api/auth-token.ts`。
- 各页面用到的具体 API 必须按领域放在 `src/api/` 下，例如 `articles.ts`、`comments.ts`、`assets.ts`。
- 页面和组件不得直接调用 `apiGet`、`apiPost`、`apiPatch`、`apiClient`、`fetch` 或 `axios` 方法。
- 资源预签名上传这类第三方直传也要封装在 `src/api/assets.ts`，页面只调用业务函数。

## TanStack Query 规则

- `queryFn` 优先引用 `src/api/` 中具有明确返回类型的业务函数。
- `queryKey` 必须是数组，并能唯一描述数据。
- 当 `queryFn` 依赖 `slug`、`id`、筛选条件等变量时，这些变量必须进入 `queryKey`。
- mutation 成功后只 invalidates 受影响的 query key，不做全局无差别刷新。

## 页面与组件规则

- 路由页面文件使用 `*Page.tsx` 命名。
- 包含 JSX 的文件使用 `.tsx`；不包含 JSX 的类型、工具、API 文件使用 `.ts`。
- 页面内部接口类型如果对应后端数据，放入 `src/api/` 对应领域文件；只服务于视图的小型 UI 状态类型可以留在页面内。
- 单个页面超过明显可读范围时，应优先把局部 UI 拆为同目录组件，而不是把页面塞入 `components/`。
- 跨页面复用组件移动到 `src/components/` 前，需要确认至少两个页面使用。

## 命名规则

- 组件和页面使用 PascalCase，例如 `AdminLayout.tsx`、`ArticleListPage.tsx`。
- API 函数使用动词 + 领域名，例如 `listAdminArticles`、`publishAdminArticle`、`uploadAdminAsset`。
- 类型使用领域语义命名，例如 `AdminArticleRecord`、`AnalyticsSummary`。
- 常量使用 camelCase；只有真正跨模块稳定复用的常量才导出。

## 资源规则

- 源码中引用的图片、字体、媒体资源应通过 import 或 `new URL(..., import.meta.url)` 进入 Vite 构建图。
- 必须保持原文件名、无需 import 的静态文件才放入 `public/`。
- 用户上传文件不得放入前端 `public/`，必须走对象存储和后端 API 元数据。

## 验证规则

前端结构调整后至少执行：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
npm run build
```

涉及依赖或 HTTP 客户端调整时同时执行：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
npm audit --omit=dev
```

结构规则检查：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform
grep -RIn -E '\bapi(Get|Post|Patch)\b|\bapiClient\b|\bfetch\s*[(]|\baxios\b' frontend/src/pages frontend/src/layouts
```
