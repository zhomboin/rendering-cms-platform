# Rendering CMS Platform

`rendering-cms-platform` 是面向个人技术博客 CMS 化能力的新项目，用于承载文章管理、访问统计、评论审核、文件上传下载和后台发布等动态功能。

本项目与原静态博客仓库保持边界清晰：原 `Rendering` 项目只作为内容迁移来源、视觉参考和回退基线，不继续承载后台、评论、统计或上传下载等运行时能力。

## 项目目标

- 提供后台文章编辑、草稿、发布、归档和修订历史能力。
- 将原静态博客的 `content/posts/*.mdx` 作为导入来源，进入 PostgreSQL 管理。
- 统计文章访问量和站点访问量，并在后台仪表盘展示。
- 支持匿名评论提交、默认待审核和后台审核。
- 支持文件上传、下载、元数据管理和下载审计。
- 提供 Rendering 博客读取已发布文章、已审核评论和上报访问统计的接口。

## 技术架构

后端：

- Go 1.22+
- Chi
- pgx
- sqlc
- golang-migrate
- PostgreSQL

前端：

- React
- TypeScript
- Vite

对象存储：

- S3 兼容对象存储
- 本地开发使用 MinIO
- 生产环境可使用 Cloudflare R2、AWS S3 或同类服务

## 本地开发环境

本项目本地开发环境固定为 WSL2 Ubuntu 24.04。所有开发、构建、测试、数据库迁移和依赖服务启动命令默认在 WSL Ubuntu 24.04 终端中执行。

基础约束：

- 使用 Linux/Bash 命令。
- 使用 WSL/Linux 路径，例如 `/home/ubuntu/workspace/rendering-cms-platform`。
- 使用 `docker compose`，不以旧版 `docker-compose` 作为默认命令。
- 环境脚本放在 `scripts/env/`，默认使用 `.sh` Bash 脚本。

环境检查：

```bash
bash scripts/env/check-env.sh
```

复制本地环境变量：

```bash
cp scripts/env/dev.env.example .env
```

启动 PostgreSQL 和 MinIO：

```bash
bash scripts/env/start-prerequisites.sh
```

启动后端 Docker 服务：

```bash
bash scripts/env/start-backend-docker.sh
```

启动完整开发栈：

```bash
bash scripts/env/start-dev-stack.sh
```

完整开发栈会启动 PostgreSQL、MinIO、Go 后端和 Vite 前端。

停止开发服务：

```bash
bash scripts/env/stop-dev-services.sh
```

更完整的环境说明见 `docs/operations/development-environment.md`。

## 文档入口

- `AGENTS.md`：Agent 协作规则、技术约束和本地环境约束。
- `docs/cms-platform-technical-recommendation.zh-CN.md`：CMS 平台技术架构建议。
- `docs/plans/2026-04-29-rendering-cms-platform-mvp.md`：MVP 版本迭代计划。
- `docs/plans/2026-04-29-rendering-cms-platform-enhancements.md`：后续增强功能计划。
- `docs/operations/development-environment.md`：WSL2 Ubuntu 24.04 开发环境配置指南。
- `docs/operations/deployment.md`：生产 Docker 部署方案。
- `docs/operations/runbook.md`：生产运维 SOP。
- `docs/spec/`：MVP 各阶段子任务文档。

## MVP 子任务

当前 MVP 拆分为以下阶段：

1. 项目骨架与基础工程。
2. 数据库 schema 与 migration。
3. 管理员认证与后台访问控制。
4. 文章管理与 MDX 导入。
5. 访问统计与后台仪表盘。
6. 评论提交与审核。
7. 文件上传下载。
8. 前端后台壳层与路由。
9. 运维、验证与交付检查。

各阶段详细任务位于 `docs/spec/`。

## 推荐目录结构

```text
rendering-cms-platform/
  backend/
    cmd/server/
    internal/
    migrations/
    sql/
  frontend/
    src/
  docs/
    apis/
    operations/
    plans/
    spec/
    sql/
  scripts/
    env/
```

## 验证要求

后端变更后运行：

```bash
go test ./...
```

必要时运行：

```bash
go vet ./...
```

前端变更后运行：

```bash
npm run build
```

数据库结构变更必须提供 migration；接口行为变更必须同步更新 `docs/apis/` 下的中文 Markdown 文档。

## 当前状态

当前仓库已完成 MVP 主要工程骨架和功能代码落地，`docs/spec/` 下 9 个 MVP 阶段文档均已标记完成，覆盖后端 API、数据库 migration、认证、文章、统计、评论、文件、前端页面和基础运维文档。

当前实现与文档同步口径如下：

- MVP 主体代码已具备：Go 后端、React + TypeScript 前端、PostgreSQL migration、`sqlc` 查询、API 文档和 WSL2 Ubuntu 24.04 本地环境脚本。
- CMS 前端是纯管理平台，`/` 默认跳转到后台仪表盘 `/admin`；文章展示由另一个 `Rendering` 博客项目读取 CMS 已发布内容后完成。
- `docs/plans/2026-04-29-rendering-cms-platform-enhancements.md` 是 MVP 后增强计划，其中编辑器体验、搜索、评论限流、统计趋势、文件治理、角色权限、备份恢复等任务仍未完成。
- 2026-05-12 当前环境复核时，WSL 中缺少可用的 Linux Go 工具链，`npm` 解析到 Windows Node 路径，后端测试、vet 和前端构建需要在修复 WSL 工具链后重新执行。
