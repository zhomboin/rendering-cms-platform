# 代码审查报告：Rendering CMS Platform

> 审查日期：2026-06-19
> 审查范围：`backend/`、`frontend/`、`deploy/` 与相关运维配置
> 审查目标：整体架构、安全漏洞、业务逻辑、代码质量、前端布局与前端安全

---

## 一、问题总览

| 严重程度 | 数量 | 说明 |
| --- | ---: | --- |
| High | 2 | 需要优先修复，可能影响安全边界或线上发布内容 |
| Medium | 3 | 需要在近期迭代修复，影响数据可信度、安全约束或移动端可用性 |
| Low | 2 | 建议优化，主要影响维护成本和性能表现 |

## 修复状态

> 修复日期：2026-06-20

本报告列出的 7 项问题已完成代码层修复或安全降级处理：

- `X-Forwarded-For` 信任边界：后端不再信任外部传入的 `X-Forwarded-For`，反向代理配置改为传递可信客户端 IP。
- 已发布文章误保存：后端拒绝对非 `draft` 状态文章执行草稿更新，前端对已发布文章禁用直接保存和直接发布。
- 统计刷量：公开统计写入先记录 `analytics_events`，同一来源同日重复事件不再累加 daily 统计。
- 对象存储直传大小约束：预签名 PUT 绑定 `Content-Length`。
- 移动端表格：文章列表和资源列表增加横向滚动约束。
- 视觉 token：后台卡片圆角收敛到 8px。
- 前端包体：后台路由改为 `React.lazy` 动态加载。

## 二、High 问题

### 2.1 可伪造 `X-Forwarded-For`，绕过登录和评论限流

**位置**

- `backend/internal/http/middleware.go`
- `backend/internal/auth/handler.go`
- `deploy/nginx/frontend.conf`

**问题描述**

后端 `ClientIPHash` 和登录安全逻辑直接读取 `X-Forwarded-For` 的第一个 IP。生产 Nginx 配置使用 `$proxy_add_x_forwarded_for` 追加转发头。非浏览器客户端可以主动携带伪造的 `X-Forwarded-For`，让后端把伪造 IP 当成真实来源。

**影响**

- 登录失败锁定可以被伪造 IP 绕过。
- 评论频率限制可以被伪造 IP 绕过。
- 下载审计记录的 IP hash 失真。
- 访问日志中的 `remote_ip_hash` 失去安全审计价值。

**修复建议**

1. 在 Nginx 层重置客户端来源头，不沿用外部传入的 `X-Forwarded-For`：

```nginx
proxy_set_header X-Real-IP $remote_addr;
proxy_set_header X-Forwarded-For $remote_addr;
```

2. 后端只信任受控反向代理注入的头部。必要时增加可信代理网段配置。
3. 为 `ClientIPHash`、登录锁定和评论限流补充伪造头测试。

**验收方式**

- 构造带伪造 `X-Forwarded-For` 的登录失败请求，确认后端仍按真实来源计数。
- 构造带不同伪造 IP 的评论请求，确认不能绕过同一真实来源的限流。

### 2.2 已发布文章点击“保存草稿”会直接修改线上内容

**位置**

- `backend/sql/articles.sql`
- `backend/internal/articles/handler.go`
- `frontend/src/pages/articles/ArticleEditorPage.tsx`

**问题描述**

后端 `UpdateDraftArticle` 查询只按 `article_id` 更新文章，没有限制 `status = 'draft'`。前端编辑页无论文章当前状态是草稿还是已发布，都展示“保存草稿”按钮并调用同一个更新接口。

**影响**

编辑已发布文章时，点击“保存草稿”会直接更新 `title`、`summary`、`body_mdx` 等字段。公开文章读取接口会立即读到新内容，绕过“修订草稿 -> 预览 -> 再发布”的内容发布流程。

**修复建议**

1. 后端把“保存草稿”和“修改已发布文章”拆成不同用例。
2. `UpdateDraftArticle` 至少增加 `where article_id = $1 and status = 'draft'`。
3. 已发布文章的修改应创建新修订或待发布版本，不直接覆盖线上版本。
4. 前端根据文章状态调整按钮文案和行为，例如“保存修订”“发布更新”。

**验收方式**

- 对已发布文章调用草稿更新接口，应返回明确错误，不应修改公开文章内容。
- 新增后端测试覆盖草稿更新、已发布文章更新拒绝、发布更新流程。
- 前端编辑已发布文章时不应出现误导性的“保存草稿”行为。

## 三、Medium 问题

### 3.1 公开统计接口可被无限刷量

**位置**

- `backend/internal/analytics/handler.go`
- `backend/sql/analytics.sql`

**问题描述**

公开接口 `POST /api/v1/articles/{slug}/views` 和 `POST /api/v1/analytics/site-views` 会直接累加访问量。当前没有按 IP hash、路径、User-Agent 或时间窗口做去重，也没有接口级限流。

**影响**

- 后台看板访问量容易被脚本刷高。
- 热门文章排序可能被污染。
- 后续如果用统计数据做运营判断，数据可信度不足。

**修复建议**

1. 增加 `analytics_events` 或等价去重表，记录 hash 后的来源、路径和时间窗口。
2. 至少按 `ip_hash + article_id + date/hour` 做低成本去重或限流。
3. 对站点 PV 和文章 PV 分别定义“粗略统计”与“可信统计”的口径。

**验收方式**

- 同一来源在短时间内重复上报，不应无限增加统计值。
- 不同文章、不同时间窗口的统计仍能正常累计。

### 3.2 对象存储直传没有服务端强制大小约束

**位置**

- `backend/internal/assets/service.go`
- `backend/internal/assets/handler.go`
- `backend/internal/storage/s3.go`
- `frontend/src/api/assets.ts`

**问题描述**

后端生成预签名 PUT URL 前校验的是前端上报的 `byteSize`。实际文件直传到 S3/R2，不经过后端和 Nginx。恶意已登录用户可以先上报合法大小，拿到预签名 URL 后上传超过 `20MB` 的对象。

**影响**

- 上传大小上限无法被服务端强制执行。
- 对象存储可能出现超规格文件。
- 数据库记录的 `byte_size` 可能与对象真实大小不一致。

**修复建议**

1. 优先改为 presigned POST policy，并设置 `content-length-range`。
2. 如果继续使用 presigned PUT，应在上传后通过 HEAD 校验对象大小和 Content-Type，不合规则清理对象并标记资源不可用。
3. 前端保留 `file.size` 校验，但只作为体验优化，不能作为安全边界。

**验收方式**

- 尝试用预签名 URL 上传超过 20MB 文件，应被对象存储拒绝，或上传后被后端校验清理。
- 数据库资源记录状态应能反映上传完成、校验失败或已删除。

### 3.3 后台表格移动端存在横向溢出风险

**位置**

- `frontend/src/pages/articles/ArticleListPage.tsx`
- `frontend/src/pages/assets/AssetsPage.tsx`

**问题描述**

文章列表和资源列表使用 Ant Design Table，但没有设置 `scroll.x`，也没有在移动端压缩列或切换为列表卡片。列宽在手机宽度下容易撑破后台内容区。

**影响**

- 移动端出现页面级横向滚动。
- 操作按钮可能被挤出视口。
- 后台重复管理场景可用性下降。

**修复建议**

1. 为表格统一设置 `scroll={{ x: ... }}`。
2. 移动端隐藏低优先级列，保留标题、状态和操作。
3. 可抽取统一的后台表格容器，复用横向滚动和分页样式。

**验收方式**

- 在 375px 和 390px 宽度下检查文章列表、资源列表、仪表盘表格，不应出现页面级横向溢出。
- 操作按钮和筛选控件必须完整可点击。

## 四、Low 问题

### 4.1 前端设计 token 和内联样式混用，维护成本偏高

**位置**

- `frontend/src/app/theme.ts`
- `frontend/src/layouts/AdminLayout.tsx`
- `frontend/src/pages/**/*.tsx`
- `frontend/src/pages/articles/ArticleEditorPage.css`

**问题描述**

项目已经有 Ant Design 主题 token，但页面中仍大量使用内联颜色、间距、阴影和圆角。部分主题配置使用 24px 大圆角，页面级卡片又使用 8px，视觉规则不稳定。

**影响**

- 后续调整品牌色、暗色模式或紧凑布局时改动面大。
- 页面之间视觉一致性不足。
- 审查和维护时难以判断哪些样式是系统规范，哪些是临时覆盖。

**修复建议**

1. 将后台页面常用容器、筛选栏、表格卡片、统计卡抽成组件。
2. 收敛圆角、间距、颜色到 `theme.ts` 或 CSS 变量。
3. 减少页面内联样式，保留少量与组件状态强相关的样式。

### 4.2 前端构建存在单包体积偏大警告

**位置**

- `frontend/vite.config.ts`
- `frontend/src/routes/index.tsx`

**问题描述**

前端构建通过，但产物单个 JS chunk 约 `1,294.61 kB`，gzip 后约 `419.33 kB`。Vite 输出超过 500KB 的 chunk 警告。

**影响**

- 首屏加载时间增加。
- 后台路由较多时，未访问页面也会进入首包。

**修复建议**

1. 对后台页面路由使用 `React.lazy` 和动态导入。
2. 必要时在 Vite 中配置 `manualChunks`，拆分 `antd`、图标和业务页面。
3. 构建后持续关注 gzip 体积和首屏加载。

## 五、正向发现

- 后端整体分层清晰，`cmd/server` 负责依赖组装，业务 handler 通过接口依赖 sqlc 查询。
- 公开路由和后台路由有明确分组，后台 API 使用 `/api/v1/admin` 并统一挂载认证中间件。
- 前端 API 访问集中在 `frontend/src/api/`，页面没有大面积直接调用 `fetch` 或散落 axios 实例。
- 未发现 `dangerouslySetInnerHTML` 这类明显前端 XSS 高危写法。
- MDX 预览使用 React 节点渲染文本，默认具备文本转义保护。
- 前端已有本地草稿保护和 refresh token 单飞续期机制，能降低编辑器丢稿风险。

## 六、验证记录

### 已通过

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
docker run --rm -v /home/ubuntu/workspace/rendering-cms-platform/frontend:/app -w /app node:22-alpine npm run build
```

结果：构建成功。Vite 提示单个 chunk 超过 500KB。

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/frontend
docker run --rm -v /home/ubuntu/workspace/rendering-cms-platform/frontend:/app -w /app node:22-alpine npm run test:mdx-preview
```

结果：2 个 MDX 预览解析器测试通过。

### 未完成

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
docker run --rm -v /home/ubuntu/workspace/rendering-cms-platform/backend:/app -w /app golang:1.26-alpine go test ./...
```

结果：未进入业务测试阶段。当前环境访问 `proxy.golang.org` 下载 Go 依赖时出现 TLS handshake timeout。

## 七、修复优先级

1. 先修复 `X-Forwarded-For` 信任边界，并补充登录锁定、评论限流测试。
2. 再修复已发布文章直接被“保存草稿”覆盖的问题，明确修订和发布流程。
3. 增加统计接口反刷量策略，保护后台看板数据可信度。
4. 改造对象存储上传大小强约束，避免只信任客户端 `byteSize`。
5. 修复文章列表和资源列表移动端表格溢出。
6. 收敛前端样式 token，并按路由拆分前端包体。
