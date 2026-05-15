# MVP 阶段 03：登录认证和后台保护

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

实现管理员登录基础能力，提供密码哈希、token 签发校验和后台接口保护中间件，确保后台 API 不会无认证暴露。

## 文件范围

- Create: `backend/internal/auth/password.go`
- Create: `backend/internal/auth/password_test.go`
- Create: `backend/internal/auth/token.go`
- Create: `backend/internal/auth/token_test.go`
- Create: `backend/internal/auth/handler.go`
- Create: `backend/internal/http/middleware.go`
- Create: `docs/apis/auth.md`

## 子任务

- [x] 在 `password_test.go` 编写 `TestPasswordHashAndVerify`，验证正确密码通过、错误密码失败。
- [x] 运行 `cd backend && go test ./internal/auth`，确认测试因函数未定义失败。
- [x] 在 `password.go` 实现 `HashPassword(password string) (string, error)`。
- [x] 在 `password.go` 实现 `VerifyPassword(hash string, password string) bool`。
- [x] 在 `token_test.go` 编写 `TestIssueAndParseToken`，验证 `userId` 和 `role` 可从 token 解析。
- [x] 在 `token.go` 定义 `Claims`，字段包含 `UserID`、`Role` 和 `jwt.RegisteredClaims`。
- [x] 在 `token.go` 实现 `IssueToken(secret, userID, role string)`，默认过期时间 24 小时。
- [x] 在 `token.go` 实现 `ParseToken(secret, raw string)`。
- [x] 在 `handler.go` 定义 `POST /api/v1/auth/login` 的请求和响应结构。
- [x] 在 `backend/internal/http/middleware.go` 实现后台认证中间件，要求 Bearer token 有效。
- [x] 将后台 API 约束为 `admin` 或 `editor` 角色可访问。
- [x] 创建 `docs/apis/auth.md`，记录登录、登出和认证头规则。
- [x] 运行 `cd backend && go test ./internal/auth`。

## 完成记录

- 完成时间：2026-04-30。
- 密码哈希使用 bcrypt。
- JWT token 使用 HS256，携带 `userId` 和 `role`，默认有效期 24 小时。
- 后台认证中间件要求 Bearer token，并限制 `admin`、`editor` 角色。
- `POST /api/v1/auth/login` 已接入数据库用户查询和密码校验。
- 验证命令已通过：`cd backend && go test ./internal/auth ./internal/http ./...`。

## 验收标准

- 密码必须使用 bcrypt。
- token 必须携带用户 ID 和角色。
- 后台路由必须经过认证中间件。
- 认证 API 文档使用中文 Markdown。

## 建议提交

```bash
git add backend/internal/auth backend/internal/http/middleware.go docs/apis/auth.md
git commit -m "feat: add admin authentication"
```
