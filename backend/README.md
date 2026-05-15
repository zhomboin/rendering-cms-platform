# Rendering CMS Platform Backend

`backend/` 是 Rendering CMS Platform 的 Go API 服务，负责后台管理、公开内容读取、认证授权、文章管理、评论审核、访问统计、文件元数据和数据库访问。

本模块只提供 API 与业务运行时能力，不直接写入原静态博客仓库，也不保存上传文件本体。上传文件本体应存储在 S3 兼容对象存储中，数据库只保存元数据与 `storage_key`。

## 技术栈

- Go 1.26.2
- Chi
- pgx / pgxpool
- sqlc
- golang-migrate
- PostgreSQL
- S3 兼容对象存储

## 目录结构

```text
backend/
  cmd/server/                 # HTTP 服务启动入口
  cmd/import-mdx/              # MDX 导入命令行工具
  internal/config/            # 环境变量配置加载
  internal/database/          # 数据库连接与 sqlc 生成代码
    dbgen/                    # sqlc generate 输出目录
  internal/http/              # 路由与 HTTP 入口
  internal/auth/              # 登录、JWT、密码哈希
  internal/articles/          # 文章管理
  internal/analytics/         # 访问统计
  internal/comments/          # 评论提交与审核
  internal/assets/            # 文件上传下载与审计
  internal/storage/           # S3 / MinIO 客户端
  migrations/                 # PostgreSQL migration
  sql/                        # sqlc 查询文件
  go.mod
  go.sum
  sqlc.yaml
```

## Go 后端学习导读

如果你有 Java Web 开发背景，建议阅读：

- [Go 后端项目导读](../docs/guides/go-backend-guide.md)

该文档按当前代码介绍项目结构、模块职责、Java Web 类比、Go 语法点、sqlc 数据库访问和建议阅读顺序。

## 环境变量

后端从环境变量读取配置。开发环境建议复制根目录示例：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform
cp scripts/env/dev.env.example .env
```

当前基础配置项：

- `HTTP_ADDR`：HTTP 监听地址，默认 `:8080`。
- `DATABASE_URL`：PostgreSQL 连接字符串。
- `JWT_SECRET`：JWT 密钥，必填。
- `FRONTEND_ORIGIN`：兼容旧配置的单个前端地址，默认 `http://127.0.0.1:5173`。
- `FRONTEND_ORIGINS`：CORS 白名单，多个前端地址使用英文逗号分隔，例如 `http://127.0.0.1:3000,http://127.0.0.1:5173`。
- `S3_ENDPOINT`：S3 兼容服务 endpoint。
- `S3_REGION`：S3 区域，默认 `us-east-1`。
- `S3_BUCKET`：对象存储 bucket。
- `S3_ACCESS_KEY_ID`：对象存储访问 key。
- `S3_SECRET_ACCESS_KEY`：对象存储访问密钥。

## 本地开发

本项目固定在 WSL2 Ubuntu 24.04 中执行命令。

启动依赖服务：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform
bash scripts/env/start-prerequisites.sh
```

使用 Docker 启动后端：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform
bash scripts/env/start-backend-docker.sh
```

运行测试：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
go test ./...
```

也可以在 WSL 中直接启动后端：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
set -a
source ../.env
set +a
go run ./cmd/server
```

健康检查：

```bash
curl http://127.0.0.1:8080/api/v1/health
```

预期返回：

```json
{"status":"ok"}
```

## 数据库与 sqlc

Migration 位于 `backend/migrations/`。

sqlc 配置位于 `backend/sqlc.yaml`，查询文件位于 `backend/sql/`，生成代码输出到 `backend/internal/database/dbgen/`。

重新生成数据库访问代码：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
sqlc generate
```

执行 migration 的具体命令应在数据库服务启动并确认 `DATABASE_URL` 后运行。生产环境执行 migration 前必须先备份 PostgreSQL。

## 当前 API

当前接口契约以根目录 `docs/apis/` 下的中文 Markdown 文档为准。后续接口行为变更必须同步更新对应 API 文档。

## 验证要求

后端变更后至少运行：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
go test ./...
```

涉及 SQL、migration 或查询文件时运行：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
sqlc generate
go test ./...
```
