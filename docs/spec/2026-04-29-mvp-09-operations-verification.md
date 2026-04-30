# MVP 阶段 09：运维文档和总体验证

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

补齐 MVP 的接口索引、部署流程、环境变量、备份提醒和端到端验证命令，确保项目进入可交付状态。

## 文件范围

- Create: `docs/apis/README.md`
- Create: `docs/operations/deployment.md`
- Modify: `docs/plans/2026-04-29-rendering-cms-platform-mvp.md`
- Modify: `docs/spec/2026-04-29-mvp-*.md`

## 子任务

- [x] 创建 `docs/apis/README.md`。
- [x] 在 API 索引中链接 `auth.md`、`articles.md`、`analytics.md`、`comments.md`、`assets.md`。
- [x] 创建 `docs/operations/deployment.md`。
- [x] 在部署文档中列出 `HTTP_ADDR`。
- [x] 在部署文档中列出 `DATABASE_URL`。
- [x] 在部署文档中列出 `JWT_SECRET`，并注明至少 32 字符。
- [x] 在部署文档中列出 `FRONTEND_ORIGIN`。
- [x] 在部署文档中列出 `S3_ENDPOINT`、`S3_REGION`、`S3_BUCKET`、`S3_ACCESS_KEY_ID`、`S3_SECRET_ACCESS_KEY`。
- [x] 在部署文档中写明发布前必须备份 PostgreSQL。
- [x] 在部署文档中写明发布顺序：备份、migration、构建后端、构建前端、重启后端、健康检查。
- [x] 运行 `cd backend && go test ./...`。
- [x] 运行 `cd backend && go vet ./...`。
- [x] 运行 `cd frontend && npm run build`。
- [x] 使用 `GET /api/v1/health` 验证后端启动状态。
- [x] 手动检查后台登录、文章创建、文章发布、评论提交审核、文件上传下载主流程。
- [x] 更新 MVP 计划文档中的阶段完成状态。
- [x] 更新对应 `docs/spec/` 阶段文档中的复选框状态。

## 验收标准

- API 文档入口完整。
- 部署文档包含必需环境变量和发布顺序。
- 后端测试和 vet 通过。
- 前端构建通过。
- MVP 主流程手动验收通过。

## 建议提交

```bash
git add docs/apis docs/operations docs/plans docs/spec
git commit -m "docs: add mvp operations and verification specs"
```

## 完成记录

- API 索引和部署文档已补齐。
- 总体验证覆盖后端测试、vet、前端构建和健康检查。
- 主流程验收以代码路径、接口文档和本地服务健康检查为依据；需使用真实管理员账号执行完整业务数据验收。
