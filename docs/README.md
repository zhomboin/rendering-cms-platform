# 文档索引

本文档是 Rendering CMS Platform 的当前文档入口，用于区分权威文档、运维文档、历史归档和临时材料。

## 当前权威文档

- `docs/apis/`：后端接口契约，接口行为变更时必须同步更新。
- `docs/sql/schema.md`：数据库表、索引、隐私字段和迁移规则说明。
- `docs/operations/development-environment.md`：WSL2 Ubuntu 24.04 本地开发环境。
- `docs/operations/deployment.md`：生产 Docker 部署入口。
- `docs/operations/runbook.md`：生产固定运维 SOP。
- `docs/operations/production-access.md`：生产登录、后台用户、PostgreSQL 和 MinIO 访问 SOP。
- `docs/operations/backup.md`：PostgreSQL 和 MinIO 备份要求。
- `docs/operations/restore.md`：恢复流程。
- `docs/operations/observability.md`：日志与可观测性约定。
- `docs/operations/rendering-blog-publishing-integration.md`：Rendering 博客读取 CMS 已发布内容的对接方案。
- `docs/operations/rendering-blog-analytics-integration.md`：Rendering 博客访问统计上报对接方案。
- `docs/cms-platform-technical-recommendation.zh-CN.md`：项目架构边界和技术路线基线。
- `docs/guides/go-backend-guide.md`：Go 后端代码导读。
- `docs/guides/frontend-architecture.md`：前端目录结构与代码开发规范。
- `docs/guides/frontend-design.md`：后台管理界面设计规范。

## 历史归档

`docs/archive/` 只保存已完成或阶段性材料，不作为当前执行入口：

- `docs/archive/plans/`：已完成的 MVP 与 enhancement 实施计划。
- `docs/archive/spec/`：已完成的 MVP 阶段拆分文档。
- `docs/archive/reviews/`：已处理或阶段性的代码审查报告。

归档文档中的命令、状态、路径和待办可能反映当时上下文。需要执行当前任务时，以本文档“当前权威文档”中的入口为准。

## 已清理内容

- `docs/temp/login.html` 是登录页视觉改造的临时参考稿，已完成落地后移除。
- `backend/docs/` 已收敛到 `docs/guides/`，避免文档分散。
- `frontend/DESIGN.md` 和 `frontend/ARCHITECTURE.md` 已收敛到 `docs/guides/`，避免前端规则散落在模块目录。
