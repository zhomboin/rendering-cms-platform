# 认证 API

本文档记录后台认证接口和后台 API 访问规则。所有接口前缀为 `/api/v1`。

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

密码验证成功且无风险信号时的成功响应：

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

密码验证成功但命中邮箱验证码触发条件时，返回验证码挑战响应（不包含任何登录令牌）：

```json
{
  "mfaRequired": true,
  "challengeToken": "mfa-challenge-jwt-token",
  "email": "admin@example.com",
  "expiresIn": 600
}
```

前端应凭 `challengeToken` 引导用户输入邮箱收到的 6 位数字验证码，再调用邮箱验证码校验接口完成登录。

失败响应：

```json
{
  "error": "邮箱或密码错误"
}
```

状态码：

- `200 OK`：登录成功，或需要邮箱验证码（以响应体是否包含 `mfaRequired` 区分）。
- `400 Bad Request`：请求体格式不正确或邮箱、密码为空。
- `401 Unauthorized`：邮箱或密码错误。
- `403 Forbidden`：用户不是 `admin` 或 `editor`。
- `429 Too Many Requests`：登录失败次数过多账号或来源被临时锁定，或验证码发送超过频率限制。
- `500 Internal Server Error`：令牌生成、验证码邮件发送或登录安全检查失败。

## 邮箱验证码校验

```http
POST /api/v1/auth/mfa/verify
Content-Type: application/json
```

请求体：

```json
{
  "challengeToken": "mfa-challenge-jwt-token",
  "code": "123456"
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
  "error": "验证码错误或已过期"
}
```

状态码：

- `200 OK`：验证码校验通过，登录完成，响应结构与登录成功一致。
- `400 Bad Request`：请求体格式不正确或 `challengeToken`、`code` 为空。
- `401 Unauthorized`：验证码错误、已过期、尝试次数用尽或已使用；`challengeToken` 无效或已过期时返回 `登录验证已失效，请重新登录`。
- `500 Internal Server Error`：登录安全检查或令牌生成失败。

## 邮箱验证码重发

```http
POST /api/v1/auth/mfa/resend
Content-Type: application/json
```

请求体：

```json
{
  "challengeToken": "mfa-challenge-jwt-token"
}
```

成功响应：

```json
{
  "expiresIn": 600
}
```

状态码：

- `200 OK`：旧验证码已作废，新验证码已发送，`expiresIn` 为新验证码有效秒数。
- `400 Bad Request`：请求体格式不正确或 `challengeToken` 为空。
- `401 Unauthorized`：`challengeToken` 无效或已过期。
- `429 Too Many Requests`：距上次发送不足 60 秒，或该邮箱一小时内发送数量达到上限（5 封）。
- `500 Internal Server Error`：验证码邮件发送或登录安全检查失败。

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

## 邮箱验证码规则

- 邮箱验证码功能依赖 SMTP 配置；`SMTP_HOST`、`SMTP_USERNAME`、`SMTP_PASSWORD` 配置齐全即启用，未配置齐全时自动关闭，登录不触发验证码。`LOGIN_MFA_ENABLED` 可显式覆盖该默认行为。
- 触发条件在密码验证成功后判定：本次登录 IP 哈希与上次成功登录的 IP 基线（`users.last_login_ip_hash`）不同，或该邮箱自上次成功登录以来连续失败达到 3 次。首次登录无 IP 基线时视为未变更。
- 验证码为 6 位随机数字，有效期 10 分钟，仅可使用一次。
- 验证码不以明文存储，数据库保存 `sha256(JWT_SECRET + ":" + code)` 哈希。
- 单个验证码最多允许 5 次校验尝试，超限后作废；尝试次数以数据库原子递增方式消耗，并发请求无法绕过上限。
- 验证码发送间隔 60 秒，每邮箱每小时最多发送 5 封：重发接口在间隔内返回 429；登录路径在间隔内命中风险条件时复用现有验证码（重新返回挑战响应，不重复发信）。重发成功会作废旧验证码。
- 验证码挑战通过短期 `challengeToken`（JWT，`tokenType` 为 `mfa_challenge`，有效期 10 分钟，`subject` 为登录邮箱）绑定到密码验证成功的会话，校验和重发接口不接受没有挑战令牌的请求，防止对任意邮箱爆破验证码或借接口发送垃圾邮件。
- 验证码校验失败统一返回 `验证码错误或已过期`，不区分错误类型，避免探测。
- 验证码猜错会以失败原因 `mfa_invalid_code` 计入 `login_attempts`，与密码失败共享三级锁定规则，防止持正确密码的攻击者无限轮换验证码爆破。
- 验证码校验通过才记录登录成功并更新 IP 基线；挑战签发阶段不计入 `login_attempts`。
- `LOGIN_MFA_DEV_LOG=true` 时验证码写入后端日志而不实际发信，仅用于本地联调。

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
- `tokenType` 为 `mfa_challenge` 的令牌仅用于验证码校验与重发，不能访问后台 API，也不能当作 access 或 refresh token 使用。
- 允许访问后台 API 的角色为 `admin` 和 `editor`。
- 前端在后台 API 返回 `401 Unauthorized` 时，会优先使用 refresh token 无感刷新并重放原请求；refresh token 刷新失败后才清理登录态并跳转登录页。

## 登出

当前使用 Bearer token。前端登出时删除本地保存的 token 即可。

后续如改为安全 Cookie Session，应新增：

```http
POST /api/v1/auth/logout
```

并由后端清理登录态 Cookie。
