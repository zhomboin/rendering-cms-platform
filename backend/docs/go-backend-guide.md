# Go 后端项目导读

本文面向有 Java Web 开发经验、正在学习 Go 语言的开发者，说明当前 `backend/` Go 后端的项目结构、模块职责、常见 Go 语法和使用到的主要依赖。

## 整体定位

`backend/` 是 Rendering CMS Platform 的 Go API 服务，负责：

- 后台登录和 JWT 鉴权。
- 文章草稿、编辑、发布和公开读取。
- 访问统计和后台看板数据。
- 评论提交、审核和公开展示。
- 文件资源上传、下载链接生成和下载审计。
- PostgreSQL 数据访问和 S3 兼容对象存储对接。

如果使用 Java Web 思维类比，可以理解为：

```text
cmd/server/main.go      ≈ Spring Boot Application main
internal/http/          ≈ WebMvcConfig + Filter + Router
internal/*/handler.go   ≈ Controller
internal/*/service.go   ≈ Service / Domain helper
internal/database/      ≈ DataSource 配置
internal/database/dbgen ≈ Mapper / Repository 生成代码
migrations/             ≈ Flyway migration
sql/*.sql               ≈ MyBatis XML / Mapper SQL
```

Go 没有 Spring 容器，本项目采用显式依赖装配：在 `cmd/server/main.go` 中读取配置、打开数据库连接、创建 sqlc 查询对象、创建各业务 handler，最后挂载到 HTTP Router。

## 目录结构

```text
backend/
  cmd/
    server/        # HTTP 服务入口
    import-mdx/    # MDX 导入命令行工具
  internal/
    config/        # 环境变量配置
    database/      # PostgreSQL 连接
    database/dbgen # sqlc 生成的数据库访问代码
    http/          # 路由和中间件
    auth/          # 登录、JWT、密码哈希
    articles/      # 文章管理
    analytics/     # 访问统计
    comments/      # 评论提交和审核
    assets/        # 文件上传下载和审计
    storage/       # S3 / MinIO 客户端
  migrations/      # PostgreSQL migration
  sql/             # sqlc 查询 SQL
  go.mod           # Go module 和依赖声明
  go.sum           # 依赖版本校验
  sqlc.yaml        # sqlc 配置
```

`internal/` 是 Go 的特殊目录。放在 `internal` 下的 package 只能被当前 module 内部引用，外部项目不能直接 import。它适合放应用内部实现，类似 Java 项目中不希望作为 SDK 暴露的内部包。

## 启动入口

入口文件：

```text
backend/cmd/server/main.go
```

核心流程：

```go
cfg, err := config.Load()
db, err := database.Open(context.Background(), cfg.DatabaseURL)
queries := dbgen.New(db)

articleHandler := articles.NewHandler(queries)
analyticsHandler := analytics.NewHandler(queries)
commentHandler := comments.NewHandler(queries)
assetHandler := assets.NewHandler(queries, storageClient)
```

含义：

- `config.Load()`：读取环境变量，类似 Spring 的配置绑定。
- `database.Open()`：创建 PostgreSQL 连接池，类似 `DataSource`。
- `dbgen.New(db)`：创建 sqlc 查询入口，类似 Mapper 汇总对象。
- `NewHandler(...)`：手动创建各业务 Controller。
- `http.Server`：启动 HTTP 服务。

Go 的依赖关系一般显式写出来，不依赖注解扫描和自动注入。

## 路由与中间件

路由文件：

```text
backend/internal/http/router.go
```

本项目使用 Chi 路由库：

```go
github.com/go-chi/chi/v5
```

典型路由：

```go
router.Get("/api/v1/health", handler)
router.Post("/api/v1/auth/login", config.LoginHandler)
```

后台路由统一放在 `/api/v1/admin` 下：

```go
router.Route("/api/v1/admin", func(admin chi.Router) {
    admin.Use(AdminAuthMiddleware(config.JWTSecret))
    ...
})
```

这类似 Spring Security 对 `/admin/**` 配置统一过滤器。

中间件文件：

```text
backend/internal/http/middleware.go
```

`AdminAuthMiddleware` 负责：

- 读取 `Authorization` Header。
- 校验 `Bearer <token>`。
- 解析 JWT。
- 校验角色是否为 `admin` 或 `editor`。
- 将当前用户放入 request context。

Go 的 `context.Context` 可以类比 Java 中的 `SecurityContextHolder` 或 request attribute，不过 Go 里会显式通过请求对象传递。

## 业务模块

### auth

路径：

```text
backend/internal/auth/
```

职责：

- bcrypt 密码哈希与校验。
- JWT 签发与解析。
- 登录接口。
- 根据邮箱查询用户。

主要文件：

```text
password.go
token.go
handler.go
```

主要依赖：

```go
golang.org/x/crypto/bcrypt
github.com/golang-jwt/jwt/v5
```

### articles

路径：

```text
backend/internal/articles/
```

职责：

- 公开文章列表。
- 公开文章详情。
- 后台文章列表。
- 创建草稿。
- 更新草稿。
- 发布文章。
- 通过数据库触发器写入文章版本日志。

对应接口：

```text
GET    /api/v1/articles
GET    /api/v1/articles/{slug}
GET    /api/v1/admin/articles
POST   /api/v1/admin/articles
PATCH  /api/v1/admin/articles/{id}
POST   /api/v1/admin/articles/{id}/publish
```

### analytics

路径：

```text
backend/internal/analytics/
```

职责：

- 记录文章访问量。
- 记录站点访问量。
- 输出后台统计汇总。
- 计算热门文章。

对应接口：

```text
POST /api/v1/articles/{slug}/views
GET  /api/v1/admin/analytics/summary
```

### comments

路径：

```text
backend/internal/comments/
```

职责：

- 访客提交评论。
- 新评论默认 `pending`。
- 公开只展示 `approved` 评论。
- 后台审核 `approved` / `rejected`。
- 只保存 IP 哈希，不保存原始 IP。

对应接口：

```text
GET   /api/v1/articles/{slug}/comments
POST  /api/v1/articles/{slug}/comments
GET   /api/v1/admin/comments
PATCH /api/v1/admin/comments/{id}
```

### assets

路径：

```text
backend/internal/assets/
```

职责：

- 校验上传文件类型和大小。
- 申请预签名上传 URL。
- 查询资源列表。
- 申请预签名下载 URL。
- 写入下载审计。

对应接口：

```text
GET  /api/v1/admin/assets
POST /api/v1/admin/assets/upload-url
GET  /api/v1/admin/assets/{id}/download-url
```

### storage

路径：

```text
backend/internal/storage/
```

职责：

- 创建 S3 兼容客户端。
- 生成上传 URL。
- 生成下载 URL。

主要依赖：

```go
github.com/aws/aws-sdk-go-v2/service/s3
```

本地开发使用 MinIO，生产环境可以替换为其他 S3 兼容对象存储。

## 数据库访问

数据库连接文件：

```text
backend/internal/database/db.go
```

当前使用 `pgxpool`：

```go
github.com/jackc/pgx/v5/pgxpool
```

类比 Java：

```text
pgxpool.Pool ≈ HikariCP DataSource
DATABASE_URL ≈ JDBC URL
dbgen.Queries ≈ MyBatis Mapper 汇总对象
```

本项目使用 sqlc。SQL 文件在：

```text
backend/sql/
```

生成代码在：

```text
backend/internal/database/dbgen/
```

示例 SQL：

```sql
-- name: ListPublishedArticles :many
select ...
from articles
where status = 'published';
```

sqlc 会生成类型安全 Go 方法：

```go
ListPublishedArticles(ctx context.Context) ([]Article, error)
```

这种方式类似 MyBatis，但 SQL 是源头，Go 类型由 sqlc 自动生成，字段类型错误可以在编译期暴露。

## 常见 Go 语法

### package 和 import

Go 文件开头会声明 package：

```go
package articles
```

导入依赖：

```go
import (
    "context"
    "net/http"

    "rendering-cms-platform/backend/internal/articles"
)
```

标准库直接用包名，项目内部包使用 `go.mod` 中的 module 路径作为前缀。

### struct

Go 没有 class，常用 `struct` 表示数据结构：

```go
type Config struct {
    HTTPAddr    string
    DatabaseURL string
    JWTSecret   string
}
```

类似 Java：

```java
class Config {
    String httpAddr;
    String databaseUrl;
    String jwtSecret;
}
```

### 方法 receiver

Go 方法通过 receiver 绑定到类型：

```go
func (h Handler) listPublishedArticles(w http.ResponseWriter, r *http.Request) {
    ...
}
```

`(h Handler)` 表示这个函数属于 `Handler` 类型。类比 Java：

```java
class Handler {
    void listPublishedArticles(...) {}
}
```

### 指针

```go
queries *dbgen.Queries
r *http.Request
```

`*T` 表示指向 T 的指针。Java 对象引用默认有类似语义，Go 需要显式写出。常见用途：

- 避免复制大对象。
- 共享数据库连接池、请求对象等状态。
- 允许方法修改对象内部状态。

### error 显式处理

Go 不用异常处理常规错误，通常返回 `error`：

```go
articles, err := h.queries.ListPublishedArticles(r.Context())
if err != nil {
    writeError(w, http.StatusInternalServerError, "文章列表读取失败")
    return
}
```

这是 Go 中非常核心的写法。看到 `err != nil` 基本就表示失败分支。

### 短变量声明

```go
articles, err := h.queries.ListPublishedArticles(r.Context())
```

`:=` 表示声明并赋值，由 Go 自动推断类型。已经声明过的变量再次赋值时使用 `=`。

### slice

```go
Tags []string
```

`[]string` 是 slice，类似 Java 的 `List<String>`。它比固定长度数组更常用。

### map

```go
map[string]interface{}{
    "articleId": article.ArticleID.String(),
    "title":     article.Title,
}
```

类似 Java 的 `Map<String, Object>`。`interface{}` 表示任意类型，新版本 Go 也可以写成 `any`。

### JSON tag

```go
type ArticlePayload struct {
    BodyMdx string `json:"bodyMdx"`
}
```

反引号中的 `json:"bodyMdx"` 是 struct tag，类似 Java Jackson 的 `@JsonProperty("bodyMdx")`。

### interface

Go 的 interface 描述行为，而不是描述继承关系。例如资源模块中可以定义：

```go
type URLSigner interface {
    PresignUploadURL(...) (string, error)
    PresignDownloadURL(...) (string, error)
}
```

只要某个类型实现了这些方法，就自动满足该 interface，不需要像 Java 一样显式 `implements`。

### context

HTTP 请求中常见：

```go
r.Context()
```

数据库查询、外部请求和中间件都会传递 `context.Context`，用于：

- 超时控制。
- 请求取消。
- 传递请求范围内的值。

## 测试

Go 测试文件以 `_test.go` 结尾：

```text
service_test.go
handler_test.go
config_test.go
```

测试函数以 `Test` 开头：

```go
func TestValidateUpload(t *testing.T) {
    ...
}
```

运行全部后端测试：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
go test ./...
```

`./...` 表示当前目录下所有 package，类似 Maven 或 Gradle 的全模块测试。

## 建议阅读顺序

如果目标是借这个项目学习 Go，可以按下面顺序看代码：

1. `internal/config/config.go`：学习 struct、函数、环境变量和 error 返回。
2. `internal/database/db.go`：学习数据库连接池的最小封装。
3. `internal/http/router.go`：学习函数类型、可变参数、Functional Options 和路由注册。
4. `internal/http/middleware.go`：学习中间件、闭包、context 和 JWT 鉴权流程。
5. `internal/assets/service.go`：学习简单业务函数和单元测试。
6. `internal/articles/handler.go`：学习 HTTP handler、JSON 编解码、错误处理和数据库调用。
7. `sql/*.sql` 与 `internal/database/dbgen/`：学习 sqlc 如何把 SQL 生成类型安全 Go 代码。

## 和 Java Web 的主要差异

- Go 没有 Spring 容器，依赖通常手动装配。
- Go 没有 class，主要使用 `struct + method + interface`。
- Go 不用异常处理常规错误，而是显式返回和检查 `error`。
- Go 的 HTTP 标准库能力较强，Chi 这类框架更多是薄路由层。
- Go 项目常把 SQL 作为一等公民，配合 sqlc 生成类型安全访问代码。
- Go 编译和测试命令简单，`go test ./...` 是日常开发的基础验证命令。
