# 代码审查报告：Rendering CMS Platform

> 审查日期：2026-05-13
> 审查范围：`frontend/`（React + TypeScript）和 `backend/`（Go + PostgreSQL）
> 审查目标：前端界面问题、安全漏洞问题、代码规范问题

---

## 目录

- [一、问题总览](#一问题总览)
- [二、前端界面问题](#二前端界面问题)
- [三、安全问题](#三安全问题)
- [四、代码规范问题](#四代码规范问题)
- [五、修复优先级清单](#五修复优先级清单)

---

## 一、问题总览

| 严重程度 | 数量 | 说明 |
|---------|------|------|
| 🔴 严重 | 3 | 应立即修复，可能导致安全事故或系统崩溃 |
| ⚠️ 需修复 | 6 | 应在下个迭代修复，存在安全隐患或编码缺陷 |
| 🟡 建议改进 | 10 | 可在后续优化，影响体验、性能或可维护性 |

---

## 二、前端界面问题

### 2.1 缺少错误边界 (Error Boundary) — 🔴 严重

**文件**: `frontend/src/routes/index.tsx`、`frontend/src/app/providers.tsx`

**问题描述**: 整个应用没有 React Error Boundary 包裹路由。任何一个页面组件崩溃都会导致整个 AdminLayout 壳层白屏，用户无法通过导航切换页面恢复。

**影响**: 生产环境中，单个页面的未处理异常将导致整个后台不可用。

**建议修复**:
```tsx
// 在 providers.tsx 中增加 Error Boundary
<QueryClientProvider client={queryClient}>
  <ConfigProvider theme={appTheme}>
    <ErrorBoundary fallback={<ErrorFallback />}>
      <AppRoutes />
    </ErrorBoundary>
  </ConfigProvider>
</QueryClientProvider>
```

**参考**: React 官方文档 [Error Boundaries](https://react.dev/reference/react/Component#catching-rendering-errors-with-an-error-boundary)

---

### 2.2 TanStack Query 缺少 staleTime/gcTime 配置 — 🟡 建议改进

**文件**: `frontend/src/app/providers.tsx:6`

**问题描述**: `QueryClient` 使用默认构造函数创建，未配置 `staleTime` 和 `gcTime`。所有查询在组件挂载、窗口重新聚焦时都会立即重新获取，造成不必要的网络请求。

**当前代码**:
```typescript
const queryClient = new QueryClient();
```

**建议修复**:
```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000,  // 5 分钟内视为新鲜，不重新获取
      gcTime: 15 * 60 * 1000,     // 缓存保留 15 分钟
      retry: 1,
      refetchOnWindowFocus: false, // 管理后台不需要自动重取
    },
  },
});
```

**参考**: [TanStack Query - Important Defaults](https://tanstack.com/query/v5/docs/framework/react/guides/important-defaults)

---

### 2.3 认证状态非响应式 — 🟡 建议改进

**文件**: `frontend/src/layouts/AdminLayout.tsx:35-36`

**问题描述**: `AdminLayout` 在每次渲染时直接调用 `getAuthToken()`/`getAuthUser()` 读取 localStorage。如果 token 在页面使用期间过期或由其他标签页清除，布局组件不会自动重新渲染。

**影响**: token 过期后，用户仍能看到页面内容（因为组件未重新渲染），直到手动触发导航或刷新才会被重定向到登录页。

**建议修复**: 创建 AuthContext 封装认证状态，监听 `storage` 事件实现跨标签页同步。

```typescript
// 建议新增 src/app/AuthContext.tsx
const AuthContext = createContext<AuthState | null>(null);

function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState(getAuthUser());
  const [token, setToken] = useState(getAuthToken());

  useEffect(() => {
    const handleStorage = (e: StorageEvent) => {
      if (e.key === 'rendering-cms-token') {
        setToken(e.newValue);
        setUser(getAuthUser());
      }
    };
    window.addEventListener('storage', handleStorage);
    return () => window.removeEventListener('storage', handleStorage);
  }, []);

  return (
    <AuthContext.Provider value={{ user, token }}>
      {children}
    </AuthContext.Provider>
  );
}
```

---

### 2.4 下载操作使用 `window.open()` — 🟡 建议改进

**文件**: `frontend/src/pages/assets/AssetsPage.tsx:61`

**问题描述**: 资源下载使用 `window.open(downloadUrl, '_blank', 'noopener,noreferrer')` 打开预签名 URL。浏览器弹窗阻止器可能会拦截此操作，导致下载失败。

**当前代码**:
```typescript
const handleDownload = async (asset: AssetFile) => {
  try {
    const downloadUrl = await getAdminAssetDownloadUrl(asset.assetId);
    window.open(downloadUrl, '_blank', 'noopener,noreferrer');
  } catch (error) {
    message.error(error instanceof Error ? error.message : '下载链接生成失败');
  }
};
```

**建议修复**:
```typescript
const handleDownload = async (asset: AssetFile) => {
  try {
    const downloadUrl = await getAdminAssetDownloadUrl(asset.assetId);
    const a = document.createElement('a');
    a.href = downloadUrl;
    a.download = asset.filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  } catch (error) {
    message.error(error instanceof Error ? error.message : '下载链接生成失败');
  }
};
```

---

### 2.5 评论审核按钮 loading 状态共享 — 🟢 低优先级

**文件**: `frontend/src/pages/comments/CommentsPage.tsx:111,126`

**问题描述**: 所有待审核评论的"通过"/"拒绝"按钮共享 `reviewMutation.isPending` 状态。当审核一项评论时，所有待审核评论的按钮都显示 loading 状态。

**影响**: 视觉上会让用户感觉"所有评论都在处理中"，体验不佳。

**建议修复**: 维护一个 `reviewingId` 状态追踪正在操作的评论 ID：

```typescript
const [reviewingId, setReviewingId] = useState<string | null>(null);

// 在 CommentCard 中
loading={reviewingId === comment.commentId}
```

---

### 2.6 文章列表客户端过滤 — 🟡 建议改进

**文件**: `frontend/src/pages/articles/ArticleListPage.tsx:46-55`

**问题描述**: 文章列表一次性加载全部文章，通过 `useMemo` 进行客户端过滤。当文章数量增长到数百篇以上时性能堪忧。

**影响**: 文章积累后，页面首次加载会传输大量数据，客户端过滤占用主线程。

**建议修复**: 改为后端分页 + 筛选参数。在后端 `ListAdminArticles` 查询中增加 `status`、`keyword`、`limit`、`offset` 参数。

---

### 2.7 文章编辑器表单初始化延迟 — 🟢 低优先级

**文件**: `frontend/src/pages/articles/ArticleEditorPage.tsx:60-75`

**问题描述**: 编辑模式下，表单字段通过 `useEffect` + `setFieldsValue` 填充。在 `useEffect` 触发前，表单会短暂显示空值，产生"闪烁"效果。

**建议修复**: 使用 Ant Design Form 的 `fields` 属性替代 `setFieldsValue`，确保同步初始化：

```tsx
<Form
  form={form}
  fields={isEdit && currentArticle ? articleToFields(currentArticle) : initialFields}
  // ...
>
```

---

### 2.8 缺少公开路由 — 🟡 建议改进

**文件**: `frontend/src/routes/index.tsx`

**问题描述**: README 中声明的公开路由 `/articles` 和 `/articles/:slug` 未在路由文件中定义。这与项目文档不一致。

**建议**: 确认需求后，要么实现公开路由，要么更新 README 与实际路由范围保持一致。

---

## 三、安全问题

### 3.1 JWT Token 存储在 localStorage — 🔴 严重

**文件**: `frontend/src/api/auth-token.ts:6,12,21`

**问题描述**: JWT token 明文存储在 `localStorage` 中。任何注入的 XSS 脚本都可以通过 `localStorage.getItem('rendering-cms-token')` 直接读取并窃取 token。

**影响**: 成功的 XSS 攻击可导致攻击者获得后台管理员权限。

**OWASP 分类**: [A02:2021 - Cryptographic Failures](https://owasp.org/Top10/A02_2021-Cryptographic_Failures/)

**建议修复**:

1. **前端**: 改用 `httpOnly` Cookie 存储 token（JavaScript 不可读）：
   - 后端 login handler 响应中设置 `Set-Cookie` header
   - token 存储 key 改为 `__Host-` 前缀，限制 cookie 范围

2. **后端**: 在 login handler 返回响应时设置 cookie：
```go
http.SetCookie(w, &http.Cookie{
    Name:     "__Host-token",
    Value:    token,
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteStrictMode,
    Path:     "/api/v1/admin",
    MaxAge:   86400, // 24 小时
})
```

---

### 3.2 `ipHashFromRequest` 实现不一致 — 🔴 严重

**文件**:
- `backend/internal/auth/handler.go:189-200`
- `backend/internal/comments/handler.go:182-189`
- `backend/internal/assets/handler.go:228-235`
- `backend/internal/http/middleware.go:123-136`

**问题描述**: 不同包中的 `ipHashFromRequest` 对代理头（`X-Forwarded-For`、`X-Real-IP`）的处理不一致：

| 文件 | 检查 `X-Forwarded-For` | 检查 `X-Real-IP` |
|------|----------------------|-----------------|
| `auth/handler.go` | ✅ | ✅ |
| `http/middleware.go` | ✅ | ✅ |
| `comments/handler.go` | ❌ | ❌ |
| `assets/handler.go` | ❌ | ❌ |

**影响**: 
- 在 Nginx/HAProxy 反向代理部署下，评论和资产的 IP 哈希固定为代理服务器 IP
- 评论速率限制完全失效（所有请求共享同一 IP 哈希）
- 下载审计记录无法区分不同用户

**建议修复**: 将所有 `ipHashFromRequest` 统一到 `internal/http/` 包中，使用 `middleware.go` 的实现（同时处理 `X-Forwarded-For` 和 `X-Real-IP`）：

```go
// internal/http/ip.go
func ClientIPHash(r *http.Request) string {
    ip := clientIP(r)
    sum := sha256.Sum256([]byte(ip))
    return hex.EncodeToString(sum[:])
}
```

---

### 3.3 CORS `AllowedHeaders: ["*"]` — ⚠️ 需修复

**文件**: `backend/internal/http/middleware.go:112`

```go
AllowedHeaders: []string{"*"},
```

**问题描述**: 通配符 `*` 允许任意请求头，在 `AllowCredentials: true` 场景下过于宽松。攻击者可以发送任意自定义头，增加攻击面。

**影响**: 放宽了浏览器安全沙箱限制。

**OWASP 分类**: [A05:2021 - Security Misconfiguration](https://owasp.org/Top10/A05_2021-Security_Misconfiguration/)

**建议修复**:
```go
AllowedHeaders: []string{
    "Content-Type",
    "Authorization",
    "X-Requested-With",
},
```

---

### 3.4 缺少 CSRF 保护 — ⚠️ 需修复

**文件**: 
- `frontend/src/api/client.ts:24` (`withCredentials: true`)
- `backend/internal/http/middleware.go` (无 CSRF 中间件)

**问题描述**: 应用使用 Bearer token (`Authorization` header) 进行认证，同时开启 `withCredentials: true`。虽然 Bearer token 方式对 CSRF 有一定天然防御（浏览器不会自动添加自定义 header），但 `withCredentials: true` 配合 cookie session 可能在未来引入 CSRF 风险。

**影响**: 如果后续迁移到 cookie-based 认证而无 CSRF 保护，攻击者可通过跨站请求伪造执行管理员操作。

**OWASP 分类**: [A01:2021 - Broken Access Control](https://owasp.org/Top10/A01_2021-Broken_Access_Control/)

**建议修复**:
1. 如果保持 Bearer token 方案：移除 `withCredentials: true`（当前不必要），减少 cookie 传递
2. 如果迁移到 httpOnly cookie 方案：必须同步实现 CSRF token（double-submit cookie 模式）

---

### 3.5 缺少 Content-Security-Policy 头 — ⚠️ 需修复

**文件**: 
- `backend/internal/http/middleware.go` (无 CSP 中间件)
- `frontend/index.html` (无 `<meta>` CSP)

**问题描述**: 前端和后端均未设置 Content-Security-Policy 响应头。CSP 是现代浏览器防御 XSS 攻击的最重要机制之一。

**影响**: 即使存在 XSS 注入点，浏览器也无法阻止恶意脚本执行。

**OWASP 分类**: [A05:2021 - Security Misconfiguration](https://owasp.org/Top10/A05_2021-Security_Misconfiguration/)

**建议修复**: 在 Go 后端 middleware 中增加 CSP 中间件：

```go
func ContentSecurityPolicyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Security-Policy",
            "default-src 'self'; " +
            "script-src 'self'; " +
            "style-src 'self' 'unsafe-inline'; " +
            "img-src 'self' data: https:; " +
            "connect-src 'self' http://127.0.0.1:8080;")
        next.ServeHTTP(w, r)
    })
}
```

---

### 3.6 登录时序攻击 (Timing Attack) — 🟡 建议改进

**文件**: `backend/internal/auth/handler.go:111-115`

```go
user, err := finder.FindUserByEmail(r.Context(), request.Email)
if err != nil || !VerifyPassword(user.PasswordHash, request.Password) {
```

**问题描述**: 
- 邮箱不存在时：`err != nil` 为 `true`，短路求值，**不执行** `VerifyPassword`（bcrypt 比较）
- 邮箱存在但密码错误时：`err != nil` 为 `false`，**执行** bcrypt 比较再返回错误
- bcrypt 计算耗时约 100-300ms，攻击者可通过测量响应时间差异枚举有效邮箱

**影响**: 攻击者可以枚举系统中已注册的管理员邮箱。

**建议修复**:
```go
user, err := finder.FindUserByEmail(r.Context(), request.Email)
if err != nil {
    // 执行 dummy bcrypt 比较以消除时间差异
    _ = VerifyPassword(
        "$2a$10$dummyhashfordummyhashfordummyhashfordummyhashfordummyhashfo",
        request.Password,
    )
    recordLoginAttempt(r.Context(), store, request.Email, ipHash, false, "invalid_credentials")
    writeAuthError(w, http.StatusUnauthorized, "邮箱或密码错误")
    return
}
if !VerifyPassword(user.PasswordHash, request.Password) {
    // ...
}
```

---

### 3.7 `http.Server` 未设置超时限制 — ⚠️ 需修复

**文件**: `backend/cmd/server/main.go:51-56`

```go
server := &http.Server{
    Addr:    cfg.HTTPAddr,
    Handler: httpapi.NewRouter(...),
}
```

**问题描述**: `http.Server` 未设置 `ReadTimeout`、`WriteTimeout`、`IdleTimeout`、`MaxHeaderBytes`。攻击者可以通过慢速连接消耗服务器资源。

**影响**: Slowloris 等 DoS 攻击可能导致服务器资源耗尽。

**OWASP 分类**: [A05:2021 - Security Misconfiguration](https://owasp.org/Top10/A05_2021-Security_Misconfiguration/)

**建议修复**:
```go
server := &http.Server{
    Addr:           cfg.HTTPAddr,
    Handler:        handler,
    ReadTimeout:    15 * time.Second,
    WriteTimeout:   30 * time.Second,
    IdleTimeout:    60 * time.Second,
    MaxHeaderBytes: 1 << 20, // 1MB
}
```

---

### 3.8 日志中记录原始客户端 IP — 🟡 建议改进

**文件**: `backend/internal/http/middleware.go:59-67`

```go
logger.InfoContext(r.Context(), "http_request",
    // ...
    "remote_addr", clientIP(r),
    "user_agent", r.UserAgent(),
)
```

**问题描述**: 请求日志中记录了原始客户端 IP。虽然业务数据库只存储 IP 哈希（符合项目安全规则），但日志文件中保留了原始 IP，需评估隐私合规影响。

**影响**: 如果日志文件被未授权访问，可能泄露访问者 IP 信息。

**建议修复**: 在日志中也使用 IP 哈希，或者在日志配置中设置自动清理策略。

---

### 3.9 JWT 无 refresh token 机制 — 🟡 建议改进

**文件**: `backend/internal/auth/token.go:26`

```go
ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
```

**问题描述**: JWT 固定 24 小时过期，不支持续期。用户每 24 小时必须重新登录，管理员体验不佳。

**影响**: 用户体验，长时间写作后可能因 token 过期丢失未保存内容。

**建议修复**: 实现双 token 机制：
- Access token (短期，如 15 分钟) 用于 API 调用
- Refresh token (长期，如 7 天) 用于续期

---

### 3.10 S3 凭证通过环境变量传递 — 🟡 建议改进

**文件**: `backend/internal/config/config.go:42-43`

```go
AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
SecretAccessKey: os.Getenv("S3_SECRET_ACCESS_KEY"),
```

**问题描述**: S3 访问密钥通过明文环境变量传入。可能被错误的日志、进程列表（`/proc/*/environ`）、调试端点或容器编排面板泄露。

**影响**: 如果环境变量泄露，攻击者可直接访问 S3 存储桶。

**建议修复**: 生产环境使用 IAM role（EC2/EKS）、Kubernetes Secrets + 挂载卷或 Vault 注入。本地开发保留环境变量方式。

---

### 3.11 后端缺少请求体大小限制 — ⚠️ 需修复

**文件**: `backend/cmd/server/main.go:51-56`

**问题描述**: `http.Server` 未通过 `http.MaxBytesReader` 或中间件限制请求体大小。虽然后端对资产上传有 `MaxUploadBytes = 20MB` 的业务校验，但该校验发生在 JSON 解码之后，大请求体已完全读入内存。

**影响**: 攻击者可以发送超大请求体消耗服务器内存。

**建议修复**: 在 Chi 路由器中增加请求体大小限制中间件：
```go
func RequestSizeLimitMiddleware(limit int64) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            r.Body = http.MaxBytesReader(w, r.Body, limit)
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 四、代码规范问题

### 4.1 重复代码 — 🔴 严重

**涉及文件**: 跨 5 个包, 约 10 处重复

以下辅助函数在多个包中独立定义了几乎相同的实现：

| 函数 | 重复位置 |
|------|---------|
| `writeError` | `internal/articles/handler.go`、`internal/comments/handler.go`、`internal/assets/handler.go`、`internal/analytics/handler.go`、`internal/auth/handler.go` |
| `writeJSON` | 同上 5 处 |
| `uuidFromString` | `internal/articles/handler.go`、`internal/comments/handler.go`、`internal/assets/handler.go`、`internal/analytics/handler.go` |
| `nullableText` | `internal/articles/handler.go`、`internal/comments/handler.go`、`internal/assets/handler.go`、`internal/auth/handler.go` |
| `textValue` | `internal/articles/handler.go`、`internal/comments/handler.go`、`internal/assets/handler.go` |
| `timestamptzValue` | `internal/articles/handler.go`、`internal/comments/handler.go`、`internal/assets/handler.go`、`internal/analytics/handler.go` |
| `ipHashFromRequest` | `internal/auth/handler.go`、`internal/comments/handler.go`、`internal/assets/handler.go` |

**问题描述**: 大量 DRY 违反，增加维护负担。修改一处功能需要同步修改多个文件。

**建议修复**: 统一提取到 `internal/http/helpers.go` 或新建 `internal/shared/` 包：

```go
// internal/http/helpers.go
package httpapi

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "net/http"

    "github.com/jackc/pgx/v5/pgtype"
)

func WriteJSON(w http.ResponseWriter, status int, body interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(body); err != nil {
        slog.Error("json encode failed", "error", err)
    }
}

func WriteError(w http.ResponseWriter, status int, message string) {
    WriteJSON(w, status, map[string]string{"error": message})
}

func UUIDFromString(value string) (pgtype.UUID, error) {
    var uuid pgtype.UUID
    if err := uuid.Scan(value); err != nil {
        return pgtype.UUID{}, err
    }
    return uuid, nil
}

func NullableText(value string) pgtype.Text {
    return pgtype.Text{String: value, Valid: value != ""}
}

func TextValue(value pgtype.Text) *string {
    if !value.Valid {
        return nil
    }
    return &value.String
}

func TimestamptzValue(value pgtype.Timestamptz) interface{} {
    if !value.Valid {
        return nil
    }
    return value.Time
}

func ClientIPHash(r *http.Request) string {
    ip := clientIP(r)
    sum := sha256.Sum256([]byte(ip))
    return hex.EncodeToString(sum[:])
}
```

---

### 4.2 `map[string]interface{}` 替代类型化结构体 — ⚠️ 需修复

**涉及文件**:
- `backend/internal/articles/handler.go:235-252`
- `backend/internal/comments/handler.go:191-228`
- `backend/internal/assets/handler.go:237-249`
- `backend/internal/analytics/handler.go`

**当前代码**:
```go
func mapArticle(article dbgen.Article) map[string]interface{} {
    return map[string]interface{}{
        "articleId": article.ArticleID.String(),
        "slug":      article.Slug,
        "title":     article.Title,
        // ...
    }
}
```

**问题描述**: 所有 handler 使用动态 `map[string]interface{}` 进行 JSON 序列化，而非定义编译时类型检查的响应结构体。这带来以下问题：
1. 字段名拼写错误只能在运行时暴露
2. 字段类型不一致无法被编译器检查
3. Go 的 JSON 编码器对 struct 有优化，对 `map[string]interface{}` 无优化
4. 前端类型定义需要手动维护，与后端不一致时会静默失败

**建议修复**:
```go
// internal/articles/response.go
type ArticleResponse struct {
    ArticleID    string  `json:"articleId"`
    Slug         string  `json:"slug"`
    Title        string  `json:"title"`
    Summary      string  `json:"summary"`
    BodyMdx      string  `json:"bodyMdx"`
    Status       string  `json:"status"`
    Version      int32   `json:"version"`
    Tags         []string `json:"tags"`
    Featured     bool    `json:"featured"`
    CoverImageURL *string `json:"coverImageUrl,omitempty"`
    PublishedAt  *time.Time `json:"publishedAt,omitempty"`
    AuthorID     string  `json:"authorId"`
    CreatedAt    time.Time `json:"createdAt"`
    UpdatedAt    time.Time `json:"updatedAt"`
}
```

---

### 4.3 静默丢弃 JSON 编码错误 — ⚠️ 需修复

**涉及文件**: 所有包含 `writeJSON` 的 handler 文件

```go
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(body)  // 丢弃错误
}
```

**问题描述**: JSON 编码失败时错误被静默丢弃。如果编码失败（例如包含不可序列化的值），客户端将收到截断的响应，且无任何日志记录。

**建议修复**: 至少记录错误：
```go
func writeJSON(w http.ResponseWriter, status int, body interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(body); err != nil {
        slog.Error("failed to encode JSON response", "error", err, "status", status)
    }
}
```

---

### 4.4 Handler 中未记录业务错误日志 — ⚠️ 需修复

**涉及文件**: 所有 handler 文件

**当前代码**:
```go
articles, err := h.queries.ListAdminArticles(r.Context())
if err != nil {
    writeError(w, http.StatusInternalServerError, "后台文章列表读取失败")
    return
}
```

**问题描述**: handler 返回给客户端的错误信息是固定的中文消息，但实际的 Go error (`err`) 被丢弃，没有记录到日志。调试时只能看到 "后台文章列表读取失败"，无法获知实际数据库连接问题、SQL 语法错误还是其他原因。

**建议修复**: 统一增加结构化日志：
```go
articles, err := h.queries.ListAdminArticles(r.Context())
if err != nil {
    slog.ErrorContext(r.Context(), "list admin articles failed", "error", err)
    writeError(w, http.StatusInternalServerError, "后台文章列表读取失败")
    return
}
```

---

### 4.5 数据库健康检查缺失 — 🟡 建议改进

**文件**: `backend/internal/http/router.go:74-80`

```go
router.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
})
```

**问题描述**: 健康检查端点仅返回 `{"status":"ok"}`，不验证数据库连接。即使 PostgreSQL 宕机，健康检查仍然返回 200。

**影响**: 容器编排系统（Kubernetes liveness/readiness probe）无法感知数据库不可用，导致流量持续发送到不健康实例。

**建议修复**: 注入数据库连接并执行 ping：
```go
router.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
    if err := db.PingContext(r.Context()); err != nil {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusServiceUnavailable)
        _ = json.NewEncoder(w).Encode(map[string]string{
            "status": "degraded",
            "db":     "unreachable",
        })
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
})
```

---

### 4.6 Go 版本过新 — 🟡 建议改进

**文件**: `backend/go.mod:3`

```
go 1.26.2
```

**问题描述**: 截至 2026 年 5 月，Go 1.24.x 为最新稳定版本。`go 1.26.2` 可能指向未发布的预发布版本或未来版本。

**影响**: 生产环境中使用未稳定版本可能遇到未发现的 bug 或 breaking changes。

**建议修复**: 降级到 `go 1.24.x`（当前最新稳定版）。

---

### 4.7 前端 TypeScript 类型安全性不足 — 🟡 建议改进

**涉及文件**: `frontend/src/pages/auth/LoginPage.tsx:39` 等多处

```typescript
const error = err as { message?: string };
```

**问题描述**: 多处使用 `as` 类型断言而不做运行时类型检查。一旦后端错误响应格式变化，前端可能显示 `undefined` 或崩溃。

**建议修复**:
1. 检查并开启 `tsconfig.app.json` 中的 `strict: true`
2. 使用运行时类型守卫替代 `as` 断言：

```typescript
function isApiError(err: unknown): err is ApiRequestError {
  return err instanceof ApiRequestError;
}

catch (err) {
  const message = isApiError(err) ? err.message : '登录失败，请检查邮箱和密码';
  void message.error(message);
}
```

---

### 4.8 前端 API 响应类型与后端不一致 — 🟡 建议改进

**问题描述**: 前端 API 类型定义（`frontend/src/api/articles.ts` 等）与后端 `map[string]interface{}` 响应之间没有编译时一致性保证。如果后端修改了响应字段名，前端类型定义不会报错，而是在运行时静默失败。

**建议修复**: 
1. 后端先改为类型化响应结构体（见 4.2）
2. 后续考虑使用 OpenAPI/Swagger 自动生成前端类型定义

---

### 4.9 错误未使用 `fmt.Errorf` 包装 — 🟡 建议改进

**问题描述**: Go 1.13+ 推荐的错误链包装 (`fmt.Errorf("...: %w", err)`) 未使用。调用链上游无法通过 `errors.Is()` 和 `errors.As()` 追踪错误来源。

**当前示例**:
```go
// 应改为
return fmt.Errorf("find user by email: %w", err)
```

---

### 4.10 缺少共享组件目录 — 🟢 低优先级

**文件**: `frontend/src/` 目录结构

**问题描述**: 根据 `frontend/ARCHITECTURE.md`，`src/components/` 应用于跨页面复用的 UI 组件，但该目录不存在。`MdxPreview` 组件与文章编辑器页面共置，阻碍了跨页面复用。所有页面的标题栏、加载状态、空状态模式也重复出现。

**建议修复**: 创建 `src/components/` 目录，将通用组件（如 `MdxPreview`、`PageHeader`、`ErrorAlert`）提取到其中。

---

## 五、修复优先级清单

### 🔴 严重 — 应立即修复

| # | 类别 | 问题 | 文件 | 工作量 |
|---|------|------|------|--------|
| 1 | 安全 | JWT 存储在 localStorage | `frontend/src/api/auth-token.ts` | 中 |
| 2 | 安全 | `ipHashFromRequest` 实现不一致 | `backend/internal/{auth,comments,assets}/handler.go` | 小 |
| 3 | 规范 | 多处重复代码 (writeError 等 7 个函数) | 跨 5 个包 | 中 |

### ⚠️ 需修复 — 下个迭代完成

| # | 类别 | 问题 | 文件 | 工作量 |
|---|------|------|------|--------|
| 4 | 安全 | CORS `AllowedHeaders: ["*"]` | `backend/internal/http/middleware.go` | 小 |
| 5 | 安全 | 缺少 CSRF 保护 | `frontend/src/api/client.ts` + `backend/internal/http/` | 中 |
| 6 | 安全 | `http.Server` 未设置 timeout | `backend/cmd/server/main.go` | 小 |
| 7 | 安全 | 缺少请求体大小限制中间件 | `backend/internal/http/middleware.go` | 小 |
| 8 | 规范 | `map[string]interface{}` 替代类型化结构体 | 所有 handler 文件 | 大 |
| 9 | 规范 | 静默丢弃 JSON 编码错误 | 所有 handler 文件 | 小 |
| 10 | 规范 | Handler 中未记录业务错误日志 | 所有 handler 文件 | 中 |

### 🟡 建议改进 — 后续迭代

| # | 类别 | 问题 | 文件 | 工作量 |
|---|------|------|------|--------|
| 11 | 安全 | 缺少 CSP 头 | `backend/internal/http/middleware.go` | 小 |
| 12 | 安全 | 登录时序攻击 | `backend/internal/auth/handler.go` | 小 |
| 13 | 安全 | JWT 无 refresh token | `backend/internal/auth/token.go` | 中 |
| 14 | 安全 | S3 凭证明文环境变量 | `backend/internal/config/config.go` | 中 |
| 15 | 安全 | 日志记录原始 IP | `backend/internal/http/middleware.go` | 小 |
| 16 | 前端 | 缺少 Error Boundary | `frontend/src/app/providers.tsx` | 小 |
| 17 | 前端 | TanStack Query 无缓存配置 | `frontend/src/app/providers.tsx` | 小 |
| 18 | 前端 | `window.open()` 下载方式 | `frontend/src/pages/assets/AssetsPage.tsx` | 小 |
| 19 | 规范 | 数据库健康检查缺失 | `backend/internal/http/router.go` | 小 |
| 20 | 规范 | Go 1.26.2 版本过新 | `backend/go.mod` | 小 |
| 21 | 规范 | 前端 TypeScript 类型安全性 | `frontend/src/` 多处 | 中 |
| 22 | 前端 | 认证状态非响应式 | `frontend/src/layouts/AdminLayout.tsx` | 中 |
| 23 | 前端 | 文章列表客户端过滤 | `frontend/src/pages/articles/ArticleListPage.tsx` | 中 |
| 24 | 前端 | 评论审核按钮共享 loading 状态 | `frontend/src/pages/comments/CommentsPage.tsx` | 小 |
| 25 | 前端 | 文章编辑器表单闪烁 | `frontend/src/pages/articles/ArticleEditorPage.tsx` | 小 |
| 26 | 规范 | 缺少共享组件目录 | `frontend/src/` 目录结构 | 中 |

---

## 附录：审查方法

| 审查维度 | 方法 |
|---------|------|
| 架构审查 | 遍历项目目录结构，分析模块划分和依赖关系 |
| 安全审查 | OWASP Top 10 (2021) 对照检查，手动代码审计 |
| 前端审查 | React 最佳实践对照，Ant Design 组件使用规范检查 |
| 后端审查 | Go 编码规范对照，DRY 原则检查，错误处理模式分析 |
| 数据层审查 | SQL 查询文件 + 数据库 migration 审计 |
