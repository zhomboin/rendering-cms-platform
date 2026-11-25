# AI 文章与相册 MCP 接入

| 阶段 | 使用方式 | 验收条件 |
| --- | --- | --- |
| 草稿协作 | AI 建稿、读取自己的草稿、按版本修改，作者从后台预览 | 同一请求重试只产生一篇文章，默认隔离其他连接与后台稿件，私有文章始终隔离 |
| 授权发布 | 作者为指定连接单独开启发布，AI 按已确认的版本发文 | 默认只写草稿，撤销后停止访问，并发编辑与过期版本无法覆盖或发布新内容 |
| 结果核验 | 区分 CMS 发布、公开页面可访问和搜索收录 | 仅报告实际观察到的结果，博客缓存或索引延迟可单独复查 |

定时任务由 AI 客户端或外部调度器触发。CMS 提供文章、相册和照片工具，人工审稿沿用现有后台。外站分发走独立的 PostBridge 授权流程。

## 工具

AI 客户端连接 `/api/v1/mcp`，选择 Streamable HTTP。服务使用官方 Go SDK，与客户端协商协议版本。MCP JSON 请求体上限 2 MiB。相册大图可通过工具返回的同源文件入口上传。连接开通步骤见[接入操作](../operations/article-mcp.md)。

| 工具 | 输入 | 返回与重试 |
| --- | --- | --- |
| `create_draft` | `requestKey`、`articleName`、`title`、`bodyMdx`，可选 `summary`、`tags`、`coverImageUrl`、`featured` 和 `featuredRank` | 返回文章 ID、版本和需登录的预览链接。同一连接用同一请求键和内容重试得到同一篇文章的当前状态。相同键配不同内容会报错。 |
| `get_article` | `articleId` | 按当前可见范围读取文章及版本，包含封面、精选开关、精选排序和已记录的精选时间。 |
| `update_draft` | `articleId`、`expectedVersion`，以及要修改的字段 | 支持单独提交封面、精选开关或精选排序。修改正文时同时提交 `title` 和 `bodyMdx`，省略的摘要和标签清空。省略的展示字段保留原值。 |
| `publish_article` | `articleId`、`expectedVersion` | 仅向有发布权限的连接提供。成功返回 CMS 发布状态及预期博客地址。若响应丢失，先读状态；出现版本冲突后重新审稿。 |
| `article_status` | `articleId` | 返回 CMS 状态、版本、页面核验、搜索核验和观察时间。 |

封面 `coverImageUrl` 接受长期有效的 HTTP(S) 图片地址或以单个 `/` 开头的站内路径，传空字符串可清除。首页精选开关为 `featured`，默认关闭；首次开启会记录 `featuredAt`。精选排序 `featuredRank` 为 1 至 999 的整数，默认 100，数字越小越靠前。修改草稿会产生新版本，发布使用更新后的版本号。缺少读取文章权限时，写入回执省略展示字段。

文章英文名使用小写字母、数字和中划线，最多 128 字节。请求键不能为空，最多 128 字节。正文遵循[Markdown 规则](../guides/markdown.md)。缓存草稿 ID 供后续会话继续编辑；预览链接供作者在已登录的后台打开。

## 相册和照片

这两组权限默认全部关闭。默认只能查看本连接创建的相册。「全部可读内容」允许列出和读取后台及其他连接创建的相册，并按已授工具读取其中的照片。连接可维护自建的草稿相册。管理员勾选「上传照片」并指定已发布相册后，连接可以向这些相册追加照片，新增照片立即公开。授权独立于读取范围；编辑说明、排序、封面、删除及下架仍遵守各自工具与归属限制。照片上传成功即有公开图片地址，下架相册仅改变博客相册可见性。

| 工具 | 输入及作用 |
| --- | --- |
| `create_album` | 使用 `requestKey`、`slug`、`title`、`description` 创建草稿，可传英文标题、英文导语和标签。同键同内容重试返回相同相册 ID。 |
| `list_albums`、`get_album` | 分别授予列出可见相册摘要、按 `albumId` 读取相册详情的权限。 |
| `update_album` | 按 `albumId` 替换完整基本信息。首次发布后 slug 固定。 |
| `publish_album`、`unpublish_album` | 分别授予发布和下架权。发布时相册至少有一张照片。 |
| `list_album_photos` | 按 `albumId` 分页获取照片 ID、顺序、说明、尺寸和图片地址。两个列表均可传 `offset` 和 `limit`，从 0 开始，默认每次 24 条，最多 100 条。 |
| `upload_album_photo` | 必填 `albumId` 和 `requestKey`。1 MiB 内可直接提交 `dataBase64`、必填替代文本 `alt` 及可选 `filename`、`caption`、`takenAt`。省略图片数据可取得同源文件上传地址。 |
| `update_album_photo` | 按 `albumId`、`photoId` 替换 `alt`、`caption` 和 `takenAt`。后两项省略时清空。 |
| `reorder_album_photos` | 提交相册 ID 和完整的 `photoIds` 顺序，每张照片恰好出现一次。 |
| `set_album_cover` | 提交相册 ID 和本相册的 `photoId` 指定封面，省略照片 ID 可恢复默认封面。 |
| `delete_album_photo` | 按相册 ID 和照片 ID 删除照片，并按相册清理规则回收图片。 |

文件入口为 `POST /api/v1/mcp/albums/{albumId}/photos`。客户端使用同一 Bearer 令牌，将请求键放入 `Idempotency-Key` 请求头，表单使用 multipart 的 `file`、`alt`，可选 `caption`、`takenAt`。每张图片最多 50 MiB，支持 JPEG、PNG、WebP。两种上传方式共用请求键、格式校验、元数据清理和持久化规则。同一连接、同相册、同键重试只关联一张照片；内容变化返回冲突。工具返回上传地址时尚未写入图片，收到含 `photoId` 的回执才表示上传完成。详见 [相册图片规则](photos.md#图片内容与隐私规则)。

## 授权边界

每个连接归属一个后台管理员。管理员可逐项授予文章、相册和照片工具。默认勾选建稿、读取、修改和状态查询。工具列表只返回本连接获准的工具；工具在读写事务内复核连接权限，上传在写入照片记录前再次复核。未获读取权的写入和幂等重试回执保留 ID、状态、版本和链接，标题和正文字段为空。空授权仍可认证并获取空工具列表；撤销整条连接会使凭据失效。OAuth 的文章发布需要令牌具有文章发布范围，相册和照片需要额外的 `albums:manage` 范围。每条连接的 `readScope` 默认为 `own`。选择 `all` 后，已授权的读取或状态工具可以按 ID 访问后台和其他连接的非私有文章（包括归档），相册列表、详情及照片清单可读取全部相册。切换范围不会增加工具，向指定已发布相册追加照片需单独授权，其他写入仍须满足本连接归属。私有文章对所有连接不可读。将 AI 稿转私有会永久解除原连接对该稿的访问（含全量读取），即使以后转回草稿。

| 后台接口（前缀 `/api/v1/admin/mcp`） | 用途 |
| --- | --- |
| `GET /settings` | 返回全站开关、三个来源、客户端接入地址、OAuth 发行方和配置版本。 |
| `PATCH /settings` | 提交完整的 `enabled`、`origin`、`adminOrigin`、`blogOrigin` 和 `revision`。并发修改返回 409。关闭后 MCP 入口和资源发现返回 404，管理员仍可管理连接、查询记录。 |
| `GET /operations` | 按时间倒序列出本人连接的工具调用，每页 50 条。返回 `items` 和下一页游标 `next`，续页使用 `?before=<next>`。`pending` 表示结果待确认。日志仅保存连接、工具、结果、时间和成功访问的文章或相册 ID。 |
| `POST /connections` | 必填 `label`，可选 `readScope`（`own` 或 `all`）、工具名数组 `permissions` 及已发布相册 ID 数组 `publishedUploadAlbumIds`（默认空）；省略时授予默认四项。未知、重复或 null 返回 400。个人连接令牌仅在创建时返回一次。指定 `oauthClient` 和 `oauthSubject` 时绑定配置好的 OAuth 发行方。 |
| `GET /connections` | 列出本人签发的连接、认证方式、已配置的 `permissions`、`readScope`、`publishedUploadAlbumIds`、权限版本 `permissionRevision`、撤销状态、客户端自报名称和最近活动时间。首次观测前 `lastSeenAt` 为空。 |
| `PATCH /connections/{id}` | 可分别或一起提交 `readScope`、完整 `permissions`、完整 `publishedUploadAlbumIds`。空相册数组收回全部指定相册；省略保留原值，取消上传工具时同步清空。新增目标必须存在且已发布，非空目标要求有上传工具权限。已授权相册下架后暂停追加，再发布后恢复。后台同时传 `expectedRevision`；连接变更或权限版本过期返回 409，请刷新重选。创建或修改时也可单独传 `allowPublish`；修改时只切换发布权限。 |
| `DELETE /connections/{id}` | 撤销连接，文章和修订保留。 |
| `GET /articles/{id}/search-check` | 为连接所有者返回已发布稿件的标题、公开链接和版本。 |
| `POST /articles/{id}/search-check` | 所有者提交该版本的 Pagefind 核验结果。版本变化时返回 409。 |

### 读懂发布结果

- `cms=published` 表示数据库已提交发布。公开地址由服务端配置的博客域名与 CMS 短链生成。
- `page=reachable` 表示本次读取收到 HTML 且 canonical 匹配这篇文章。它只验证页面地址可访问。404 或 410 返回 `not_found`，超时、重定向和地址不匹配返回 `unverified`。草稿返回 `not_checked`。
- `search=found` 或 `not_found` 表示在 `searchCheckedAt` 用文章标题查询本站 Pagefind 的实际结果。该记录按 CMS 版本隔离。当前版本缺少记录时返回 `not_checked`。索引故障不写入“未收录”。核验脚本见[操作步骤](../operations/article-mcp.md#核验博客搜索)。
