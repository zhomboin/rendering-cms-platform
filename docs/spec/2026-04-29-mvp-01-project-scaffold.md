# MVP 阶段 01：项目骨架和基础命令

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

建立 Go 后端和 React + TypeScript 前端的最小可运行骨架，为后续数据库、认证、文章、统计、评论和文件模块提供统一目录与基础命令。

## 文件范围

- Create: `backend/go.mod`
- Create: `backend/cmd/server/main.go`
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/http/router.go`
- Create: `frontend/package.json`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/routes/index.tsx`

## 子任务

- [x] 创建后端目录：`backend/cmd/server`、`backend/internal/config`、`backend/internal/http`。
- [x] 在 `backend/` 下执行 `go mod init rendering-cms-platform/backend`。
- [x] 安装后端基础依赖：`chi`、`pgxpool`、`jwt`、`bcrypt`、`uuid`、`aws-sdk-go-v2`。
- [x] 在 `backend/internal/config/config.go` 实现 `Config` 结构和 `Load()`，读取 `HTTP_ADDR`、`DATABASE_URL`、`JWT_SECRET`、`FRONTEND_ORIGIN`、S3 配置。
- [x] 在 `backend/internal/http/router.go` 实现 `NewRouter()`。
- [x] 添加 `GET /api/v1/health`，返回 `{ "status": "ok" }`。
- [x] 在 `backend/cmd/server/main.go` 启动 HTTP 服务，默认监听 `:8080`。
- [x] 创建前端 Vite React TypeScript 项目。
- [x] 安装前端基础依赖：`react-router-dom`、`@tanstack/react-query`、`antd`。
- [x] 创建 `frontend/src/routes/index.tsx`，集中声明后续页面路由入口。
- [x] 运行 `cd backend && go test ./...`。
- [x] 运行 `cd frontend && npm run build`。

## 完成记录

- 完成时间：2026-04-30。
- 后端已提供 `GET /api/v1/health`，返回 `{ "status": "ok" }`。
- 后端已补充 `config.Load()` 和路由单元测试。
- 前端已完成 Vite + React + TypeScript 最小后台壳层。
- 验证命令已通过：`cd backend && go test ./...`、`cd frontend && npm run build`。

## 验收标准

- `GET /api/v1/health` 的路由已存在。
- 后端可以通过 `go test ./...` 编译。
- 前端可以通过 `npm run build` 构建。
- 目录结构与 `AGENTS.md` 中的项目边界一致。

## 建议提交

```bash
git add backend frontend
git commit -m "feat: scaffold cms platform"
```
