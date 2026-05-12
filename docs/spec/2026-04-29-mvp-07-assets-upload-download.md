# MVP 阶段 07：文件上传、下载和审计

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this spec task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

## 目标

实现后台文件上传 URL 申请、对象存储直传、下载 URL 申请和下载审计，禁止把上传文件写入 Git 或前端 `public/`。

## 文件范围

- Create: `backend/internal/assets/service.go`
- Create: `backend/internal/assets/service_test.go`
- Create: `backend/internal/assets/handler.go`
- Create: `backend/internal/storage/s3.go`
- Modify: `backend/sql/assets.sql`
- Create: `frontend/src/pages/assets/AssetsPage.tsx`
- Create: `docs/apis/assets.md`

## 子任务

- [x] 在 `service_test.go` 编写 `TestValidateUpload`。
- [x] 在 `service.go` 定义 `MaxUploadBytes = 20 * 1024 * 1024`。
- [x] 在 `service.go` 定义允许类型：`image/png`、`image/jpeg`、`image/webp`、`application/pdf`、`text/plain`、`application/zip`。
- [x] 在 `service.go` 实现 `ValidateUpload(filename, contentType string, byteSize int) error`。
- [x] 在 `storage/s3.go` 封装 S3 兼容客户端创建逻辑。
- [x] 在 `storage/s3.go` 实现预签名上传 URL 方法。
- [x] 在 `storage/s3.go` 实现预签名下载 URL 方法。
- [x] 在 `backend/sql/assets.sql` 增加创建 `assets` 记录 SQL。
- [x] 在 `backend/sql/assets.sql` 增加查询 `assets` 记录 SQL。
- [x] 在 `backend/sql/assets.sql` 增加创建 `download_events` 记录 SQL。
- [x] 在 `handler.go` 实现 `POST /api/v1/admin/assets/upload-url`。
- [x] 在 `handler.go` 实现 `GET /api/v1/admin/assets/{id}/download-url`。
- [x] 创建 `AssetsPage.tsx`，包含上传表单、资源列表、下载按钮区域。
- [x] 创建 `docs/apis/assets.md`。
- [x] 运行 `cd backend && go test ./internal/assets`。
- [x] 运行 `cd frontend && npm run build`。

## 验收标准

- 上传文件类型和大小必须被校验。
- 文件内容不进入 Git 和前端 `public/`。
- 下载 URL 生成时必须写入 `download_events`。
- 资源 API 必须要求后台认证。

## 建议提交

```bash
git add backend/internal/assets backend/internal/storage backend/sql/assets.sql frontend/src/pages/assets docs/apis/assets.md
git commit -m "feat: add asset upload download audit"
```

## 完成记录

- 实现上传类型和大小校验，最大 `20MB`。
- 实现 S3 兼容对象存储预签名上传和下载 URL。
- 下载 URL 生成时写入 `download_events`，仅保存 IP 哈希。
- 后台资源页面接入资源列表、上传和下载接口。
