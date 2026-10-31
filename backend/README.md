# Rendering CMS Platform Backend

`backend/` 是 Rendering CMS Platform 的 Go API 服务，负责后台管理、公开内容读取、认证授权、文章管理、评论审核、访问统计、文件元数据和数据库访问。

本模块只提供 API 与业务运行时能力，不直接写入原静态博客仓库，也不保存上传文件本体。上传文件本体应存储在 S3 兼容对象存储中，数据库只保存元数据与 `storage_key`。

## 技术栈

- Go 1.26.10
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
  internal/dashboard/         # 首页只读汇总、图文排行与相册摘要
  internal/comments/          # 评论提交与审核
  internal/assets/            # 文件上传下载与审计
  internal/albums/            # 相册与照片管理（公开查询 + 后台管理）
  internal/imaging/           # 照片处理：魔数嗅探、尺寸提取、EXIF/XMP 等元数据剥离
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
- `S3_USE_PATH_STYLE`：是否使用路径风格寻址；本地 MinIO 使用 `true`，生产 Cloudflare R2 使用 `false`。
- `S3_PUBLIC_BASE_URL`：公开访问对象的基础 URL，例如 `https://assets.rendering.me`，用于文章图片回填。
- `S3_BLOG_IMAGE_PREFIX`：文章图片对象 key 前缀，默认 `blog`。
- `S3_ASSET_FILE_PREFIX`：后台资源文件对象 key 前缀，默认 `assets`。
- `S3_PHOTO_PREFIX`：相册照片对象 key 前缀，默认 `photos`；照片为公开内容，依赖 `S3_PUBLIC_BASE_URL` 拼接公开地址，未配置公共基地址时照片上传接口返回 `503`。

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

本地仅运行受改动影响的包测试，完整验证由 GitHub CI 执行，具体要求见下方验证章节。

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

本地按改动影响范围运行相关包测试，必要时做静态检查；涉及 SQL 时用 sqlc 重新生成查询并验证生成文件一致。全量检查由 GitHub CI 承担。

## 首页汇总与验证

首页三个只读接口由 internal/dashboard 装配在后台鉴权路由下，查询位于 sql/dashboard.sql，生成文件由 sqlc v1.31.1 维护。字段、数量上限、资源状态与统计口径见 [首页 API](../docs/apis/dashboard.md)。不新增数据库 schema，不改变公开访问上报。

本地按改动范围运行相关包测试。dashboard 的数据库集成测试通过 TEST_DATABASE_URL 显式连接专用 PostgreSQL，逐项应用现有 migration 并清理隔离 schema；没有配置时会跳过。全量 Go 测试、静态检查、查询生成一致性与真实数据库验证由 [GitHub CI](../.github/workflows/verify.yml) 执行。
