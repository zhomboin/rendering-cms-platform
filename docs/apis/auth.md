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
  "refreshToken": "refresh-jwt-token",
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
- `429 Too Many Requests`：登录失败次数过多，账号或来源被临时锁定。
- `500 Internal Server Error`：令牌生成失败。

## 刷新登录态

```http
POST /api/v1/auth/refresh
Content-Type: application/json
```

请求体：

```json
{
  "refreshToken": "refresh-jwt-token"
}
```

成功响应：

```json
{
  "token": "new-jwt-token",
  "refreshToken": "new-refresh-jwt-token"
}
```

状态码：

- `200 OK`：刷新成功，前端应使用响应中的新 `token` 和 `refreshToken` 覆盖本地旧值。
- `400 Bad Request`：请求体格式不正确或 `refreshToken` 为空。
- `401 Unauthorized`：`refreshToken` 无效或已过期。
- `403 Forbidden`：用户角色不允许访问后台。
- `500 Internal Server Error`：令牌生成失败。

## 登录安全规则

- 登录尝试记录在 `login_attempts` 表，用于审计和防止爆破登录。
- 登录尝试按邮箱和 IP 哈希两个维度检查，任一维度触发规则都会拒绝登录。
- 5 分钟内失败 5 次及以上，锁定 5 分钟。
- 15 分钟内失败 10 次及以上，锁定 15 分钟。
- 1 小时内失败 20 次及以上，锁定 1 天。
- 多档规则同时命中时使用最长锁定结果。
- 锁定期间返回 `429 Too Many Requests`，不校验密码。
- 失败登录统一返回 `邮箱或密码错误`，避免枚举账号。
- 不保存原始 IP 地址，只保存 IP 哈希。

## 后台认证规则

后台 API 必须携带 Bearer token：

```http
Authorization: Bearer <jwt-token>
```

token 规则：

- 使用 HS256 签名。
- 签名密钥来自 `JWT_SECRET`。
- access token 有效期默认为 2 小时。
- refresh token 有效期默认为 7 天，仅用于 `/api/v1/auth/refresh`，不能访问后台 API。
- token claims 必须包含 `userId`、`role` 和 `tokenType`。
- 允许访问后台 API 的角色为 `admin` 和 `editor`。
- 前端在后台 API 返回 `401 Unauthorized` 时，会优先使用 refresh token 无感刷新并重放原请求；refresh token 刷新失败后才清理登录态并跳转登录页。

## 登出

MVP 当前使用 Bearer token。前端登出时删除本地保存的 token 即可。

后续如改为安全 Cookie Session，应新增：

```http
POST /api/v1/auth/logout
```

并由后端清理登录态 Cookie。
