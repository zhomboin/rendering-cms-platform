# 登录邮箱验证码（两步登录）实施计划

**目标：** 在现有登录防爆破锁定之上增加邮箱验证码二次校验：密码验证成功后，若本次登录 IP 与上次成功登录不同，或该邮箱连续失败达到 3 次，则向账号邮箱发送 6 位随机数字验证码，验证通过才完成登录，进一步保护后台账号安全。

**已确认决策：**

- 触发判定在密码验证成功后进行（step-up 认证）：密码错误仍走既有失败锁定流程，不发送邮件。
- 触发条件：登录 IP 哈希与 `users.last_login_ip_hash` 基线不同，或该邮箱自上次成功登录以来连续失败 ≥ 3 次；首次登录无基线视为未变更。
- 验证码为 6 位随机数字（`crypto/rand`，补前导零），有效期 10 分钟，仅可使用一次，单码最多校验 5 次。
- 验证码重发间隔 60 秒，每邮箱每小时最多发送 5 封；重发作废旧验证码。
- 验证码不落明文，数据库保存 `sha256(JWT_SECRET + ":" + code)` 哈希。
- 验证码校验与重发接口凭短期 `challengeToken`（JWT，`tokenType=mfa_challenge`，subject 为登录邮箱，TTL 10 分钟）访问，只有密码验证成功才能获得，防止对任意邮箱爆破验证码或借接口发送垃圾邮件。
- 验证码校验失败统一返回 `验证码错误或已过期`，不区分错误类型，避免探测。
- SMTP 未配置齐全时功能自动关闭（登录走原有流程），`LOGIN_MFA_ENABLED` 可显式覆盖；`LOGIN_MFA_DEV_LOG=true` 时验证码写入后端日志，供本地联调两步登录。
- 发信使用个人邮箱 SMTP 授权码（465 隐式 TLS / 587 STARTTLS），不引入付费邮件服务；代码通过 `auth.EmailSender` 接口抽象，后续可平滑替换发送实现。
- 既有 `login_attempts` 三级锁定规则不变；挑战签发阶段不计入 `login_attempts`，完整登录成功（含验证码路径）才记录 success 并更新 IP 基线。

**架构：** PostgreSQL migration `000012_login_email_mfa` 新增 `login_email_codes` 表并为 `users` 增加 `last_login_ip_hash` 列；后端新增 `internal/mail` SMTP 发信包与 `internal/auth/mfa.go` 验证码域逻辑，登录接口在角色检查后插入风险判定；前端登录页改为同页两步表单（密码 → 验证码）。

**技术栈：** Go、Chi、pgx/sqlc、golang-migrate、PostgreSQL、React + TypeScript + Vite。

## 执行状态（2026-10-21）

- [x] migration `000012_login_email_mfa`：up 新建 `login_email_codes` 表（code_hash、expires_at、attempts、used_at）并为 `users` 增加 `last_login_ip_hash`；down 删除表和列。
- [x] sqlc 查询：`CreateLoginEmailCode`、`GetLatestActiveLoginEmailCode`、`ClaimLoginEmailCode`（原子占用）、`IncrementLoginEmailCodeAttempts`、`CountRecentLoginEmailCodes`（小时限流）、`ListRecentLoginAttemptOutcomes`（连续失败判定）、`UpdateUserLastLoginIPHash`；`GetUserByEmail` 增加返回 `last_login_ip_hash`。
- [x] 后端 `internal/mail`：标准库 SMTP 发信，465 隐式 TLS / 其余端口 STARTTLS，中文验证码邮件。
- [x] 后端 `internal/auth`：`GenerateLoginCode`、`HashLoginCode`、`EvaluateLoginRisk`、`EmailCodeStore`/`EmailSender` 接口、`LoginDeps` 依赖注入；登录接口风险判定与挑战响应；新增 `POST /api/v1/auth/mfa/verify`、`POST /api/v1/auth/mfa/resend`；`token.go` 新增 `mfa_challenge` 类型签发与解析。
- [x] 后端配置：`SMTP_HOST/PORT/USERNAME/PASSWORD/FROM`、`LOGIN_MFA_ENABLED`（默认跟随 SMTP）、`LOGIN_MFA_DEV_LOG`。
- [x] 后端测试：风险判定边界（连续失败、成功重置、IP 变更）、验证码格式与哈希、挑战签发（IP 变更/连续失败/无风险跳过/发送失败/小时上限/dev log）、校验（正确发 token、错误计数、超限、过期、重复占用、坏 challengeToken）、重发（间隔限流、作废旧码）。
- [x] 前端 API：`loginAdmin` 返回可辨识联合（`mfaRequired` 挑战分支 / 完整 token 分支），新增 `verifyLoginCode`、`resendLoginCode`。
- [x] 登录页两步表单：第二步 `Input.OTP` 6 位输入、60 秒重发倒计时、返回重新登录；验证通过后才持久化 token 并跳转。
- [x] `docs/apis/auth.md`、`docs/sql/schema.md`、`docs/operations/deployment.md`、`docs/operations/development-environment.md`、`docs/operations/production-access.md`、`docs/guides/go-backend-guide.md`、`docs/guides/frontend-architecture.md`、`scripts/env/dev.env.example`、`deploy/production.env.example` 已同步。
- [x] `go test ./...`、`go vet ./...`、`npm run build` 全部通过；migration 已在本地开发库完成 up/down/up 回环验证。
- [ ] 预发布/生产环境执行：先备份 PostgreSQL，再执行 migration；配置 SMTP 授权码后验收换 IP 登录触发验证码、验证码邮件可达、同 IP 二次登录不再触发、`LOGIN_MFA_DEV_LOG` 保持关闭。

## 登录流程口径

```text
登录请求 -> 锁定检查 -> 密码验证 -> 角色检查
  ├─ 未命中风险（IP 未变且连续失败 < 3）-> 直接签发 access/refresh token
  └─ 命中风险（IP 变更 或 连续失败 >= 3）-> 生成 6 位验证码 -> 写库 -> SMTP 发信
        -> 返回 mfaRequired + challengeToken（不含任何登录令牌）
             ├─ POST /auth/mfa/verify（challengeToken + code）-> 校验通过 -> 签发 token + 记录成功 + 更新 IP 基线
             └─ POST /auth/mfa/resend（challengeToken）-> 作废旧码并发送新码（60s 间隔 / 每小时 5 封）
```

## 行为说明

- 登录接口 `200` 响应以 `mfaRequired` 字段区分两种结果：直接登录成功（`token`/`refreshToken`/`user`）或验证码挑战（`challengeToken`/`email`/`expiresIn`）。
- 验证码校验失败、过期、尝试次数用尽、已使用统一返回 `401` `验证码错误或已过期`；`challengeToken` 无效或过期返回 `401` `登录验证已失效，请重新登录`。
- 重发在 60 秒间隔内或小时配额超限时返回 `429` `验证码发送过于频繁，请稍后再试`。
- 登录触发验证码发送超过小时配额时，登录接口返回 `429`（密码已验证正确也不发放令牌）。
- `tokenType` 为 `mfa_challenge` 的 JWT 不能作为 access 或 refresh token 使用，后台 API 鉴权天然拒绝。

## 边界与已知影响

- 未配置 SMTP 且未开启 `LOGIN_MFA_DEV_LOG` 时，若通过 `LOGIN_MFA_ENABLED=true` 强制启用，命中风险条件的登录会返回 `500` `邮件验证码功能未正确配置`；默认配置不会出现该情况。
- `users.last_login_ip_hash` 基线在验证码路径下以校验请求的来源 IP 更新；Nginx 可信代理边界不变，仍不保存原始 IP。
- 验证码邮件依赖第三方 SMTP 服务质量，可能进入收件方垃圾箱；排查口径见 `docs/operations/production-access.md`。
- `login_email_codes` 表随时间累积已使用记录，个人博客量级下增长缓慢，清理策略留待后续按需补充。
- 前端 401 自动刷新拦截器对登录页请求天然跳过重定向，两步登录无需改动拦截器。
