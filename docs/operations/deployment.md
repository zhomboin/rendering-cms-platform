# MVP 部署流程

本文档记录 Rendering CMS Platform MVP 的基础部署和验证流程。命令默认在 WSL Ubuntu 24.04 或 Linux 服务器中执行。

## 环境变量

后端运行前必须提供以下环境变量：

```env
HTTP_ADDR=:8080
DATABASE_URL=postgres://rendering:password@127.0.0.1:5432/rendering_cms?sslmode=disable
JWT_SECRET=replace-with-32-plus-character-secret
FRONTEND_ORIGIN=http://127.0.0.1:5173
FRONTEND_ORIGINS=http://127.0.0.1:3000,http://127.0.0.1:5173
LOG_DIR=/var/log/rendering-cms-platform
BACKEND_LOG_HOST_DIR=/var/log/rendering-cms-platform

S3_ENDPOINT=https://example.r2.cloudflarestorage.com
S3_REGION=auto
S3_BUCKET=rendering-assets
S3_ACCESS_KEY_ID=replace-me
S3_SECRET_ACCESS_KEY=replace-me
```

说明：

- `JWT_SECRET` 必须至少 32 字符，并且生产环境不得使用示例值。
- `DATABASE_URL` 指向 PostgreSQL，生产发布前必须确认连接用户具备 migration 所需权限。
- `FRONTEND_ORIGINS` 是后端 CORS 白名单，多个地址使用英文逗号分隔；如果设置了该变量，后端优先使用它，否则回退到单值 `FRONTEND_ORIGIN`。
- `LOG_DIR` 指向容器内后端请求日志目录，Docker 环境默认值为 `/var/log/rendering-cms-platform`。
- `BACKEND_LOG_HOST_DIR` 指向宿主机系统盘上的日志挂载目录，本地默认使用 `logs/backend`，生产环境建议使用 `/var/log/rendering-cms-platform` 这类系统日志目录；日志文件按天写入 `backend-YYYY-MM-DD.log`。
- `S3_ENDPOINT` 可指向 MinIO、Cloudflare R2 或其他 S3 兼容对象存储。
- 上传文件内容只进入对象存储，不写入 Git 仓库或前端 `public/` 目录。

## 发布前检查

发布前必须备份 PostgreSQL：

```bash
pg_dump "$DATABASE_URL" > "backup-$(date +%Y%m%d-%H%M%S).sql"
```

确认本地验证通过：

```bash
cd backend
go test ./...
go vet ./...

cd ../frontend
npm run build
```

## 发布顺序

1. 备份 PostgreSQL。
2. 执行 SQL migration。
3. 构建 Go 后端。
4. 构建 React 前端。
5. 重启后端服务。
6. 请求 `/api/v1/health`。

## Migration

使用 `golang-migrate` 执行数据库变更：

```bash
cd backend
migrate -path migrations -database "$DATABASE_URL" up
```

## 后端构建

```bash
cd backend
go build -o ../tmp/backend-server ./cmd/server
```

## 前端构建

```bash
cd frontend
npm ci
npm run build
```

## 健康检查

后端启动后执行：

```bash
curl -fsS http://127.0.0.1:8080/api/v1/health
```

期望响应：

```json
{
  "status": "ok"
}
```

## MVP 主流程验收

- 使用管理员账号登录后台。
- 创建文章草稿并发布。
- 在 Rendering 博客中读取 CMS 已发布文章并查看文章页面。
- 打开 Rendering 博客文章详情，确认访问统计写入 CMS。
- 提交评论，确认评论默认待审核。
- 后台审核评论后，Rendering 博客文章详情页只展示已通过评论。
- 上传允许类型的文件，确认文件进入对象存储。
- 生成下载链接，确认 `download_events` 写入审计记录。
