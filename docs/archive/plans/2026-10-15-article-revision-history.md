# 文章修订历史可视化实施计划

**目标：** 让 `article_logs` 从“只写不读”变为可用：后台可查看文章版本列表、把历史版本与当前内容做差异对比、并把历史版本载入草稿后再次编辑发布。

**设计决策：**

- 恢复不做新写接口：恢复 = 把历史快照字段载入编辑器表单（同时写入本地草稿缓存），用户确认后走现有 `PATCH /admin/articles/{id}` 保存草稿，触发器自动 version+1 并写入新修订快照，全程可审计。
- 恢复仅 `draft` 状态可用；已发布/已归档文章的恢复按钮禁用并提示先下架。
- 行级 diff 自研 LCS 纯函数（`revisionDiff.ts`），不引入第三方 diff 库。
- 版本列表不返回 `bodyMdx`，单版本详情返回完整快照。

**技术栈：** Go、Chi、pgx/sqlc、PostgreSQL、React + TypeScript + Vite + Ant Design。

## 执行状态（2026-10-15）

- [x] `ListArticleRevisions` / `GetArticleRevision` sqlc 查询已实现并生成代码；无 migration。
- [x] `GET /api/v1/admin/articles/{id}/revisions` 版本摘要列表（version 倒序、无 bodyMdx）。
- [x] `GET /api/v1/admin/articles/{id}/revisions/{version}` 完整快照；version 非法 400、不存在 404。
- [x] 后端测试：列表瘦身与排序、详情含 bodyMdx、404、非法参数 400 且不访问数据库。
- [x] 前端 `listArticleRevisions` / `getArticleRevision` API 封装；`AdminArticleRecord` 补充 `version` 可选字段。
- [x] `revisionDiff.ts` 行级 LCS diff 纯函数（零新依赖）。
- [x] 编辑器顶部「历史版本」入口 + 右侧 Drawer：版本列表（当前版本标记）、元数据字段差异（仅显示变化字段）、正文行级 diff（绿=历史版本、红=当前）。
- [x] 「恢复此版本」：确认弹窗 → 载入表单 + 写入本地草稿缓存（防止后台刷新覆盖）→ 提示手动保存；非草稿状态禁用并提示先下架。
- [x] 保存/发布/下架成功后失效修订查询缓存。
- [x] `docs/apis/articles.md` 新增「文章修订历史」章节与恢复语义说明。
- [x] `go test ./...`、`go vet ./...`、`npm run build` 全部通过。

## 边界与已知行为

- 版本列表受保留策略约束（每篇最近 20 个快照 + 首版），更早快照已被每日 00:15 清理任务删除，列表中不可见。
- 恢复载入会覆盖编辑器当前未保存的本地草稿内容（载入值同时写入本地草稿缓存）。
- 首版（version = 1）永远保留，可用作创建时的原始内容基线。

## 非目标

- 不做版本回滚专用写接口、不做 MDX 渲染级或并排预览对比、不引入 diff 依赖库。
- 不修改 `article_logs` 表结构与保留策略。
- 不影响公开接口（新端点全部在 `/api/v1/admin` 鉴权组内）。
