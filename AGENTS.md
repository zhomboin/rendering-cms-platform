# Rendering CMS Platform Agent Guide

## 文档规则

- 本项目新增和维护的文档必须使用 Markdown 格式，文件扩展名使用 `.md`。
- 文档语言默认使用中文，面向协作和交付的说明必须写成中文。
- 所有项目文件（文档、源代码、配置、脚本）必须使用 UTF-8 无 BOM 编码保存。
- 项目内文档链接使用项目相对路径；本机绝对路径和生产连接元数据只记录在不提交的本地配置文档，不写入仓库文档。
- 需求、接口、数据模型、部署、运维和阶段计划都应放在 `docs/` 目录下。
- API 文档放在 `docs/apis/`，SQL 与迁移说明放在 `docs/sql/`，部署和备份说明放在 `docs/operations/`。
- 除仓库根目录与各模块的 `README.md`、`AGENTS.md` 外，正式文档只放在 `docs/` 目录下；各类过程报告放在 `reports/` 目录下；所有临时文件和临时代码放在 `temp/` 目录下。
- `reports/` 和 `temp/` 已加入 `.gitignore`，只在本地存在，不提交进 Git，也不在仓库中维护。
- 整个项目只维护一份主进度记录文档 `docs/progress.md`，用于记录每次修改，Agent 必须在每次变更后定期维护更新；各专项进度独立成文（如 `docs/plans/` 下的阶段计划），从主进度文档通过链接进入，不在主进度文档内展开。
- 每次用户提出的后续注意事项和新增约束，都必须整合进本文件和 `docs/` 下的对应权威文档，保证规范始终只有最新版本。

## Git 工作流与变更管理

- 项目使用 Git 管理，主分支为 `main`。
- 新功能开发不得直接在 `main` 分支上进行；必须从 `main` 拉取独立功能分支开发，完成后通过 Pull Request 合入 `main`。
- 每次 PR 合并前必须完成一次代码检视（code review），检视通过后才能合并。
- 生产诊断默认只读；所有未列入运维文档的服务器写命令必须先获得用户确认。用户已明确批准的修复按批准范围执行，不得扩大到其它配置、数据或备份策略。
- 生产安全边界统一见 `docs/operations/production-access.md`；本机专用配置统一记录在主工作区的 `docs/operations/local-config.local.md`，该文件必须排除 Git 跟踪及 Docker 打包。生产 IP、SSH 端口、登录用户、主机指纹、私钥路径与其它机器专属配置不得进入提交、PR 或生产部署。
- 连接权限、高权限账号和网关已登记操作均不等于生产写入授权；部署、重启、定时器变更等写操作必须属于用户明确授权的任务范围。
- 禁止执行未定义的高级操作。新增操作必须先在运维文档定义目标、参数、影响、前置检查、备份或回退及验收条件，并获得用户明确批准；走网关时还须登记为固定操作。不得通过直接 SSH、任意 Shell、修改白名单或关闭安全检查绕过限制。
- 每个功能完成后，必须检查所有文档（含 `docs/README.md` 索引、`docs/progress.md` 主进度记录和相关权威文档）是否与最新进度和要求一致，不一致处当场同步更新。
- 每次被推翻、废弃或清理的项目内容，统一记录到独立变更记录文档 `docs/change-log.md`；除该文档外，其它所有文档只保留最新版本的内容，严禁出现任何历史版本标注（例如"在某版本为 xxx，后修订为 xxx"这类表述一律禁止）。

## 本地开发环境约束

- 本项目本地开发环境固定为 WSL2 Ubuntu 24.04。
- 所有本地开发、依赖服务启动、后端测试、前端构建、数据库迁移和脚本验证命令都默认在 WSL Ubuntu 24.04 终端中执行。
- 文档、脚本和任务说明中的命令示例必须使用 Linux/Bash 命令格式，不使用 PowerShell、CMD、Windows 路径或 Windows 专用脚本作为默认入口。
- 项目路径示例应使用 WSL/Linux 风格，例如 `/home/ubuntu/workspace/rendering-cms-platform`；如需说明 Windows 访问路径，只能作为补充说明。
- 环境脚本统一放在 `scripts/env/`，优先使用 `.sh` Bash 脚本；不新增 `.ps1`、`.bat` 或 `.cmd` 开发脚本，除非用户明确要求。
- 生产 SSH 使用 Windows 本地私钥时，允许从 WSL 调用 Windows OpenSSH；具体程序路径与连接参数只记录在本地配置文档。不得为连接而把私钥复制进仓库或放宽私钥权限。
- Docker 命令使用 Docker Engine 或 Docker Desktop WSL integration 下的 `docker` 与 `docker compose`，不要使用旧版 `docker-compose` 作为默认命令。
- Node.js、Go、PostgreSQL、sqlc、migrate 等工具的安装、检查和运行说明都以 Ubuntu 24.04 包管理器或 Linux 官方安装方式为准。
- 后续需要在 WSL Ubuntu 24.04 中执行项目相关命令时，默认直接执行；除非命令具有破坏性、会修改系统级配置、需要安装系统依赖或访问受限凭据，否则不要再次向用户确认。
- 任务执行过程中临时启动的所有进程（本地服务、前后端 dev 服务、测试或联调辅助进程、临时 Docker 容器等）必须在任务完成后全部结束，不得残留任何进程。

## 项目边界

- 本仓库是新的 `rendering-cms-platform` 项目，用于承载个人技术博客的 CMS 化能力。
- 不要把统计、评论、上传下载、后台编辑发布等动态能力直接叠加到原静态博客仓库。
- 原 `Rendering` 静态博客只作为内容迁移来源、视觉参考和回退基线。
- 当前静态博客中的 `content/posts/*.mdx` 只作为导入来源和备份格式，不作为新平台的运行时数据源。
- 不要删除、重写或破坏原静态博客内容；需要迁移时应通过导入工具进入 PostgreSQL。

- 跨仓库故障在所属仓库修复；仅授权 CMS 修复时，不修改 Rendering 博客源码，也不触发博客部署。消费端问题通过对应运维文档转交博客项目。

## 架构约束

- 后端使用 Go 1.22+，优先采用 `Chi + pgx + sqlc + golang-migrate`。
- 前端使用 React + TypeScript + Vite。
- 后台展示遵循前端设计规范：标题、字段与提示保持简洁，不拼接重复业务名称或堆叠技术说明；上传展示逐文件真实进度与处理结果，照片缩略图及相册封面支持放大预览。
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
  reports/
  temp/
```

## 功能范围

- 文章管理：支持草稿、发布、归档、MDX 正文、标签、摘要、封面和修订历史。
- 私有文章：支持将草稿设为私有（日常记录、运维文档等），仅在登录后台可见、可编辑、可通过后台阅读页阅读，绝不进入公开站点；公开接口只返回 `published` 文章。
- 相册：后台维护相册与照片，照片由后端代理上传（魔数校验、像素尺寸服务端提取、存储前剥离 EXIF/XMP 等元数据，含 GPS），相册仅 `published` 状态对外可见；向 Rendering 博客提供相册分页列表、相册元数据、照片分页清单的公开查询接口与相册浏览上报，契约见 `docs/apis/photos.md`。
- 访问统计：采用每日去重计数，首页以浅色数据驾驶舱展示 7／30／90 天趋势、文章排行、相册、内容概况和待办；默认 30 天。不得把现有计数标为标准 PV 或 UV，不显示没有采集依据的增长率和实时指标。
- 评论：第一版支持匿名提交、默认待审核、后台审核，不做嵌套评论。
- 文件：支持后台申请预签名上传 URL、直传对象存储、申请预签名下载 URL，并记录下载审计。
- 后台：需要登录保护，覆盖仪表盘、文章编辑发布、相册管理、评论审核、资源上传下载。相册基本信息与图片管理使用独立页面；图片管理必须展示可复制的完整图片 URL。
- 公开站点：只展示已发布文章和已审核评论。

## 安全与数据规则

- 不得暴露未认证的后台 API。
- 不得公开未审核评论。
- 不得保存原始 IP 地址；如需风控或去重，只保存哈希。
- 后台登录在密码验证成功后按风险判定（登录 IP 变更或连续失败 3 次）触发邮箱验证码两步校验；验证码数据库只存哈希，校验失败响应不区分错误类型，生产环境不得开启 `LOGIN_MFA_DEV_LOG`。
- 文件上传必须校验文件名、内容类型和大小。
- 第一版允许上传类型为 `image/png`、`image/jpeg`、`image/webp`、`application/pdf`、`text/plain`、`application/zip`。
- 第一版上传大小上限为 `20MB`。
- 发布文章和保存草稿时必须写入 `article_revisions`。
- 生产 migration 前必须先备份 PostgreSQL。

## 实施顺序

1. 先维护并细化 `docs/cms-platform-technical-recommendation.zh-CN.md`；当前执行中的新阶段计划应放在 `docs/plans/`，已完成计划归档到 `docs/archive/plans/`。
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

- 本地只运行本次改动影响的测试与必要静态检查；新增行为覆盖正常、边界和异常路径。全量 Go 测试、查询生成检查、前端构建和全部前端回归由 GitHub CI 执行，入口为 `.github/workflows/verify.yml`。
- React 前端变更后运行 `npm run build`。
- 数据库 schema 变更必须提供 migration，并尽量补充查询或服务层测试。
- 接口行为变更必须同步更新 `docs/apis/` 下的中文 Markdown 文档。
- 阶段任务完成后更新对应计划文档中的复选框和状态说明。
- 每个功能完成后，按"Git 工作流与变更管理"章节的文档一致性要求核对，并确认未在任何文档中引入历史版本标注。

## 禁止事项

- 不要用 JSON 文件保存评论、统计或文章运行时数据。
- 不要让后端直接写入原静态博客仓库的 `content/posts`。
- 不要把上传文件提交进 Git。
- 不要把 `reports/` 和 `temp/` 目录下的任何内容提交进 Git。
- 不要直接在 `main` 分支上开发新功能，或绕过 PR 代码检视合入代码。
- 不要在 `docs/change-log.md` 之外的任何文档中记录历史版本、变更过程或"某版本为 xxx、后修订为 xxx"类标注。
- 不要继续依赖构建期 Pagefind 承担 CMS 动态搜索。
- 不要新增英文优先的项目文档，除非用户明确要求双语或英文版本。
