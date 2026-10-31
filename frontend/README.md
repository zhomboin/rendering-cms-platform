# Rendering CMS Platform Frontend

`frontend/` 是 Rendering CMS Platform 的 React + TypeScript 前端应用，承担 CMS 后台管理界面。

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
    app/                      # 应用级 Provider 和主题配置
    layouts/                  # 跨路由布局
    pages/                    # 路由页面
      albums/
      articles/
      assets/
      auth/
      comments/
      dashboard/
    routes/                   # 路由集中声明
    types/                    # 全局类型声明
    main.tsx                  # 应用入口
  index.html
  package.json
  vite.config.ts
```

## 路由入口

当前路由集中在 `src/routes/index.tsx`：

- `/`：默认跳转到后台仪表盘 `/admin`。
- `/admin/login`：后台登录。
- `/admin`：后台仪表盘。
- `/admin/articles`：后台文章管理。
- `/admin/articles/new`：新增文章。
- `/admin/articles/:id/edit`：编辑文章。
- `/admin/comments`：评论审核。
- `/admin/assets`：资源管理。
- `/admin/albums`：相册管理。
- `/admin/albums/new`：新建相册。
- `/admin/albums/:id/edit`：相册基本信息与发布状态。
- `/admin/albums/:id/photos`：图片管理（上传进度、预览、URL 展示与复制、排序、封面、编辑与删除）。

后台页面使用 `src/layouts/AdminLayout.tsx` 作为统一壳层。访问 `/admin` 及其子路由时，如果本地没有登录 token，会跳转到 `/admin/login`；登录成功后回到原目标后台页面。

## API 配置

API client 位于 `src/api/client.ts`，使用 axios 实例统一封装请求。页面使用的具体接口按业务域放在 `src/api/` 下：

- `auth.ts`：后台登录。
- `articles.ts`：后台文章管理。
- `analytics.ts`：后台统计看板。
- `comments.ts`：后台评论审核。
- `assets.ts`：后台资源列表、预签名上传和下载。
- `albums.ts`：相册管理与照片后端代理上传。

页面和组件不得直接调用 `apiGet`、`apiPost`、`apiPatch`、`apiClient`、`fetch` 或 `axios` 方法，只能调用这些业务 API 文件暴露的类型化函数。

前端读取环境变量：

```env
VITE_API_BASE=http://127.0.0.1:8080/api/v1
```

如果未设置，默认使用：

```text
http://127.0.0.1:8080/api/v1
```

HTTP 请求规则：

- axios 请求拦截器会从本地登录 token 自动设置 `Authorization: Bearer <token>`。
- axios 响应拦截器遇到后台接口 `401` 时会清理本地 token，并跳转到 `/admin/login`。
- 资源预签名上传使用独立的 axios PUT 请求，不携带后台登录凭据。
- 相册图片使用带后台鉴权的 FormData 代理上传，清除 JSON 默认 Content-Type，由浏览器生成 multipart boundary。

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

该地址会跳转到后台仪表盘。

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

## 相册路由回归

相册路由回归入口为 scripts/test-album-routes.mjs，通过 npm run test:albums 执行。测试使用 Playwright Chromium；首次使用需通过 npx playwright install chromium 准备浏览器，并按 Playwright 要求准备 Ubuntu 运行库。

测试启动临时 Vite 服务与无头浏览器，结束时自动关闭。接口使用隔离响应，不连接生产；覆盖新建与失败重试、已有相册保存、两页独立加载和刷新、文件选择与拖拽的真实 multipart 内容、上传失败重试、类型和大小限制、URL 复制及失败提示，以及照片编辑、排序、封面和删除；上传用例受控触发 XHR 进度事件并延迟接口响应，覆盖逐文件百分比、等待处理、混合成功失败、未知总量、离页取消和重新进入；预览覆盖键盘、切换、缩放、关闭与封面行跳转隔离，并检查窄屏长文件名。

新建页没有相册 ID，不查询详情或照片，也不提供发布或上传操作；保存成功后使用创建接口返回的真实 ID 进入编辑页。生产验收须另行验证真实草稿的创建、读取及清理。

## 设计规范

后台管理界面设计规范见 `docs/guides/frontend-design.md`，前端目录结构与代码规则见 `docs/guides/frontend-architecture.md`。

实现时应优先保持：

- 左侧可折叠侧边栏 + 顶部 `80px` 导航 + 主内容区。
- 主色 `#4F46E5`。
- 页面背景 `#F8FAFC`、卡片背景 `#FFFFFF`、边框 `#E2E8F0`。
- 管理页面使用清晰、稳定、适合重复操作的信息布局。

## 验证要求

前端变更后至少运行：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
npm run build
```

涉及 API 对接时，应同时确认 `src/api/client.ts` 的路径、错误处理和认证头策略是否符合后端接口约定。


## 首页数据驾驶舱

后台首页默认展示近 30 天，可选择 7／30／90 天。概况、趋势、文章图文排行、相册表现、待办和资源摘要均来自后台只读接口；当前快照与区间统计明确区分，访问计数不是标准 PV 或 UV。完整说明见 [首页说明](../docs/guides/dashboard.md)，接口见 [首页 API](../docs/apis/dashboard.md)。

首页浏览器回归入口为 scripts/test-dashboard.mjs，通过 npm run test:dashboard 执行。用例在隔离响应下验证日期切换、空数据、分区失败与重试、刷新失败保留上次成功数据、图片失败、管理导航和移动端布局；运行结束自动关闭 Vite 与浏览器。测试依赖 Playwright Chromium 及 Ubuntu 运行库，不连接生产。

本地按影响范围运行用例和构建；[GitHub CI](../.github/workflows/verify.yml) 执行前端构建、MDX、相册与首页全部回归。
