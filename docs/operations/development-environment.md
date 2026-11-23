# WSL2 Ubuntu 24.04 开发环境配置指南

## 当前项目环境

当前项目明确运行在 WSL2 Ubuntu 24.04 环境中，仓库路径为：

```text
/home/ubuntu/workspace/rendering-cms-platform
```

开发、前端启动、依赖服务、测试与数据库操作均在 WSL2 Ubuntu 24.04 终端执行。

## 环境检查口径

在仓库根目录检查当前工具。

```bash
cd /home/ubuntu/workspace/rendering-cms-platform
bash scripts/env/check-env.sh
```

脚本只检查工具，不安装软件。检查项和项目要求如下：

| 工具 | 项目要求 | 说明 |
| --- | --- | --- |
| Go | Go 1.26.11，与 backend/go.mod 一致 | 用于本机后端开发和 CLI 工具安装；也可以使用 Docker Go 容器执行测试。 |
| Node.js | 推荐 24，与 CI 一致；镜像构建和开发容器使用 22 | 用于本机前端开发；也可以使用 Docker Node 容器执行构建。 |
| npm | npm 10+ | 与 Node.js 配套使用。 |
| Docker | Docker Engine 或 Docker Desktop WSL integration | 用于依赖服务、后端容器、前端容器和生产镜像验证。 |
| Docker Compose | Docker Compose v2 | 使用 `docker compose`，不使用旧版 `docker-compose`。 |
| PostgreSQL client `psql` | PostgreSQL client 16+ | 用于本机连接和 SQL 验证。 |
| `sqlc` | sqlc v1.31.1，与 CI 一致 | 用于生成 Go 数据访问代码。 |
| `migrate` | golang-migrate CLI | 用于执行 PostgreSQL migration。 |

## 必需工具

### Go

用途：

- 编译 `backend/`。
- 运行受改动影响的 Go 包测试。
- 安装 `sqlc` 和 `migrate` 等 Go CLI。

安装示例：

```bash
sudo apt-get update
sudo apt-get install -y golang-go
go version
```

Go 工具链与 backend/go.mod 和构建镜像保持一致；系统包不足时使用官方安装包、版本管理工具或项目固定版本的 Docker 镜像。自动化终端应确认 PATH 指向 Linux Go 与 Node.js，避免调用 WSL 继承的 Windows npm。

### Node.js 和 npm

用途：

- 创建 `frontend/`。
- 本地或 Docker 容器内运行 Vite 前端。
- 执行 `npm run build`。

安装示例：

```bash
sudo apt-get update
sudo apt-get install -y nodejs npm
node --version
npm --version
```

系统包版本不足时，使用 Linux 版本管理工具安装满足 frontend/package-lock.json 中工具要求的 Node.js；CI 使用 Node.js 24。

### Docker 和 Docker Compose

用途：

- 启动 PostgreSQL。
- 启动 MinIO，模拟 S3 兼容对象存储。
- 可选：用 Docker 启动前端 Vite 开发服务。

检查命令：

```bash
docker --version
docker compose version
```

### PostgreSQL Client

用途：

- 使用 `psql` 检查本地数据库连接。
- 执行 SQL 验证。
- 备份和恢复时使用 `pg_dump` 与 `psql`。

安装示例：

```bash
sudo apt-get update
sudo apt-get install -y postgresql-client
psql --version
```

### sqlc

用途：

- 从 `backend/sql/*.sql` 和 `backend/migrations/*.sql` 生成类型安全 Go 数据访问代码。

安装示例：

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
export PATH="$HOME/go/bin:$PATH"
sqlc version
```

### golang-migrate

用途：

- 执行 PostgreSQL migration。

安装示例：

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.3
export PATH="$HOME/go/bin:$PATH"
migrate -version
```

## 环境变量

已有 `.env` 时保留并检查其中的本地配置。首次开发可从 [环境模板](../../scripts/env/dev.env.example)创建。

```bash
test -f .env || cp scripts/env/dev.env.example .env
```

`.env` 属于本地文件。填写本地数据库、MinIO、JWT 和开发管理员变量，确保 `DATABASE_URL` 与本地 PostgreSQL 一致。CLI 需要读取该文件时，先确认它是合法的 Bash 赋值文件，带空格的值须加引号。测试用的私有参数留在文件中。

本地后端在 `APP_ENV=development` 时创建或更新由 `DEV_ADMIN_EMAIL`、`DEV_ADMIN_NAME`、`DEV_ADMIN_PASSWORD` 指定的管理员。后端直接在 WSL 运行时，模板启用 `LOGIN_MFA_ENABLED` 与 `LOGIN_MFA_DEV_LOG`，可从后端日志取得验证码。默认开发 Compose 未向后端传入 `LOGIN_MFA_*` 或 `SMTP_*`，需要在 WSL 直启后端或使用本地 Compose 覆盖文件配置两步验证。生产启动规则见 [认证 API](../apis/auth.md)。

PostBridge 的可选连接变量和密钥文件见 [PostBridge 接入 API](../apis/postbridge.md#后端配置)。开发 Compose 使用默认相册前缀 `photos`，自定义 `S3_PHOTO_PREFIX` 需要传入运行环境。默认 MinIO bucket 为私有，公开图链接联调需在本地单独配置对应前缀的读权限。

后端容器内的 `LOG_DIR` 通过 Docker bind mount 写入宿主机目录。本地默认目录为 `logs/backend`，位于项目目录下，已被 `.gitignore` 排除，不会提交到 Git。如需挂载到其他系统盘路径，可在启动脚本前导出 `BACKEND_LOG_HOST_DIR`：

```bash
export BACKEND_LOG_HOST_DIR=/var/log/rendering-cms-platform
bash scripts/env/start-backend-docker.sh
```

### 同步浏览器访问地址

当前后端、前端、PostgreSQL 和 MinIO 都通过 Docker 端口映射访问，Windows 浏览器和 WSL 内部通常可使用 `127.0.0.1`。表中的回环地址用于本机访问，映射配置以 Compose 文件为准。因此 `.env` 中面向浏览器的地址默认写成本机地址。

脚本：

```bash
bash scripts/env/sync-wsl-network-env.sh
```

脚本默认使用 `BROWSER_HOST=127.0.0.1`。如果某个环境中 Windows localhost 转发不可用，可以显式设置 `BROWSER_HOST=wsl`，此时脚本会优先使用已导出的 `WSL_IP` 或 `WIN_IP`，否则自动读取 WSL `eth0` 地址。脚本会更新 `.env` 中的以下变量：

```env
FRONTEND_ORIGIN=http://127.0.0.1:5173
FRONTEND_ORIGINS=http://127.0.0.1:3000,http://127.0.0.1:5173,http://localhost:3000,http://localhost:5173
VITE_API_BASE=http://127.0.0.1:8080/api/v1
S3_ENDPOINT=http://127.0.0.1:9000
S3_USE_PATH_STYLE=true
S3_PUBLIC_BASE_URL=http://127.0.0.1:9000/rendering-assets
MINIO_API_CORS_ALLOW_ORIGIN=http://127.0.0.1:3000,http://127.0.0.1:5173,http://localhost:3000,http://localhost:5173
```

脚本不会修改：

- `HTTP_ADDR`：后端仍应监听 `0.0.0.0:8080`。
- `DATABASE_URL`：后端在 WSL 内连接 PostgreSQL，仍使用 `127.0.0.1:5432`。

`FRONTEND_ORIGINS` 是后端 CORS 白名单，`MINIO_API_CORS_ALLOW_ORIGIN` 是 MinIO 直传对象存储的 CORS 白名单，多个地址都使用英文逗号分隔。`S3_USE_PATH_STYLE=true` 仅用于本地 MinIO；生产 Cloudflare R2 必须设置为 `false`。`S3_PUBLIC_BASE_URL` 用于文章图片公开访问 URL，和上传用的 `S3_ENDPOINT` 分开配置。`FRONTEND_ORIGIN` 保留给旧配置兼容；如果同时设置两者，后端优先读取 `FRONTEND_ORIGINS`。

`start-backend-docker.sh` 和 `start-frontend-docker.sh` 会在启动容器前自动执行该同步脚本。如果确实需要使用 WSL 真实 IP，执行启动脚本前先设置：

```bash
BROWSER_HOST=wsl bash scripts/env/start-dev-stack.sh
```

## Docker 服务

Docker Compose 配置文件：

```text
scripts/env/docker-compose.dev.yml
```

服务列表：

| 服务 | 说明 | 端口 |
| --- | --- | --- |
| `postgres` | 本地 PostgreSQL 数据库 | `127.0.0.1:5432` |
| `minio` | 本地 S3 兼容对象存储及控制台 | `127.0.0.1:9000`、`127.0.0.1:9001` |
| `minio-init` | 初始化 MinIO bucket | 无对外端口 |
| `backend` | Go API 开发服务，使用 `golang:1.26.11-alpine` | `127.0.0.1:8080` |
| `frontend` | Vite 前端开发服务，使用 `node:22-alpine` | `127.0.0.1:5173` |

`backend` 和 `frontend` 使用 Docker Compose profile，不会随前置依赖服务自动启动。这样可以先启动数据库和 MinIO，再按需要单独启动后端或前端。

## 脚本说明

### 环境检查

```bash
bash scripts/env/check-env.sh
```

脚本只检查工具，不安装软件。检查项：

- Go
- Node.js
- npm
- Docker
- Docker Compose
- psql
- sqlc
- migrate

### 启动前置依赖服务

```bash
bash scripts/env/start-prerequisites.sh
```

启动：

- `postgres`
- `minio`
- `minio-init`

这是后端开发、migration、文件上传下载开发前必须先启动的服务。

`start-prerequisites.sh` 只负责启动 PostgreSQL、MinIO 和 bucket 初始化，不启动后端或前端。

### 启动后端 Docker 服务

```bash
bash scripts/env/start-backend-docker.sh
```

前提是已配置 `.env`、Docker 可用，且已完成本地数据库迁移。

该脚本会先同步 WSL 浏览器访问地址，再确保 PostgreSQL、MinIO 和 `minio-init` 已启动，最后启动后端 Docker 服务，监听：

```text
http://127.0.0.1:8080
```

如果显式使用 `BROWSER_HOST=wsl`，则按脚本写入 `.env` 的 WSL IP 访问。

### 启动前端 Docker 服务

```bash
bash scripts/env/start-frontend-docker.sh
```

前提是已配置 `.env` 且 Docker 可用。

该脚本通过 Docker 运行 Vite 前端服务，监听：

```text
http://127.0.0.1:5173
```

如果显式使用 `BROWSER_HOST=wsl`，则按脚本写入 `.env` 的 WSL IP 访问。

### 启动完整开发栈

```bash
bash scripts/env/start-dev-stack.sh
```

执行顺序：

1. 运行 `start-backend-docker.sh`，启动 PostgreSQL、MinIO、`minio-init` 和后端。
2. 运行 `start-frontend-docker.sh`，启动前端。

### 停止开发服务

```bash
bash scripts/env/stop-dev-services.sh
```

停止 Compose 中的前置依赖服务、后端 profile 服务和前端 profile 服务。

AI 草稿协作与发布按 [MCP 本地接入](article-mcp.md#本地启动)配置和验证。

## 推荐开发启动顺序

1. 在 WSL 中进入仓库根目录，运行环境检查并准备 `.env`。
2. 启动 PostgreSQL 和 MinIO。确认 PostgreSQL 可连接后，在仓库根目录加载本地环境变量并执行 migration。

   ```bash
   bash scripts/env/start-prerequisites.sh
   docker compose --env-file .env -f scripts/env/docker-compose.dev.yml up -d --wait postgres
   (
     set -e
     set -a
     source ./.env
     set +a
     migrate -path backend/migrations -database "$DATABASE_URL" up
   )
   ```

3. 运行 `bash scripts/env/start-dev-stack.sh` 启动后端和前端。脚本只启动服务，数据库迁移使用上一步的命令。
4. 用 `curl -fsS http://127.0.0.1:8080/api/v1/health` 检查后端，打开 `http://127.0.0.1:5173` 并用本地管理员登录。
5. 按改动范围运行测试。前端改动执行 `npm run build`。Go、SQL 和浏览器回归入口见 [后端](../../backend/README.md)与 [前端](../../frontend/README.md)。
6. 结束时执行 `bash scripts/env/stop-dev-services.sh` 清理本次启动的 Compose 服务，并结束其它临时进程。

## 注意事项

- 本项目运行环境以 WSL2 Ubuntu 24.04 为准。
- 任务结束前关闭本次临时开启的服务、浏览器、辅助进程及容器；保留任务开始前已存在的服务。
- 不维护 PowerShell 环境脚本。
- 不要把 `.env` 提交到 Git。
- 不要把上传文件写入 Git 或 `frontend/public/`。
- 本地使用 MinIO，生产使用 Cloudflare R2。生产域名、凭据和备份按 [部署手册](deployment.md)维护。
- 生产 migration 前必须先备份 PostgreSQL。

## 部署脚本隔离验证

迁移部署脚本的针对性测试入口见 [部署收尾](deploy-readiness.md#本地检查与环境证据)。测试在 temp 内运行 Git 和外部工具替身，不启动数据库或容器。修改部署脚本不要求在本地重复前后端全量构建，完整应用验证交由 GitHub CI。
