# Rendering CMS Platform Frontend

`frontend/` 是 Rendering CMS Platform 的 React + TypeScript 前端应用，承担公开文章阅读页面和后台管理界面。

当前前端以 Vite 构建，使用 React Router 管理路由，使用 TanStack Query 作为服务端状态入口，使用 Ant Design 构建后台管理界面。

## 技术栈

- React 19
- TypeScript
- Vite
- React Router
- TanStack Query
- Ant Design

## 目录结构

```text
frontend/
  src/
    api/                      # axios client、拦截器和类型化业务 API
    components/               # 通用布局和组件
    features/                 # 按业务域拆分页面与组件
      analytics/
      articles/
      assets/
      auth/
      comments/
    routes/                   # 路由集中声明
    main.tsx                  # 应用入口
  DESIGN.md                   # 后台管理界面设计规范
  index.html
  package.json
  vite.config.ts
```

## 路由入口

当前路由集中在 `src/routes/index.tsx`：

- `/`：默认跳转到后台仪表盘 `/admin`。
- `/articles`：公开文章列表。
- `/articles/:slug`：公开文章详情。
- `/admin/login`：后台登录。
- `/admin`：后台仪表盘。
- `/admin/articles`：后台文章管理。
- `/admin/articles/new`：新增文章。
- `/admin/articles/:id/edit`：编辑文章。
- `/admin/comments`：评论审核。
- `/admin/assets`：资源管理。

后台页面使用 `src/components/AdminLayout.tsx` 作为统一壳层。访问 `/admin` 及其子路由时，如果本地没有登录 token，会跳转到 `/admin/login`；登录成功后回到原目标后台页面。

## API 配置

API client 位于 `src/api/client.ts`，使用 axios 实例统一封装请求。页面使用的具体接口按业务域放在 `src/api/` 下：

- `auth.ts`：后台登录。
- `articles.ts`：公开文章、公开评论和后台文章管理。
- `analytics.ts`：后台统计看板。
- `comments.ts`：后台评论审核。
- `assets.ts`：后台资源列表、预签名上传和下载。

页面和组件不得直接调用 `apiGet`、`apiPost`、`apiPatch`、`apiClient`、`fetch` 或 `axios` 方法，只能调用这些业务 API 文件暴露的类型化函数。

前端读取环境变量：

```env
VITE_API_BASE=http://127.0.0.1:8080/api/v1
```

如果未设置，默认使用：

```text
http://127.0.0.1:8080/api/v1
```

所有 API 请求默认携带 `credentials: 'include'`，用于后续 Cookie Session 或安全登录态集成。

HTTP 请求规则：

- axios 请求拦截器会从本地登录 token 自动设置 `Authorization: Bearer <token>`。
- axios 响应拦截器遇到后台接口 `401` 时会清理本地 token，并跳转到 `/admin/login`。
- 资源预签名上传使用独立的 axios PUT 请求，不携带后台登录凭据。

## 本地开发

本项目固定在 WSL2 Ubuntu 24.04 中执行命令。

安装依赖：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
npm install
```

启动开发服务：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
npm run dev
```

默认访问：

```text
http://127.0.0.1:5173
```

该地址会进入后台仪表盘；公开文章列表入口为：

```text
http://127.0.0.1:5173/articles
```

构建：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
npm run build
```

预览生产构建：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
npm run preview
```

## 设计规范

后台管理界面设计规范见 `frontend/DESIGN.md`。

实现时应优先保持：

- 左侧 `240px` 侧边栏 + 顶部 `80px` 导航 + 主内容区。
- 主色 `#4F46E5`。
- 页面背景 `#F8FAFC`、卡片背景 `#FFFFFF`、边框 `#E2E8F0`。
- 管理页面使用清晰、稳定、适合重复操作的信息布局。

## 验证要求

前端变更后至少运行：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
npm run build
```

涉及 API 对接时，应同时确认 `src/api/client.ts` 的路径、错误处理和凭据策略是否符合后端接口约定。
