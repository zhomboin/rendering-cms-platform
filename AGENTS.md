# Rendering CMS Platform Agent Guide

## 文档规则

- 本项目新增和维护的文档必须使用 Markdown 格式，文件扩展名使用 `.md`。
- 文档语言默认使用中文，面向协作和交付的说明必须写成中文。
- 所有 Markdown 文档必须使用 UTF-8 编码保存。
- 文档路径必须使用项目相对路径，不写入本机绝对路径。
- 需求、接口、数据模型、部署、运维和阶段计划都应放在 `docs/` 目录下。
- API 文档放在 `docs/apis/`，SQL 与迁移说明放在 `docs/sql/`，部署和备份说明放在 `docs/operations/`。

## 本地开发环境约束

- 本项目本地开发环境固定为 WSL2 Ubuntu 24.04。
- 所有本地开发、依赖服务启动、后端测试、前端构建、数据库迁移和脚本验证命令都默认在 WSL Ubuntu 24.04 终端中执行。
- 文档、脚本和任务说明中的命令示例必须使用 Linux/Bash 命令格式，不使用 PowerShell、CMD、Windows 路径或 Windows 专用脚本作为默认入口。
- 项目路径示例应使用 WSL/Linux 风格，例如 `/home/ubuntu/workspace/rendering-cms-platform`；如需说明 Windows 访问路径，只能作为补充说明。
- 环境脚本统一放在 `scripts/env/`，优先使用 `.sh` Bash 脚本；不新增 `.ps1`、`.bat` 或 `.cmd` 开发脚本，除非用户明确要求。
- Docker 命令使用 Docker Engine 或 Docker Desktop WSL integration 下的 `docker` 与 `docker compose`，不要使用旧版 `docker-compose` 作为默认命令。
- Node.js、Go、PostgreSQL、sqlc、migrate 等工具的安装、检查和运行说明都以 Ubuntu 24.04 包管理器或 Linux 官方安装方式为准。
- 后续需要在 WSL Ubuntu 24.04 中执行项目相关命令时，默认直接执行；除非命令具有破坏性、会修改系统级配置、需要安装系统依赖或访问受限凭据，否则不要再次向用户确认。

## 项目边界

- 本仓库是新的 `rendering-cms-platform` 项目，用于承载个人技术博客的 CMS 化能力。
- 不要把统计、评论、上传下载、后台编辑发布等动态能力直接叠加到原静态博客仓库。
- 原 `Rendering` 静态博客只作为内容迁移来源、视觉参考和回退基线。
- 当前静态博客中的 `content/posts/*.mdx` 只作为导入来源和备份格式，不作为新平台的运行时数据源。
- 不要删除、重写或破坏原静态博客内容；需要迁移时应通过导入工具进入 PostgreSQL。

## 架构约束

- 后端使用 Go 1.22+，优先采用 `Chi + pgx + sqlc + golang-migrate`。
- 前端使用 React + TypeScript + Vite。
- 数据库使用 PostgreSQL，业务数据以 SQL migration 管理。
- 文件上传下载使用 S3 兼容对象存储，例如 Cloudflare R2、AWS S3 或 MinIO。
- 上传文件只在数据库保存元数据和 object key，不写入 Git，也不写入前端 `public/`。
- 后台登录态可使用 JWT 或安全 Cookie Session，密码必须使用 bcrypt 哈希。
- 后端 API 统一使用 `/api/v1` 前缀。

## 推荐目录结构

```text
rendering-cms-platform/
  backend/
    cmd/server/
    internal/auth/
    internal/articles/
    internal/comments/
    internal/analytics/
    internal/assets/
    internal/config/
    internal/database/
    internal/http/
    internal/storage/
    migrations/
    sql/
  frontend/
    src/api/
    src/routes/
    src/pages/
    src/components/
    src/features/articles/
    src/features/comments/
    src/features/analytics/
    src/features/assets/
    src/features/auth/
  docs/
    apis/
    sql/
    operations/
```

## 功能范围

- 文章管理：支持草稿、发布、归档、MDX 正文、标签、摘要、封面和修订历史。
- 访问统计：第一版采用日聚合，记录文章访问量和站点访问量，并在后台展示今日、近 7 天和热门文章。
- 评论：第一版支持匿名提交、默认待审核、后台审核，不做嵌套评论。
- 文件：支持后台申请预签名上传 URL、直传对象存储、申请预签名下载 URL，并记录下载审计。
- 后台：需要登录保护，覆盖仪表盘、文章编辑发布、评论审核、资源上传下载。
- 公开站点：只展示已发布文章和已审核评论。

## 安全与数据规则

- 不得暴露未认证的后台 API。
- 不得公开未审核评论。
- 不得保存原始 IP 地址；如需风控或去重，只保存哈希。
- 文件上传必须校验文件名、内容类型和大小。
- 第一版允许上传类型为 `image/png`、`image/jpeg`、`image/webp`、`application/pdf`、`text/plain`、`application/zip`。
- 第一版上传大小上限为 `20MB`。
- 发布文章和保存草稿时必须写入 `article_revisions`。
- 生产 migration 前必须先备份 PostgreSQL。

## 实施顺序

1. 先维护并细化 `docs/cms-platform-technical-recommendation.zh-CN.md` 和 `docs/plans/2026-04-29-rendering-cms-platform-phase.md`。
2. 搭建 Go 后端骨架、配置、健康检查和数据库连接。
3. 建立 SQL migration、`sqlc` 查询和核心数据表。
4. 搭建 React + TypeScript 前端骨架和后台壳层。
5. 实现登录、鉴权和管理员初始化。
6. 编写 MDX 导入工具，迁移现有文章。
7. 实现公开文章读取和后台文章编辑发布。
8. 实现文章统计和后台看板。
9. 实现评论提交与审核。
10. 实现文件上传、下载和审计。
11. 补齐部署、备份、接口和运维文档。

## 验证要求

- Go 后端变更后运行 `go test ./...`，必要时运行 `go vet ./...`。
- React 前端变更后运行 `npm run build`。
- 数据库 schema 变更必须提供 migration，并尽量补充查询或服务层测试。
- 接口行为变更必须同步更新 `docs/apis/` 下的中文 Markdown 文档。
- 阶段任务完成后更新对应计划文档中的复选框和状态说明。

## 禁止事项

- 不要用 JSON 文件保存评论、统计或文章运行时数据。
- 不要让后端直接写入原静态博客仓库的 `content/posts`。
- 不要把上传文件提交进 Git。
- 不要继续依赖构建期 Pagefind 承担 CMS 动态搜索。
- 不要新增英文优先的项目文档，除非用户明确要求双语或英文版本。
