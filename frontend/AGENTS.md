# Frontend Agent Guide

本文件约束 `frontend/` 目录下的前端开发工作。根目录 `AGENTS.md` 的文档、环境、安全和项目边界约束仍然有效。

## 开发环境

- 所有前端命令默认在 WSL2 Ubuntu 24.04 中执行。
- 命令示例必须使用 Linux/Bash 形式，不使用 PowerShell、CMD 或 Windows 路径。
- 项目路径示例使用 `/home/ubuntu/workspace/rendering-cms-platform/frontend`。
- 后续执行依赖安装、构建、开发服务启动等项目命令时默认直接执行；破坏性操作、系统级配置修改、安装系统依赖或访问受限凭据除外。

## 技术边界

- 前端使用 React + TypeScript + Vite。
- 路由集中维护在 `src/routes/index.tsx`。
- API 请求入口集中维护在 `src/api/client.ts`。
- 后台壳层优先复用 `src/components/AdminLayout.tsx`。
- 后台界面组件优先使用 Ant Design，不随意引入新的 UI 组件库。
- 前端不保存运行时业务数据到 JSON 文件；文章、评论、统计、资产等数据来自后端 API。

## 目录职责

- `src/api/`：请求封装、API client、后续类型化服务入口。
- `src/components/`：跨页面复用组件和布局。
- `src/features/`：按业务域组织页面和局部组件。
- `src/routes/`：全局路由声明。
- `DESIGN.md`：后台界面设计规范。

## UI 规则

- 后台页面保持严肃、简洁、清晰，不做营销式 hero 页面。
- 优先构建可操作的管理界面，而不是说明型展示页。
- 页面布局遵循 `DESIGN.md` 中的侧边栏、顶部导航、内容区和 8px 间距体系。
- 主色使用 `#4F46E5`，避免大面积高饱和色块。
- 管理页面应优先支持扫描、筛选、编辑、审核和重复操作。
- 文案默认中文，按钮、表单、空状态和错误提示都应使用清晰中文。
- 组件内文本必须在常见桌面和移动宽度下不溢出、不重叠。

## API 与状态

- API base URL 使用 `VITE_API_BASE_URL`，默认 `http://127.0.0.1:8080/api/v1`。
- API 请求默认携带 `credentials: 'include'`。
- 服务端状态优先通过 TanStack Query 管理。
- 不要在页面组件中散落 `fetch` 调用；新增接口应先封装到 `src/api/` 或对应 feature 的服务文件。
- 未审核评论、未认证后台数据和敏感字段不得在公开页面展示。

## 编码规则

- TypeScript 保持严格类型，不使用无意义的 `any`。
- 页面级组件放在对应 `src/features/<domain>/` 下。
- 通用布局和跨业务组件放在 `src/components/`。
- 新增路由必须同步更新 `src/routes/index.tsx`。
- 新增环境变量必须以 `VITE_` 开头，并同步更新 README 或相关文档。
- 不提交 `dist/`、`node_modules/` 或本地缓存。

## 验证命令

前端常规验证：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
npm run build
```

需要本地预览时：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
npm run dev
```
