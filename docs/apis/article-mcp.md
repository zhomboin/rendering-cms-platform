# AI 文章 MCP 接入

| 阶段 | 使用方式 | 验收条件 |
| --- | --- | --- |
| 草稿协作 | AI 建稿、读取自己的草稿、按版本修改，作者从后台预览 | 同一请求重试只产生一篇文章，其他连接、私人文章和普通后台稿件隔离 |
| 授权发布 | 作者为指定连接单独开启发布，AI 按已确认的版本发文 | 默认只写草稿，撤销后停止访问，并发编辑与过期版本无法覆盖或发布新内容 |
| 结果核验 | 区分 CMS 发布、公开页面可访问和搜索收录 | 仅报告实际观察到的结果，博客缓存或索引延迟可单独复查 |

定时任务由 AI 客户端或外部调度器触发。CMS 提供文章工具，人工审稿沿用现有后台。外站分发走独立的 PostBridge 授权流程。

## 工具

AI 客户端连接 `/api/v1/mcp`，选择 Streamable HTTP。服务使用官方 Go SDK，与客户端协商协议版本。请求体上限 2 MiB。连接开通步骤见[接入操作](../operations/article-mcp.md)。

| 工具 | 输入 | 返回与重试 |
| --- | --- | --- |
| `create_draft` | `requestKey`、`articleName`、`title`、`bodyMdx`，可选 `summary`、`tags` | 返回文章 ID、版本和需登录的预览链接。同一连接用同一请求键和内容重试得到同一篇文章的当前状态。相同键配不同内容会报错。 |
| `get_article` | `articleId` | 读取本连接创建且仍有权限的文章及版本。 |
| `update_draft` | `articleId`、`expectedVersion`、完整标题和正文，可选摘要和标签 | 仅写草稿，省略的摘要和标签清空。名称、封面、精选设置保持原值。 |
| `publish_article` | `articleId`、`expectedVersion` | 仅向有发布权限的连接提供。成功返回 CMS 发布状态及预期博客地址。若响应丢失，先读状态；出现版本冲突后重新审稿。 |
| `article_status` | `articleId` | 返回 CMS 状态、版本、页面核验、搜索核验和观察时间。 |

文章英文名使用小写字母、数字和中划线，最多 128 字节。请求键不能为空，最多 128 字节。正文遵循[Markdown 规则](../guides/markdown.md)。缓存草稿 ID 供后续会话继续编辑；预览链接供作者在已登录的后台打开。

### 授权边界

每个连接归属一个后台管理员。管理员可逐项授予上表五个工具。默认勾选建稿、读取、修改和状态查询。工具列表只返回本连接获准的工具；每次文章操作均在事务中复核权限。未获读取权的写入和幂等重试回执保留 ID、状态、版本和链接，标题和正文字段为空。空授权仍可认证并获取空工具列表；撤销整条连接会使凭据失效。OAuth 发布还需令牌具有发布范围。普通后台草稿和其他连接的稿件对工具不可见。将 AI 稿转私有会永久解除该连接对该稿的访问，即使以后转回草稿。

| 后台接口（前缀 `/api/v1/admin/mcp`） | 用途 |
| --- | --- |
| `GET /settings` | 返回全站开关、三个来源、客户端接入地址、OAuth 发行方和配置版本。 |
| `PATCH /settings` | 提交完整的 `enabled`、`origin`、`adminOrigin`、`blogOrigin` 和 `revision`。并发修改返回 409。关闭后 MCP 入口和资源发现返回 404，管理员仍可管理连接、查询记录。 |
| `GET /operations` | 按时间倒序列出本人连接的工具调用，每页 50 条。返回 `items` 和下一页游标 `next`，续页使用 `?before=<next>`。`pending` 表示结果待确认。日志仅保存连接、工具、结果、时间和成功访问的文章 ID。 |
| `POST /connections` | 必填 `label`，可传工具名数组 `permissions`；省略时授予默认四项。未知、重复或 null 返回 400。个人连接令牌仅在创建时返回一次。指定 `oauthClient` 和 `oauthSubject` 时绑定配置好的 OAuth 发行方。 |
| `GET /connections` | 列出本人签发的连接、认证方式、已配置的 `permissions`、权限版本 `permissionRevision`、撤销状态、客户端自报名称和最近活动时间。首次观测前 `lastSeenAt` 为空。 |
| `PATCH /connections/{id}` | 提交完整 `permissions`（允许空数组）。后台同时传 `expectedRevision`；连接变更或权限版本过期返回 409，请刷新重选。创建或修改时也可单独传 `allowPublish`；修改时只切换发布权限。 |
| `DELETE /connections/{id}` | 撤销连接，文章和修订保留。 |
| `GET /articles/{id}/search-check` | 为连接所有者返回已发布稿件的标题、公开链接和版本。 |
| `POST /articles/{id}/search-check` | 所有者提交该版本的 Pagefind 核验结果。版本变化时返回 409。 |

### 读懂发布结果

- `cms=published` 表示数据库已提交发布。公开地址由服务端配置的博客域名与 CMS 短链生成。
- `page=reachable` 表示本次读取收到 HTML 且 canonical 匹配这篇文章。它只验证页面地址可访问。404 或 410 返回 `not_found`，超时、重定向和地址不匹配返回 `unverified`。草稿返回 `not_checked`。
- `search=found` 或 `not_found` 表示在 `searchCheckedAt` 用文章标题查询本站 Pagefind 的实际结果。该记录按 CMS 版本隔离。当前版本缺少记录时返回 `not_checked`。索引故障不写入“未收录”。核验脚本见[操作步骤](../operations/article-mcp.md#核验博客搜索)。
