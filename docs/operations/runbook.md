# 运行手册

本文档记录 Rendering CMS Platform 的日常发布、验证和故障处理流程。命令默认在 WSL Ubuntu 24.04 或 Linux 服务器中执行。

## 发布前检查

1. 确认当前 Git 分支和提交：

   ```bash
   git status --short --branch
   git log -1 --oneline
   ```

2. 确认环境变量已配置：

   ```bash
   test -n "$DATABASE_URL"
   test -n "$JWT_SECRET"
   test -n "$FRONTEND_ORIGINS"
   ```

3. 运行后端验证：

   ```bash
   cd backend
   go test ./...
   go vet ./...
   ```

4. 运行前端构建：

   ```bash
   cd frontend
   npm ci
   npm run build
   ```

5. 备份 PostgreSQL：

   ```bash
   mkdir -p backups
   pg_dump "$DATABASE_URL" > "backups/rendering-cms-$(date +%Y%m%d-%H%M%S).sql"
   ```

6. 执行 migration：

   ```bash
   cd backend
   migrate -path migrations -database "$DATABASE_URL" up
   ```

## 发布顺序

1. 拉取或部署目标提交。
2. 构建后端二进制。
3. 构建前端静态资源。
4. 执行数据库 migration。
5. 重启后端服务。
6. 刷新前端静态资源或重启静态资源服务。
7. 执行发布后 smoke test。

## 发布后 smoke test

1. 请求健康检查：

   ```bash
   curl -fsS http://127.0.0.1:8080/api/v1/health
   ```

2. 登录后台。
3. 创建一篇草稿并保存。
4. 打开文章编辑页，确认草稿内容可读取。
5. 发布一篇测试文章或检查最近文章发布状态。
6. 提交一条评论并在后台审核。
7. 上传一个小文件并生成下载链接。
8. 打开统计页面，确认访问统计接口正常。
9. 检查当天后端日志文件是否出现 `http_request` 记录。

## 日常巡检

- 确认 PostgreSQL 可连接，磁盘空间充足。
- 确认对象存储 bucket 可访问。
- 确认后端日志持续写入。
- 确认备份任务按保留策略产生可用备份。
- 检查登录失败次数是否异常升高。
- 检查评论待审核队列是否堆积。

## 常见故障处理

### 后端健康检查失败

1. 检查服务进程或容器状态。
2. 检查 `DATABASE_URL` 是否可连接。
3. 检查 `JWT_SECRET`、`FRONTEND_ORIGINS` 和 S3 配置是否存在。
4. 查看后端日志文件。

### 登录失败次数异常

1. 检查 `login_attempts` 是否存在短时间高频失败。
2. 确认是否触发渐进式封禁。
3. 如来源异常，应在反向代理或网络层增加临时封禁。

### 文件下载失败

1. 检查 `assets.storage_key` 是否存在。
2. 检查 S3 凭据和 bucket 权限。
3. 检查资源状态是否为 `active` 或允许下载的状态。
4. 检查 `download_events` 是否能写入。

## 关联文档

- `docs/operations/backup.md`
- `docs/operations/restore.md`
- `docs/operations/deployment.md`
- `docs/operations/observability.md`
