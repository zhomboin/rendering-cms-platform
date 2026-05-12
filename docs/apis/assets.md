# 文件 API

本文档记录 MVP 阶段后台资源上传、资源列表、下载链接和下载审计接口。所有接口前缀为 `/api/v1`。

## 约束

- 文件内容只进入 S3 兼容对象存储，不写入 Git 仓库或前端 `public/` 目录。
- 所有资源 API 都需要后台认证。
- 下载链接生成时必须写入 `download_events`。
- 下载审计只保存 IP 哈希，不保存原始 IP。

## 允许上传类型

- `image/png`
- `image/jpeg`
- `image/webp`
- `application/pdf`
- `text/plain`
- `application/zip`

最大文件大小：`20MB`。

## 资源模型

```json
{
  "assetId": "uuid",
  "filename": "diagram.webp",
  "contentType": "image/webp",
  "byteSize": 384000,
  "publicUrl": null,
  "createdBy": "uuid",
  "createdAt": "2026-04-30T00:00:00Z",
  "status": "active",
  "deletedAt": null
}
```

## 资源列表

```http
GET /api/v1/admin/assets
Authorization: Bearer <jwt-token>
```

说明：

- 返回后台已创建的资源元数据。
- 按上传时间倒序排列。

## 申请预签名上传 URL

```http
POST /api/v1/admin/assets/upload-url
Authorization: Bearer <jwt-token>
Content-Type: application/json
```

请求体：

```json
{
  "filename": "diagram.webp",
  "contentType": "image/webp",
  "byteSize": 384000
}
```

响应：

```json
{
  "asset": {
    "assetId": "uuid",
    "filename": "diagram.webp",
    "contentType": "image/webp",
    "byteSize": 384000,
    "publicUrl": null,
    "createdBy": "uuid",
    "createdAt": "2026-04-30T00:00:00Z",
    "status": "active",
    "deletedAt": null
  },
  "uploadUrl": "https://object-storage/presigned-url",
  "method": "PUT",
  "headers": {
    "Content-Type": "image/webp"
  },
  "expiresInSeconds": 900
}
```

说明：

- 后端先校验文件名、类型和大小，再写入 `assets` 记录。
- 客户端使用返回的 `uploadUrl` 和 `headers` 直接上传到对象存储。

## 申请预签名下载 URL

```http
GET /api/v1/admin/assets/{id}/download-url
Authorization: Bearer <jwt-token>
```

响应：

```json
{
  "asset": {
    "assetId": "uuid",
    "filename": "diagram.webp",
    "contentType": "image/webp",
    "byteSize": 384000,
    "publicUrl": null,
    "createdBy": "uuid",
    "createdAt": "2026-04-30T00:00:00Z",
    "status": "active",
    "deletedAt": null
  },
  "downloadUrl": "https://object-storage/presigned-url",
  "expiresInSeconds": 900
}
```

说明：

- 每次生成下载 URL 都写入 `download_events`。
- 下载审计只记录 `asset_id`、`ip_hash`、`user_agent` 和时间。

## 更新资源状态

```http
PATCH /api/v1/admin/assets/{id}
Authorization: Bearer <jwt-token>
Content-Type: application/json
```

请求体：

```json
{
  "status": "deleted"
}
```

说明：

- 更新资源状态，允许值为 `active`、`archived`、`deleted`。
- 资源删除采用软删除，设置 `status=deleted` 和 `deleted_at`，不立即删除对象存储文件。
- 状态改回 `active` 或 `archived` 时清空 `deleted_at`。
