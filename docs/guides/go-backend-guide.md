# Go 后端项目导读

Go 后端负责登录、文章、相册、访问统计、资源及 PostBridge。目录以 `backend/` 为根，本地启动和数据库迁移见 [开发环境](../operations/development-environment.md)。

## 从启动入口阅读

| 入口 | 职责 | Java Web 类比 |
| --- | --- | --- |
| [cmd/server/main.go](../../backend/cmd/server/main.go) | 加载配置、装配连接池与查询、启动任务并注册 HTTP 服务 | 应用启动类与依赖装配 |
| [internal/config](../../backend/internal/config/) | 读取环境变量并验证配置 | 配置绑定 |
| [internal/http/router.go](../../backend/internal/http/router.go) | 装配 Chi 路由、请求大小限制、公共限流和后台认证 | 路由配置与过滤器链 |
| [internal/http/middleware.go](../../backend/internal/http/middleware.go) | JWT 验证、角色判断、请求日志和跨域响应 | 认证、访问日志和 CORS 过滤器 |
| [internal/database](../../backend/internal/database/) | 建立 PostgreSQL 连接池，维护 sqlc 生成代码 | DataSource 和 Mapper |
| [cmd/import-mdx](../../backend/cmd/import-mdx/) | 将本地 MDX 导入 PostgreSQL | 批处理入口 |

通过 `main.go` 可以追踪服务对象从哪里构造、怎样注入查询和存储依赖。HTTP 处理器通过请求的 `Context` 传递认证信息、取消和超时。数据库或外部调用返回的 `error` 由调用方检查，失败时转换为对应的 HTTP 响应。

## 按业务入口阅读

| 模块 | 代码与契约 |
| --- | --- |
| 登录 | [auth](../../backend/internal/auth/)校验密码、签发 JWT、记录登录失败并执行风险触发的邮箱验证。[认证 API](../apis/auth.md) |
| 文章 | [articles](../../backend/internal/articles/)处理编辑、状态流转、修订与公开读取；公开详情及统计复用同一个文章标识解析器。[文章 API](../apis/articles.md) |
| 相册 | [albums](../../backend/internal/albums/)负责公开分页、图片上传、相册事务与对象清理重试；[imaging](../../backend/internal/imaging/)提取尺寸与拍摄时间并清理隐私元数据。[相册 API](../apis/photos.md) |
| 访问统计 | [analytics](../../backend/internal/analytics/)把文章、相册或站点事件及相应日计数原子写入，并归档文章和站点旧日计数。[统计 API](../apis/analytics.md) |
| 首页 | [dashboard](../../backend/internal/dashboard/)读取已有业务数据并提供后台只读汇总。[首页 API](../apis/dashboard.md) |
| 资源 | [assets](../../backend/internal/assets/)管理文件元数据并记录下载审计；[storage](../../backend/internal/storage/)封装 S3 兼容存储的预签名及对象操作。[文件 API](../apis/assets.md) |
| PostBridge | [integrations/postbridge](../../backend/internal/integrations/postbridge/)管理来源授权、内容快照、分发状态及待发来源事件。[PostBridge API](../apis/postbridge.md) |

在模块的 HTTP 入口顺着请求类型、依赖接口和数据库调用阅读。Go 类型通过方法集合满足接口，测试可为处理器提供同一接口的替身，无需启动完整服务。

## 数据库与验证

表结构由 [迁移](../../backend/migrations/)管理，查询写在 [SQL 目录](../../backend/sql/)。由 [sqlc 配置](../../backend/sqlc.yaml)生成的 Go 代码集中在 `internal/database/dbgen/`。修改查询时先更新 SQL，再重新生成。数据库触发器、去重与清理规则见 [数据库说明](../sql/schema.md)。

本地按影响范围运行相关包测试。需要 PostgreSQL 的集成测试使用显式配置的测试库和隔离 schema，入口见 [后端验证说明](../../backend/README.md#验证要求)。全量测试、静态检查和生成一致性由 [GitHub CI](../../.github/workflows/verify.yml)执行。
