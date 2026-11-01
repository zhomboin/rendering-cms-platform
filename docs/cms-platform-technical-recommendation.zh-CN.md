# CMS 平台技术基线

本文件说明当前仓库的架构和能力，具体任务见 [当前任务清单](plans/current-tasks.md)，整体结果见 [项目进度](progress.md)。

## 项目边界

- 本仓库提供 CMS API 和后台管理平台，运行时业务数据统一存入 PostgreSQL。
- Rendering 博客负责公开展示，通过 CMS 公开接口读取已发布文章、已审核评论和已发布相册。
- 原博客的 MDX 文件仅作为导入来源和备份格式，导入工具位于 `backend/cmd/import-mdx/`；后端不写回原博客。
- CMS 前端是管理端，根路径进入后台，受登录保护；不提供公开博客阅读页面。

## 技术栈

| 范围 | 当前实现 | 依据 |
| --- | --- | --- |
| 后端 | Go 1.26.10、Chi、pgx/pgxpool、sqlc、golang-migrate | `backend/go.mod`、`backend/cmd/server/main.go`、`backend/sqlc.yaml` |
| 认证 | bcrypt、JWT access/refresh token、风险触发的邮箱验证码 | `backend/internal/auth/`、`backend/internal/mail/` |
| 前端 | React、TypeScript、Vite、React Router、TanStack Query、Ant Design | `frontend/package.json`、`frontend/src/routes/index.tsx` |
| 数据库 | PostgreSQL，SQL migration 统一管理 | `backend/migrations/`、`backend/sql/` |
| 对象存储 | S3 兼容接口；本地 MinIO，生产配置使用 Cloudflare R2 | `backend/internal/storage/`、`deploy/production.env.example` |
| 日志 | slog 结构化日志、按日文件和保留期限 | `backend/internal/logging/` |
| 部署 | Docker Compose、Nginx、systemd timer 自动部署脚本 | `deploy/` |

Go 工具链以模块和构建镜像中的固定版本为准。部署配置具备不等于生产已验收，具体环境状态须有验收证据。

## 文章与修订

- 文章保存标题、摘要、标签、MDX 正文、封面、精选字段、作者和发布时间。
- slug 为系统生成的 6 位短链；articleName 保存文章英文名。公开消费以 canonicalSlug 为准。
- 状态为草稿、已发布、私有和保留的归档值。草稿可发布或设为私有；已发布可下架为草稿；私有需转草稿后才能发布。
- 草稿与私有文章可直接编辑。公开接口只查询已发布文章；私有文章仅在登录后台编辑和阅读。
- 每次插入或更新由数据库触发器写入 article_logs；版本号随更新递增。后台支持修订列表、正文查看、对比和恢复。
- 发布仅允许草稿，每次发布重置发布时间。公开缓存及博客重建会影响下架后的可见性时延。

契约见 [文章 API](apis/articles.md)，数据结构与修订保留规则见 [数据库说明](sql/schema.md)。

## 相册与文件

- photo_albums 保存相册信息，album_photos 保存照片信息，assets 保存对象元数据；文件本体不写入 Git 或前端 public 目录。
- 相册只有草稿与已发布状态，发布至少需要一张照片；首次发布时间保持稳定。
- 照片经后端代理上传，仅接受 PNG、JPEG、WebP；服务端检查魔数、提取尺寸、剥离 EXIF/XMP 等元数据后存储。照片使用不可变对象 key 和公开长缓存 URL。
- 通用文件使用预签名上传与下载，下载记录审计；图片上限 50MB，其它允许文件上限 20MB，具体规则见 [文件 API](apis/assets.md)。
- 相册公开接口支持分页、ETag 和未发布 404；浏览上报接入统计。相册链接稳定性、失败清理、删除事务和并发排序等待修复项见 [任务清单](plans/current-tasks.md)。

相册契约见 [相册 API](apis/photos.md)，R2 前缀、公开域名和权限见 [部署说明](operations/deployment.md)。

## 统计与评论

首页支持 7／30／90 天访问趋势、文章与相册图文展示、评论和草稿待办及资源概况，见 [首页说明](guides/dashboard.md) 和 [首页 API](apis/dashboard.md)。

- 文章、相册和站点访问按日聚合；analytics_events 保存事件、日期、IP 哈希等去重数据，不保存原始 IP。
- 文章和站点历史日聚合由启动归档及每日定时任务搬迁，后台展示概览、趋势和热门文章。相册日聚合不包含在现有文章与站点归档任务中。
- 评论支持匿名提交、默认待审核、后台审核；公开接口只展示已审核评论，不做嵌套回复。
- 评论按 IP 哈希限制一分钟内最多三次提交，登录另有失败锁定与风险验证码。

契约见 [统计 API](apis/analytics.md) 和 [评论 API](apis/comments.md)。

## 搜索与公开接口保护

- CMS 搜索使用 PostgreSQL 的 tsvector、GIN 索引与全文查询，限定已发布文章、最多 100 个 Unicode 查询字符与 20 条结果。
- 公开列表只返回摘要，详情包含正文；成功响应支持缓存、ETag 与 304，404 短缓存，服务错误不缓存。
- 应用层提供速率、并发和客户端状态容量限制，Nginx 提供公网入口保护。
- CMS 不依赖构建期 Pagefind。Rendering 的页面与搜索更新需要在其仓库验证；构建期索引的发布联动未关闭前保留人工重建入口。

容量、真实代理与跨仓库验收见 [当前任务清单](plans/current-tasks.md)。

## 部署与安全

- 本地开发固定在 WSL2 Ubuntu 24.04，环境脚本位于 scripts/env，任务结束后清理临时开启的进程和容器。
- 生产配置位于服务器受限配置文件，不提交数据库密码、JWT 密钥、SMTP 授权码或 R2 凭据。
- 后台 API 必须认证。邮箱验证码由登录 IP 变化或连续失败触发，数据库只保存验证码哈希，生产禁止启用验证码日志。
- 生产 migration 前先备份 PostgreSQL；自动部署具备互斥锁、健康检查与代码回滚，数据库恢复需遵守恢复流程。

入口见 [开发环境](operations/development-environment.md)、[部署](operations/deployment.md)、[备份](operations/backup.md)、[恢复](operations/restore.md) 和 [运维速查](operations/maintenance.md)。
