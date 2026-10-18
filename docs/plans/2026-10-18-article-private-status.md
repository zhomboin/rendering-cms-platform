# 文章私有状态与后台阅读页实施计划

**目标：** 让日常记录、运维文档等不适合公开的内容可以以 `private` 状态保存在 CMS 中：仅登录后台可见可读，永远不会出现在公开站点；同时在后台提供一个博客正文式的沉浸阅读页，私有内容与草稿、已发布文章都能获得良好的阅读体验。

**已确认决策：**

- 私有能力通过 `article_status` 枚举新增 `private` 值实现，不引入独立的 visibility 字段：公开接口全部以 `status = 'published'` 过滤，新枚举值天然不出现在公开面。
- `private` 与 `draft` 同为可编辑状态，可直接修改保存；每次保存由现有触发器写 `article_logs` 修订并递增 `version`。
- 状态流转：`draft -> private`（设为私有）、`private -> draft`（转为草稿）；想公开需先转为草稿再发布，两步均为显式操作。
- 第一版不做 `published -> private` 直转（先下架为草稿再设为私有）；`archived` 保持保留值。
- 阅读页 `/admin/articles/:id/read` 建在后台 SPA 内、独立于 AdminLayout 的全屏形态；对所有状态开放，数据走既有 `GET /api/v1/admin/articles`（登录态保护）。
- 阅读页渲染复用编辑器 MDX 预览的自研解析器与渲染器（抽取为共享模块），不引入 markdown 渲染依赖；代码块提供复制按钮，语法高亮列为后续可选。

**架构：** PostgreSQL migration 为 `article_status` 增加 `private` 枚举值（down 通过重建类型真实回滚）；后端新增设为私有/转为草稿接口，PATCH 草稿接口按状态分发到私有更新查询；前端新增阅读页、私有状态管理与操作入口。

**技术栈：** Go、Chi、pgx/sqlc、golang-migrate、PostgreSQL、React + TypeScript + Vite。

## 执行状态（2026-10-18）

- [x] migration `000011_article_private_status`：up 为 `alter type article_status add value 'private'`；down 先将 `article_logs`、`articles` 中的 `private` 回退为 `draft`，再重建三值枚举类型。
- [x] sqlc 查询：`MarkArticlePrivate`（draft -> private）、`RestoreArticleToDraft`（private -> draft）、`UpdatePrivateArticle`（守卫 `status = 'private'`，保留原 slug）。
- [x] 后端接口：`POST /api/v1/admin/articles/{id}/private`、`POST /api/v1/admin/articles/{id}/restore-draft`，404/409 语义与并发竞态兜底与 publish/unpublish 一致。
- [x] `PATCH /api/v1/admin/articles/{id}` 按当前状态分发：draft 走原更新查询，private 走 `UpdatePrivateArticle`，其余 409。
- [x] 后端测试：设为私有成功/409/404/竞态 409，转为草稿成功/409，PATCH 编辑私有文章成功、已发布仍 409。
- [x] 前端 API：`ArticleStatus` 增加 `private`，新增 `markAdminArticlePrivate`、`restoreAdminArticleToDraft`。
- [x] 文章列表页：状态筛选与 Tag 增加「私有」（紫色）；行操作增加「阅读」（全状态）、「设为私有」（草稿）、「转草稿」（私有）。
- [x] 编辑器页：私有文章可直接编辑保存（Ctrl+S 可用）；私有提示条提供「转为草稿」；草稿提供「设为私有」；私有状态下发布按钮禁用并提示；历史版本恢复放宽到草稿/私有。
- [x] 抽取共享渲染模块 `MdxBlockRenderer`，编辑器 MDX 预览与阅读页共用解析渲染逻辑。
- [x] 新增阅读页 `/admin/articles/:id/read`：全屏沉浸排版（760px 正文宽度、16px 正文、1.85 行高）、标题锚点目录侧栏（≥1200px 显示）、代码块复制按钮、页头展示状态/标签/时间/版本。
- [x] `docs/apis/articles.md`、`docs/sql/schema.md` 已同步；`AGENTS.md` 功能范围已更新。
- [x] `go test ./...`、`go vet ./...`、`npm run build`、`npm run test:mdx-preview` 全部通过。
- [ ] 预发布/生产环境执行：先备份 PostgreSQL，再执行 migration；验收草稿设为私有 -> 公开接口 404 -> 编辑保存 -> 阅读页查看 -> 转为草稿 -> 发布。

## 状态机口径

```text
draft --发布 publish--> published --下架 unpublish--> draft
draft --设为私有 private--> private --转为草稿 restore-draft--> draft
private：仅后台接口返回（登录态），published_at 保持空
archived：保留值，无接口可进入或离开
```

## 行为变更说明

- PATCH 更新接口此前仅草稿可改（非草稿 409）；现在草稿与私有均可改，其余状态 409，提示语调整为「仅草稿或私有文章可直接编辑，已发布文章请先下架」。
- 公开接口行为完全不变：列表、详情（slug/英文名）、搜索都只返回 `published`，私有文章一律 404。
- 阅读页数据来源为后台文章列表接口（本就返回 `body_mdx`），未新增公开面或免登录读接口。

## 边界与已知影响

- `article_status` 枚举值不可直接删除，down migration 通过重建类型回滚，会连带重建 `articles`、`article_logs` 的 status 列索引；生产执行任何方向的 migration 前必须先备份 PostgreSQL。
- 私有文章不产生公开访问与评论数据；评论、统计、热门文章均以 `published` 为口径，无需改动。
- 阅读页展示的是已保存内容，编辑器内未保存的修改不会出现在阅读页；编辑器按钮已通过 tooltip 说明。
- 复杂 MDX 组件（非标题/列表/引用/表格/代码块）在阅读页与预览一样按源码或纯文本展示，最终公开渲染以 Rendering 博客为准。
