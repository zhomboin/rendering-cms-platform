# Rendering CMS Platform

`rendering-cms-platform` 提供文章发布、访问统计、相册与文件管理，并接入 PostBridge 草稿分发。

CMS 负责管理数据及提供公开接口。Rendering 博客负责公开页面、访问上报和搜索索引。

## 项目目标

- 提供后台文章编辑、草稿、发布、下架、私有阅读、修订和管理员删除能力，并支持[从相册和资源选择插图](docs/guides/article-images.md)。
- 将原静态博客的 `content/posts/*.mdx` 作为导入来源，进入 PostgreSQL 管理。
- 首页数据驾驶舱展示 7／30／90 天访问趋势、文章图文排行、相册表现、草稿待办、内容和资源概况，详见 [首页说明](docs/guides/dashboard.md)。
- 支持文件上传、下载、元数据管理和下载审计。
- 支持相册管理与公开查询，照片由后端提取尺寸并剥离隐私元数据。
- 提供 Rendering 博客读取已发布文章和相册、上报访问统计的接口。

## 技术架构

后端：

- Go 1.26.11
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
- 生产环境使用 Cloudflare R2（公开访问域名 `assets.rendering.me`）

## 本地开发

在 WSL2 Ubuntu 24.04 中按 [本地开发指南](docs/operations/development-environment.md#推荐开发启动顺序)检查工具、配置本地环境、启动依赖并执行数据库迁移。随后在仓库根目录启动和检查应用。

```bash
bash scripts/env/start-dev-stack.sh
curl -fsS http://127.0.0.1:8080/api/v1/health
```

打开 `http://127.0.0.1:5173`。结束本次开发时运行 `bash scripts/env/stop-dev-services.sh` 清理启动的 Compose 服务。生产操作见 [部署手册](docs/operations/deployment.md)。

## 文档入口

- `docs/README.md`：当前文档索引，说明权威文档、任务与进度入口。
- [项目主进度](docs/progress.md)：当前功能、CI 结果和环境验收。
- `AGENTS.md`：Agent 协作规则、技术约束和本地环境约束。
- `docs/cms-platform-technical-recommendation.zh-CN.md`：CMS 平台技术架构建议。
- `docs/apis/`：当前 API 契约。
- `docs/sql/schema.md`：当前数据库结构说明。
- `docs/operations/development-environment.md`：WSL2 Ubuntu 24.04 开发环境配置指南。
- `docs/operations/deployment.md`：生产 Docker 部署方案。
- `docs/operations/runbook.md`：生产运维 SOP。
- [当前任务](docs/plans/current-tasks.md)：环境验收、部署收尾与维护事项。

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
    plans/
    apis/
    guides/
    operations/
    sql/
  scripts/
    env/
```

## 验证要求

本地按本次改动影响范围执行相关测试与必要静态检查，前端变更运行生产构建。测试覆盖正常、边界和异常路径，删除功能时同步清理失效用例。

[GitHub CI](.github/workflows/verify.yml)执行全量 Go 测试、静态检查、SQL 生成一致性、PostgreSQL 集成验证、前端构建和全部前端回归。

数据库结构变更必须提供 migration；接口行为变更同步更新 docs/apis 下的中文 Markdown 文档。

## 当前状态

CMS 根路径进入后台仪表盘，公开阅读由 Rendering 博客负责。完整能力见 [技术基线](docs/cms-platform-technical-recommendation.zh-CN.md)。

代码实现、本地验证和生产验收分别记录。当前待办统一见 [任务清单](docs/plans/current-tasks.md)，整体状态见 [项目进度](docs/progress.md)。

正文预览、表格与目录支持见 [Markdown 规则](docs/guides/markdown.md)。
