# Backend Agent Guide

本文件约束 `backend/` 目录下的后端开发工作。根目录 `AGENTS.md` 的文档、环境、安全和项目边界约束仍然有效。

## 开发环境

- 所有后端命令默认在 WSL2 Ubuntu 24.04 中执行。
- 命令示例必须使用 Linux/Bash 形式，不使用 PowerShell、CMD 或 Windows 路径。
- 项目路径示例使用 `/home/ubuntu/workspace/rendering-cms-platform/backend`。
- 后续执行后端测试、生成代码、启动服务等项目命令时默认直接执行；破坏性操作、系统级配置修改、安装系统依赖或访问受限凭据除外。

## 技术边界

- 后端使用 Go，HTTP 路由优先使用 Chi。
- 数据库访问使用 `pgx`、`pgxpool` 和 `sqlc`。
- PostgreSQL 是唯一运行时业务数据源，不要用 JSON 文件保存文章、评论、统计或文件元数据。
- 文件上传下载只保存元数据和对象存储 key，不把上传文件写入 Git，也不写入前端 `public/`。
- 后台 API 必须使用 `/api/v1` 前缀。
- 评论、下载审计、访问统计不得保存原始 IP；如需识别访问端，只保存哈希值。

## 目录职责

- `cmd/server/`：服务启动入口，只组合配置、路由和运行时依赖。
- `internal/config/`：环境变量读取和配置校验。
- `internal/http/`：路由、中间件、HTTP handler 入口。
- `internal/database/`：数据库连接封装和 sqlc 生成代码。
- `internal/database/dbgen/`：由 `sqlc generate` 生成，不手工编辑。
- `migrations/`：PostgreSQL migration，必须提供 up/down。
- `sql/`：sqlc 查询文件，按业务域拆分。

## 编码规则

- 新增 Go 文件必须通过 `gofmt`。
- 新增业务逻辑优先写测试，再实现。
- handler 不应直接拼接 SQL；数据库查询应进入 `sql/` 并通过 sqlc 生成。
- 配置项从 `internal/config` 读取，不在业务代码中散落 `os.Getenv`。
- 错误响应、认证、权限和审计逻辑后续应集中封装，不在各 handler 内重复实现。
- 不要手工修改 `internal/database/dbgen/` 下的生成文件；需要调整时修改 `migrations/` 或 `sql/` 后重新运行 `sqlc generate`。

## 数据库规则

- schema 变更必须新增 migration，不直接修改已发布 migration，除非当前 migration 尚未进入共享环境且用户明确同意。
- down migration 必须按依赖关系反向删除对象。
- 生产 migration 前必须备份 PostgreSQL。
- 新增表、字段、枚举或索引后，同步更新 `docs/sql/schema.md`。
- 涉及接口行为的数据库变更，同步更新 `docs/apis/`。

## 验证命令

后端常规验证：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
go test ./...
```

涉及 SQL 或 migration 时：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
sqlc generate
go test ./...
```

必要时增加：

```bash
go vet ./...
```
