# PostBridge 接入 API

这些接口由 CMS 提供，路径前缀为 `/api/v1/admin/postbridge`。仅 CMS 管理员可调用，沿用后台 Bearer token。响应禁止缓存。所有访问 PostBridge 的请求由 CMS 后端发起。

| 方法与路径 | 请求 | 成功响应 |
| --- | --- | --- |
| GET /connection | 无 | 200，`enabled` 和 `signingConfigured`。后者表示签名凭据已加载。已有注册回执时同时返回 `registrationId` 和 `registrationStatus`。 |
| POST /registrations | JSON 字段 `invitationToken`、`name` | 201，`registrationId` 和 `status`，待审批状态为 `pending`。字段 `recorded` 表示 CMS 是否已保存回执。 |
| POST /activate | 空正文 | 200，`status: active` 和 `targetsVerified`。目标查询成功时附带 `items`。 |
| GET /targets | 无 | 200，授权目标列表 `items`。无可选目标时返回空数组。 |

目标项包含平台、账号 ID、显示名、账号状态、最近检查时间、可用操作、正文格式、图片能力及标题和正文长度限制。字段沿用 PostBridge 契约，例如 `account_id`、`display_name` 和 `body_formats`。

注册权限、来源和目标账号由后端配置决定。CMS 保存注册编号供管理员审批，审批状态以 PostBridge 为准。CMS 未能记录回执时仍返回注册编号。邀请用过后，请在 PostBridge 管理端按注册编号核对。激活端点先激活再查询目标。激活成功而目标查询失败时仍返回 `active`，同时返回 `targetsVerified: false`，之后使用目标接口复查。

缺少后台会话返回 401，非管理员返回 403。输入错误返回 400。未配置签名凭据或 PostBridge 接入暂不可用返回 503。上游限流返回 429，透传有效的秒数形式 `Retry-After`。其它上游故障返回 502，响应只提供 CMS 生成的错误说明。

## 准备稿与分发

以下路径以 `/api/v1/admin` 为前缀，仅在签名配置齐全并完成数据库身份绑定后注册。完整流程需要目标读取、准备稿读写、分发读写及来源事件写入权限。

| 方法与路径 | 输入 | 成功响应 |
| --- | --- | --- |
| POST /articles/{articleId}/postbridge/preparations | `requestId`、`expectedVersion`、`targetAccountId`、`textOnly` | 202，分发记录，后台创建准备稿 |
| GET /postbridge/attempts | 可选 `articleId`、`status`、`page`、`pageSize`，每页默认 20，最多 100 | 200，`items`、`page`、`pageSize`、`total` |
| GET /postbridge/attempts/{attemptId} | 无 | 200，CMS 当前保存的记录 |
| POST /postbridge/attempts/{attemptId}/refresh | 空正文 | 200，返回最新可用记录，重复刷新受间隔和上游退避限制 |
| POST /postbridge/attempts/{attemptId}/submit | 空正文 | 200，复查平台稿确认后保存提交意图，后台执行。复查失败时返回错误，限流返回 429 |
| POST /postbridge/attempts/{attemptId}/cancel | 空正文 | 200，本地授权撤销，后台查找并取消可停止的远端任务 |

准备稿只允许已发布文章。带封面的文章需要显式选择 `textOnly`，CMS 原文章保留封面。请求 ID 应在同一次准备操作的重试中复用，文章修订或目标改变时使用新 ID。不存在的记录返回 404，版本或状态冲突返回 409，内容不兼容返回 422，快照超过外部请求上限返回 413。

分发记录包含 `id`、`articleId`、`sourceVersion`、`targetAccountId`、`status`、授权期限和更新时间。根据当前阶段提供 `previewUrl`、`detailUrl`、`childStatus`、`externalStatus`、`outcome`、`lastConfirmedAt` 及可用的外站结果链接。操作按钮由 `canSubmit`、`canCancel` 和 `canRefresh` 控制。

撤销以 `revoked` 表示，待发取消以 `cancelPending` 表示。取消结果单独保留在 `cancelResult`，包括未提交、取消请求已接受、远端不可取消及未找到原任务。已发出的提交若暂时查不到，显示待核对。确认平台稿使用 PostBridge 的管理员会话。未提交的准备稿在准备稿或授权到期后标记为已过期。CMS 根据子任务状态展示草稿保存或待核对，外站结果未知时可打开 PostBridge 详情。

## 后端配置

| 配置项 | 用途 |
| --- | --- |
| `POSTBRIDGE_ORIGIN` | HTTPS 根地址。留空时接入接口不注册。 |
| `POSTBRIDGE_ENVIRONMENT` | 与邀请和 PostBridge 部署一致的环境标识。 |
| `POSTBRIDGE_SOURCE_ID`、`POSTBRIDGE_ACCOUNTS` | 注册时使用的来源标识和获邀账号 ID。多个账号以逗号分隔。 |
| `POSTBRIDGE_SCOPES` | 申请的权限，用逗号分隔，须属于邀请授权范围。留空只申请 `targets:read`。 |
| `POSTBRIDGE_CLIENT_ID`、`POSTBRIDGE_KEY_ID`、`POSTBRIDGE_SERVICE_ID` | 审批后使用的应用、密钥和目标服务标识。首次注册前可以留空。 |
| `POSTBRIDGE_CLIENT_SECRET_FILE` | 后端可读的受限密钥文件，保存审批交付的标准 Base64 密钥。首次注册前可以留空。 |

开发和生产 Compose 将上述变量传给后端。配置须与数据库唯一绑定的来源 ID、环境、HTTPS origin 和应用 ID 匹配，测试和生产使用各自的数据库。密钥文件由操作者以只读方式单独挂载到后端，使用容器内路径。文件及实际配置存放于本地受限位置，避开前端共享挂载和 Git 跟踪。轮换时先在独立的受控实例加载待激活密钥，确认激活和目标查询后再切换现有实例的签名凭据。环境联调、准备稿和分发流程见 [接入与验收说明](../plans/2026-11-05-postbridge-integration.md)。
