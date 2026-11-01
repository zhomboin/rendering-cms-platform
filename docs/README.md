# 文档索引

文档只维护当前契约、操作入口与未关闭事项。项目进度见 [主进度](progress.md)，变更摘要见 [变更记录](change-log.md)。

## 接口与架构

- [技术基线](cms-platform-technical-recommendation.zh-CN.md)：项目边界、技术栈与能力。
- [API 索引](apis/README.md)：认证、文章、首页、统计、评论、文件与相册契约。
- [数据库说明](sql/schema.md)：数据表、索引、隐私字段与迁移规则。
- [Go 后端导读](guides/go-backend-guide.md)、[前端架构](guides/frontend-architecture.md)、[前端设计](guides/frontend-design.md)。
- [首页说明](guides/dashboard.md)：模块、统计口径与验证入口。
- [文章插图](guides/article-images.md)：上传、链接、相册与资源选择，公开图片范围与验证入口。
- [文件](apis/assets.md)与[相册](apis/photos.md)：图片 50MB、其它文件 20MB 的上传边界及接口规则。

## 开发与运维

- [本地开发](operations/development-environment.md)。
- [代理配置检查](../scripts/env/check-nginx-config.sh)：在 CI 临时容器中校验 CMS 两层 Nginx 配置。
- [生产部署](operations/deployment.md)、[自动部署](operations/auto-deploy.md)、[运维速查](operations/maintenance.md)、[固定运维流程](operations/runbook.md)。
- [生产访问与安全](operations/production-access.md)：操作授权与配置隔离；本机专用配置仅保存在主工作区本地，不随仓库分发。
- [备份](operations/backup.md)、[恢复](operations/restore.md)、[日志与监控](operations/observability.md)。
- [博客发布对接](operations/rendering-blog-publishing-integration.md)、[博客统计对接](operations/rendering-blog-analytics-integration.md)。

## 任务与维护

- [当前任务](plans/current-tasks.md)：待修复问题、优先级、代码依据和环境验收条件。
- 进行中的实施计划放在 docs/plans，完成后删除执行详情；未关闭事项先迁入当前任务。
- 过程报告仅在本地 reports 保存，临时材料放在 temp；任务结束后清理本次产生的临时材料和进程。
- 文档使用中文 Markdown 和 UTF-8 无 BOM，链接使用项目相对路径。
