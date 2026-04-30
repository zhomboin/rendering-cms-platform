# 认证 API

本文档记录 MVP 阶段后台认证接口和后台 API 访问规则。所有接口前缀为 `/api/v1`。

## 登录

```http
POST /api/v1/auth/login
Content-Type: application/json
```

请求体：

```json
{
  "email": "admin@example.com",
  "password": "your-password"
}
```

成功响应：

```json
{
  "token": "jwt-token",
  "user": {
    "userId": "uuid",
    "email": "admin@example.com",
    "name": "管理员",
    "role": "admin"
  }
}
```

失败响应：

```json
{
  "error": "邮箱或密码错误"
}
```

状态码：

- `200 OK`：登录成功。
- `400 Bad Request`：请求体格式不正确或邮箱、密码为空。
- `401 Unauthorized`：邮箱或密码错误。
- `403 Forbidden`：用户不是 `admin` 或 `editor`。
- `500 Internal Server Error`：令牌生成失败。

## 后台认证规则

后台 API 必须携带 Bearer token：

```http
Authorization: Bearer <jwt-token>
```

token 规则：

- 使用 HS256 签名。
- 签名密钥来自 `JWT_SECRET`。
- token 有效期默认为 24 小时。
- token claims 必须包含 `userId` 和 `role`。
- 允许访问后台 API 的角色为 `admin` 和 `editor`。

## 登出

MVP 当前使用 Bearer token。前端登出时删除本地保存的 token 即可。

后续如改为安全 Cookie Session，应新增：

```http
POST /api/v1/auth/logout
```

并由后端清理登录态 Cookie。
