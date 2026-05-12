# WSL2 Ubuntu 24.04 开发环境配置指南

## 当前项目环境

检查时间：2026-04-30。

当前项目明确运行在 WSL2 Ubuntu 24.04 环境中，仓库路径为：

```text
/home/administrator/workspace/rendering-cms-platform
```

本项目后续开发、依赖服务启动、前端启动、后端测试和数据库操作都默认在 WSL2 Ubuntu 24.04 终端中执行。项目不再维护 PowerShell 启动脚本。

## 当前工具状态

| 工具 | 当前状态 | 项目要求 |
| --- | --- | --- |
| Go | 未安装 | Go 1.22+ |
| Node.js | 未安装 | Node.js 20+，推荐 22 LTS |
| npm | 未安装 | npm 10+ |
| Docker | 未安装 | Docker Engine 或 Docker Desktop WSL integration |
| Docker Compose | 未安装 | Docker Compose v2 |
| PostgreSQL client `psql` | 未安装 | PostgreSQL client 16+ |
| `sqlc` | 未安装 | sqlc 1.25+ |
| `migrate` | 未安装 | golang-migrate CLI |

## 必需工具

### Go

用途：

- 编译 `backend/`。
- 运行 `go test ./...`。
- 安装 `sqlc` 和 `migrate` 等 Go CLI。

安装示例：

```bash
sudo apt-get update
sudo apt-get install -y golang-go
go version
```

如果 apt 源中的 Go 版本低于 1.22，使用 Go 官方安装包或版本管理工具安装。

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

如果 apt 源版本过低，使用 NodeSource、nvm 或 fnm 安装 Node.js 20+。

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
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
export PATH="$HOME/go/bin:$PATH"
sqlc version
```

### golang-migrate

用途：

- 执行 PostgreSQL migration。

安装示例：

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
export PATH="$HOME/go/bin:$PATH"
migrate -version
```

## 环境变量

复制示例配置：

```bash
cp scripts/env/dev.env.example .env
```

MVP 本地开发变量：

```env
HTTP_ADDR=0.0.0.0:8080
DATABASE_URL=postgres://rendering:rendering_dev_password@127.0.0.1:5432/rendering_cms?sslmode=disable
JWT_SECRET=replace-with-32-plus-character-secret
FRONTEND_ORIGIN=http://127.0.0.1:5173
FRONTEND_ORIGINS=http://127.0.0.1:3000,http://127.0.0.1:5173
VITE_API_BASE=http://127.0.0.1:8080/api/v1
LOG_DIR=/var/log/rendering-cms-platform

S3_ENDPOINT=http://127.0.0.1:9000
S3_REGION=us-east-1
S3_BUCKET=rendering-assets
S3_ACCESS_KEY_ID=rendering
S3_SECRET_ACCESS_KEY=rendering_dev_password

POSTGRES_DB=rendering_cms
POSTGRES_USER=rendering
POSTGRES_PASSWORD=rendering_dev_password

MINIO_ROOT_USER=rendering
MINIO_ROOT_PASSWORD=rendering_dev_password
MINIO_BUCKET=rendering-assets
```

不要提交 `.env`。

后端容器内的 `LOG_DIR` 通过 Docker bind mount 写入宿主机目录。本地默认目录为 `logs/backend`，位于项目目录下，已被 `.gitignore` 排除，不会提交到 Git。如需挂载到其他系统盘路径，可在启动脚本前导出 `BACKEND_LOG_HOST_DIR`：

```bash
export BACKEND_LOG_HOST_DIR=/var/log/rendering-cms-platform
bash scripts/env/start-backend-docker.sh
```

### 同步浏览器访问地址

当前后端、前端、PostgreSQL 和 MinIO 都通过 Docker 端口映射暴露到宿主机，Windows 浏览器和 WSL 内部默认都可以使用 `127.0.0.1` 访问。因此 `.env` 中面向浏览器的地址默认写成本机地址。

脚本：

```bash
bash scripts/env/sync-wsl-network-env.sh
```

脚本默认使用 `BROWSER_HOST=127.0.0.1`。如果某个环境中 Windows localhost 转发不可用，可以显式设置 `BROWSER_HOST=wsl`，此时脚本会优先使用已导出的 `WSL_IP` 或 `WIN_IP`，否则自动读取 WSL `eth0` 地址。脚本会更新 `.env` 中的以下变量：

```env
FRONTEND_ORIGIN=http://127.0.0.1:5173
FRONTEND_ORIGINS=http://127.0.0.1:3000,http://127.0.0.1:5173
VITE_API_BASE=http://127.0.0.1:8080/api/v1
S3_ENDPOINT=http://127.0.0.1:9000
```

脚本不会修改：

- `HTTP_ADDR`：后端仍应监听 `0.0.0.0:8080`。
- `DATABASE_URL`：后端在 WSL 内连接 PostgreSQL，仍使用 `127.0.0.1:5432`。

`FRONTEND_ORIGINS` 是后端 CORS 白名单，多个地址使用英文逗号分隔。`FRONTEND_ORIGIN` 保留给旧配置兼容；如果同时设置两者，后端优先读取 `FRONTEND_ORIGINS`。

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
| `minio` | 本地 S3 兼容对象存储 | `127.0.0.1:9000` |
| `minio-init` | 初始化 MinIO bucket | 无对外端口 |
| `backend` | Go API 开发服务，使用 `golang:1.26-alpine` | `127.0.0.1:8080` |
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

前提：

- 已创建 `backend/go.mod`。
- 已复制 `.env`。
- Docker 可用。

该脚本会先同步 WSL 浏览器访问地址，再确保 PostgreSQL、MinIO 和 `minio-init` 已启动，最后启动后端 Docker 服务，监听：

```text
http://127.0.0.1:8080
```

如果显式使用 `BROWSER_HOST=wsl`，则按脚本写入 `.env` 的 WSL IP 访问。

### 启动前端 Docker 服务

```bash
bash scripts/env/start-frontend-docker.sh
```

前提：

- 已创建 `frontend/package.json`。
- 已复制 `.env`。
- Docker 可用。

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

## 推荐开发启动顺序

1. 安装 Go、Node.js、npm、Docker、psql、sqlc、migrate。
2. 运行 `bash scripts/env/check-env.sh`。
3. 复制 `scripts/env/dev.env.example` 为 `.env`。
4. 运行 `bash scripts/env/start-backend-docker.sh` 启动依赖服务和后端。
5. 在 `backend/` 下执行 migration。
6. 在 `backend/` 下运行 `go test ./...`。
7. 运行 `bash scripts/env/start-frontend-docker.sh` 启动前端，或在 `frontend/` 下直接运行 `npm run dev -- --host 0.0.0.0`。
8. 运行 `bash scripts/env/start-dev-stack.sh` 验证依赖服务、后端和前端可一起启动。

## 注意事项

- 本项目运行环境以 WSL2 Ubuntu 24.04 为准。
- 不维护 PowerShell 环境脚本。
- 不要把 `.env` 提交到 Git。
- 不要把上传文件写入 Git 或 `frontend/public/`。
- 开发环境 MinIO 只用于本地联调，不等同于生产 MinIO 的 HTTPS 域名、凭据和备份模型。
- 生产 migration 前必须先备份 PostgreSQL。
