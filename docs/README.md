# 文档索引

本文档是 Rendering CMS Platform 的当前文档入口，用于区分权威文档、运维文档、历史归档和临时材料。

CMS 相册新建与编辑的浏览器回归入口见 [前端说明](../frontend/README.md)，页面行为约定见 [相册 API](apis/photos.md)。

## 当前权威文档

- `docs/apis/`：后端接口契约，接口行为变更时必须同步更新。
- `docs/sql/schema.md`：数据库表、索引、隐私字段和迁移规则说明。
- `docs/operations/development-environment.md`：WSL2 Ubuntu 24.04 本地开发环境。
- `docs/operations/deployment.md`：生产 Docker 部署入口（含首次部署与自动部署启用）。
- `docs/operations/auto-deploy.md`：systemd timer 自动部署机制、失败回滚策略与手动回滚。
- `docs/operations/maintenance.md`：运维速查卡（定期检查清单、目录地图、手动部署、异常第一反应）。
- `docs/operations/runbook.md`：生产固定运维 SOP。
- [生产访问与安全规范](operations/production-access.md)：本地配置入口与隔离要求、固定操作边界，以及后台、PostgreSQL 和 Cloudflare R2 访问 SOP。
- 本机专用配置统一记录在 `docs/operations/local-config.local.md`，仅在主工作区本地保存，不随仓库分发。
- `docs/operations/backup.md`：PostgreSQL 备份与 Cloudflare R2 对象备份要求。
- `docs/operations/restore.md`：恢复流程。
- `docs/operations/observability.md`：日志与可观测性约定。
- `docs/operations/rendering-blog-publishing-integration.md`：Rendering 博客读取 CMS 已发布内容的对接方案。
- `docs/operations/rendering-blog-analytics-integration.md`：Rendering 博客访问统计上报对接方案。
- `docs/cms-platform-technical-recommendation.zh-CN.md`：项目架构边界和技术路线基线。
- `docs/guides/go-backend-guide.md`：Go 后端代码导读。
- `docs/guides/frontend-architecture.md`：前端目录结构与代码开发规范。
- `docs/guides/frontend-design.md`：后台管理界面设计规范。

## 进度与变更记录

- `docs/progress.md`：项目主进度记录，记录每次修改，Agent 定期维护；各专项进度通过该文档链接进入。
- `docs/change-log.md`：被推翻、废弃和清理内容的唯一记录文档；其它文档一律只保留最新版本，不书写历史版本标注。

## 执行中计划与专项沉淀

- `docs/plans/`：执行中或待生产验收的实施计划；全部完成并投产的计划归档到 `docs/archive/plans/`。
- `docs/web-tools-platform-analysis.zh-CN.md`：Web 工具平台独立项目的前期可行性分析（决策为不并入本仓库，文档作为可迁移的需求沉淀）。

## 历史归档

`docs/archive/` 只保存已完成或阶段性材料，不作为当前执行入口：

- `docs/archive/plans/`：已完成的 MVP 与 enhancement 实施计划。
- `docs/archive/spec/`：已完成的 MVP 阶段拆分文档。
- `docs/archive/reviews/`：已处理或阶段性的代码审查报告。

归档文档中的命令、状态、路径和待办可能反映当时上下文。需要执行当前任务时，以本文档“当前权威文档”中的入口为准。
