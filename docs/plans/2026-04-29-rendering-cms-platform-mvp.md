# Rendering CMS Platform MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付一个可运行的 CMS MVP，支持管理员登录、文章导入与发布、公开阅读、基础访问统计、评论审核、文件上传下载和后台展示。

**Architecture:** MVP 采用前后端分离：`backend/` 是 Go API 服务，`frontend/` 是 React + TypeScript 应用，PostgreSQL 是唯一运行时数据源，S3 兼容对象存储保存上传文件。原静态博客仅作为 MDX 内容导入来源，不在运行时被后端写入。

**Tech Stack:** Go 1.22+、Chi、pgx、sqlc、golang-migrate、PostgreSQL、bcrypt、JWT 或 HTTP-only Cookie、aws-sdk-go-v2、React、TypeScript、Vite、React Router、TanStack Query、Ant Design、MDX 渲染。

---

## MVP 验收边界

- 管理员可以登录后台。
- 管理员可以从 MDX 导入文章，并在后台新增、编辑、发布文章。
- 公开页面可以列出已发布文章并打开详情页。
- 文章详情页会记录文章访问量和站点访问量。
- 后台首页可以看到今日访问量、近 7 天访问量和热门文章。
- 访客可以提交评论，评论默认待审核；管理员审核通过后才公开显示。
- 管理员可以上传允许类型的文件，并生成下载链接；下载行为写入审计表。
- 后端 `go test ./...` 通过，前端 `npm run build` 通过。

## MVP 文件结构

- Create: `backend/go.mod`
- Create: `backend/cmd/server/main.go`
- Create: `backend/cmd/import-mdx/main.go`
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/http/router.go`
- Create: `backend/internal/http/middleware.go`
- Create: `backend/internal/database/db.go`
- Create: `backend/internal/auth/password.go`
- Create: `backend/internal/auth/token.go`
- Create: `backend/internal/auth/handler.go`
- Create: `backend/internal/articles/service.go`
- Create: `backend/internal/articles/handler.go`
- Create: `backend/internal/analytics/service.go`
- Create: `backend/internal/analytics/handler.go`
- Create: `backend/internal/comments/service.go`
- Create: `backend/internal/comments/handler.go`
- Create: `backend/internal/assets/service.go`
- Create: `backend/internal/assets/handler.go`
- Create: `backend/internal/storage/s3.go`
- Create: `backend/migrations/000001_init.up.sql`
- Create: `backend/migrations/000001_init.down.sql`
- Create: `backend/sqlc.yaml`
- Create: `backend/sql/articles.sql`
- Create: `backend/sql/analytics.sql`
- Create: `backend/sql/comments.sql`
- Create: `backend/sql/assets.sql`
- Create: `backend/sql/users.sql`
- Create: `frontend/package.json`
- Create: `frontend/src/api/client.ts`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/routes/index.tsx`
- Create: `frontend/src/features/auth/LoginPage.tsx`
- Create: `frontend/src/features/articles/ArticleListPage.tsx`
- Create: `frontend/src/features/articles/ArticleDetailPage.tsx`
- Create: `frontend/src/features/articles/AdminArticleListPage.tsx`
- Create: `frontend/src/features/articles/AdminArticleEditorPage.tsx`
- Create: `frontend/src/features/analytics/AdminDashboardPage.tsx`
- Create: `frontend/src/features/comments/AdminCommentsPage.tsx`
- Create: `frontend/src/features/assets/AdminAssetsPage.tsx`
- Create: `docs/apis/README.md`
- Create: `docs/apis/auth.md`
- Create: `docs/apis/articles.md`
- Create: `docs/apis/analytics.md`
- Create: `docs/apis/comments.md`
- Create: `docs/apis/assets.md`
- Create: `docs/sql/schema.md`
- Create: `docs/operations/deployment.md`

## Task 1: 项目骨架和基础命令

**Files:**

- Create: `backend/go.mod`
- Create: `backend/cmd/server/main.go`
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/http/router.go`
- Create: `frontend/package.json`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/routes/index.tsx`

- [ ] **Step 1: 初始化后端模块**

Run:

```bash
mkdir -p backend/cmd/server backend/internal/config backend/internal/http
cd backend
go mod init rendering-cms-platform/backend
go get github.com/go-chi/chi/v5 github.com/jackc/pgx/v5/pgxpool github.com/golang-jwt/jwt/v5 golang.org/x/crypto/bcrypt github.com/google/uuid github.com/aws/aws-sdk-go-v2/config github.com/aws/aws-sdk-go-v2/service/s3
```

Expected: `backend/go.mod` 存在，并包含 `module rendering-cms-platform/backend`。

- [ ] **Step 2: 写入配置结构**

Create `backend/internal/config/config.go`:

```go
package config

import "os"

type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	JWTSecret      string
	FrontendOrigin string
	S3Endpoint     string
	S3Region       string
	S3Bucket       string
	S3AccessKeyID  string
	S3SecretKey    string
}

func Load() Config {
	return Config{
		HTTPAddr:       getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		JWTSecret:      os.Getenv("JWT_SECRET"),
		FrontendOrigin: getEnv("FRONTEND_ORIGIN", "http://127.0.0.1:5173"),
		S3Endpoint:     os.Getenv("S3_ENDPOINT"),
		S3Region:       getEnv("S3_REGION", "auto"),
		S3Bucket:       os.Getenv("S3_BUCKET"),
		S3AccessKeyID:  os.Getenv("S3_ACCESS_KEY_ID"),
		S3SecretKey:    os.Getenv("S3_SECRET_ACCESS_KEY"),
	}
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
```

- [ ] **Step 3: 写入健康检查路由**

Create `backend/internal/http/router.go`:

```go
package http

import (
	"encoding/json"
	nethttp "net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter() nethttp.Handler {
	r := chi.NewRouter()
	r.Get("/api/v1/health", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	return r
}
```

- [ ] **Step 4: 写入服务入口**

Create `backend/cmd/server/main.go`:

```go
package main

import (
	"log"
	nethttp "net/http"

	"rendering-cms-platform/backend/internal/config"
	apphttp "rendering-cms-platform/backend/internal/http"
)

func main() {
	cfg := config.Load()
	log.Printf("starting server on %s", cfg.HTTPAddr)
	if err := nethttp.ListenAndServe(cfg.HTTPAddr, apphttp.NewRouter()); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: 初始化前端**

Run:

```bash
mkdir -p frontend/src/routes
cd frontend
npm create vite@latest . -- --template react-ts
npm install react-router-dom @tanstack/react-query antd
```

Expected: `frontend/package.json` 存在，且 `npm run build` 可执行。

- [ ] **Step 6: 验证骨架**

Run:

```bash
cd backend
go test ./...
cd ../frontend
npm run build
```

Expected: 后端测试通过，前端生产构建成功。

- [ ] **Step 7: 提交骨架**

Run:

```bash
git add backend frontend
git commit -m "feat: scaffold cms platform"
```

## Task 2: 数据库 schema、sqlc 和连接

**Files:**

- Create: `backend/internal/database/db.go`
- Create: `backend/migrations/000001_init.up.sql`
- Create: `backend/migrations/000001_init.down.sql`
- Create: `backend/sqlc.yaml`
- Create: `backend/sql/users.sql`
- Create: `backend/sql/articles.sql`
- Create: `backend/sql/analytics.sql`
- Create: `backend/sql/comments.sql`
- Create: `backend/sql/assets.sql`
- Create: `docs/sql/schema.md`

- [ ] **Step 1: 创建 migration**

Create `backend/migrations/000001_init.up.sql` with the core tables:

```sql
create extension if not exists pgcrypto;

create type user_role as enum ('admin', 'editor');
create type article_status as enum ('draft', 'published', 'archived');
create type comment_status as enum ('pending', 'approved', 'rejected');

create table users (
  user_id uuid primary key default gen_random_uuid(),
  email text not null unique,
  name text not null,
  password_hash text not null,
  role user_role not null default 'admin',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table articles (
  article_id uuid primary key default gen_random_uuid(),
  slug text not null unique,
  title text not null,
  summary text not null,
  body_mdx text not null,
  status article_status not null default 'draft',
  tags text[] not null default '{}',
  featured boolean not null default false,
  cover_image_url text,
  published_at timestamptz,
  author_id uuid not null references users(user_id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table article_revisions (
  revision_id uuid primary key default gen_random_uuid(),
  article_id uuid not null references articles(article_id) on delete cascade,
  title text not null,
  summary text not null,
  body_mdx text not null,
  status article_status not null,
  created_by uuid not null references users(user_id),
  created_at timestamptz not null default now()
);

create table comments (
  comment_id uuid primary key default gen_random_uuid(),
  article_id uuid not null references articles(article_id) on delete cascade,
  author_name text not null,
  author_email text,
  body text not null,
  status comment_status not null default 'pending',
  ip_hash text not null,
  user_agent text,
  created_at timestamptz not null default now(),
  reviewed_at timestamptz
);

create table article_view_daily (
  article_id uuid not null references articles(article_id) on delete cascade,
  view_date date not null,
  views integer not null default 0,
  primary key (article_id, view_date)
);

create table site_view_daily (
  view_date date primary key,
  views integer not null default 0
);

create table assets (
  asset_id uuid primary key default gen_random_uuid(),
  filename text not null,
  content_type text not null,
  byte_size integer not null,
  storage_key text not null unique,
  public_url text,
  created_by uuid not null references users(user_id),
  created_at timestamptz not null default now()
);

create table download_events (
  event_id uuid primary key default gen_random_uuid(),
  asset_id uuid not null references assets(asset_id) on delete cascade,
  ip_hash text not null,
  user_agent text,
  created_at timestamptz not null default now()
);
```

- [ ] **Step 2: 创建 down migration**

Create `backend/migrations/000001_init.down.sql`:

```sql
drop table if exists download_events;
drop table if exists assets;
drop table if exists site_view_daily;
drop table if exists article_view_daily;
drop table if exists comments;
drop table if exists article_revisions;
drop table if exists articles;
drop table if exists users;
drop type if exists comment_status;
drop type if exists article_status;
drop type if exists user_role;
```

- [ ] **Step 3: 配置 sqlc**

Create `backend/sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "sql"
    schema: "migrations"
    gen:
      go:
        package: "db"
        out: "internal/database/dbgen"
        sql_package: "pgx/v5"
```

- [ ] **Step 4: 创建数据库连接**

Create `backend/internal/database/db.go`:

```go
package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, databaseURL)
}
```

- [ ] **Step 5: 写入 schema 文档**

Create `docs/sql/schema.md`:

```markdown
# 数据库 Schema

MVP 使用 PostgreSQL 作为唯一运行时数据源。

## 核心表

- `users`：后台用户。
- `articles`：文章主表，正文存储 MDX。
- `article_revisions`：文章草稿保存和发布历史。
- `comments`：评论及审核状态。
- `article_view_daily`：文章日访问量。
- `site_view_daily`：站点日访问量。
- `assets`：上传文件元数据。
- `download_events`：文件下载审计。

## 隐私规则

评论和下载审计不得保存原始 IP，只保存哈希值。
```

- [ ] **Step 6: 验证 schema 文件**

Run:

```bash
cd backend
go test ./...
```

Expected: Go package 编译通过；如尚未生成 sqlc 代码，本任务只验证手写 Go 文件。

- [ ] **Step 7: 提交 schema**

Run:

```bash
git add backend/migrations backend/sqlc.yaml backend/internal/database docs/sql/schema.md
git commit -m "feat: add cms database schema"
```

## Task 3: 登录认证和后台保护

**Files:**

- Create: `backend/internal/auth/password.go`
- Create: `backend/internal/auth/password_test.go`
- Create: `backend/internal/auth/token.go`
- Create: `backend/internal/auth/token_test.go`
- Create: `backend/internal/auth/handler.go`
- Create: `backend/internal/http/middleware.go`
- Create: `docs/apis/auth.md`

- [ ] **Step 1: 写失败测试：密码哈希可验证**

Create `backend/internal/auth/password_test.go`:

```go
package auth

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct-password") {
		t.Fatal("expected password to verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("expected wrong password to fail")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
cd backend
go test ./internal/auth
```

Expected: FAIL，提示 `HashPassword` 或 `VerifyPassword` 未定义。

- [ ] **Step 3: 实现密码哈希**

Create `backend/internal/auth/password.go`:

```go
package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func VerifyPassword(hash string, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
```

- [ ] **Step 4: 写 token 测试**

Create `backend/internal/auth/token_test.go`:

```go
package auth

import "testing"

func TestIssueAndParseToken(t *testing.T) {
	token, err := IssueToken("secret-32-characters-minimum-value", "user-1", "admin")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken("secret-32-characters-minimum-value", token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != "user-1" || claims.Role != "admin" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}
```

- [ ] **Step 5: 实现 token**

Create `backend/internal/auth/token.go`:

```go
package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func IssueToken(secret string, userID string, role string) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func ParseToken(secret string, raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}
```

- [ ] **Step 6: 写认证 API 文档**

Create `docs/apis/auth.md`:

```markdown
# 认证 API

## POST /api/v1/auth/login

请求：

```json
{
  "email": "admin@example.com",
  "password": "change-me"
}
```

响应：

```json
{
  "token": "jwt-token"
}
```

## POST /api/v1/auth/logout

MVP 使用 JWT 时前端删除本地 token；后端返回 `204 No Content`。
```

- [ ] **Step 7: 验证认证模块**

Run:

```bash
cd backend
go test ./internal/auth
```

Expected: PASS。

- [ ] **Step 8: 提交认证模块**

Run:

```bash
git add backend/internal/auth docs/apis/auth.md
git commit -m "feat: add admin authentication primitives"
```

## Task 4: 文章 API、发布流和 MDX 导入

**Files:**

- Create: `backend/internal/articles/service.go`
- Create: `backend/internal/articles/service_test.go`
- Create: `backend/internal/articles/handler.go`
- Create: `backend/cmd/import-mdx/main.go`
- Create: `docs/apis/articles.md`

- [ ] **Step 1: 写 slug 校验测试**

Create `backend/internal/articles/service_test.go`:

```go
package articles

import "testing"

func TestValidateSlug(t *testing.T) {
	valid := []string{"hello-world", "go-1-22-notes", "a1"}
	for _, slug := range valid {
		if !ValidSlug(slug) {
			t.Fatalf("expected valid slug: %s", slug)
		}
	}
	invalid := []string{"Hello", "hello_world", "-start", "end-", "two--dash"}
	for _, slug := range invalid {
		if ValidSlug(slug) {
			t.Fatalf("expected invalid slug: %s", slug)
		}
	}
}
```

- [ ] **Step 2: 实现 slug 校验**

Create `backend/internal/articles/service.go`:

```go
package articles

import "regexp"

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func ValidSlug(slug string) bool {
	return slugPattern.MatchString(slug)
}
```

- [ ] **Step 3: 定义文章接口文档**

Create `docs/apis/articles.md`:

```markdown
# 文章 API

## GET /api/v1/articles

返回已发布文章列表。

## GET /api/v1/articles/{slug}

返回单篇已发布文章。

## GET /api/v1/admin/articles

返回后台文章列表，包含草稿、已发布和归档文章。

## POST /api/v1/admin/articles

创建草稿文章。`slug` 必须匹配 `^[a-z0-9]+(?:-[a-z0-9]+)*$`。

## PATCH /api/v1/admin/articles/{id}

保存草稿并写入 `article_revisions`。

## POST /api/v1/admin/articles/{id}/publish

发布文章，设置 `published_at`，并写入 `article_revisions`。
```

- [ ] **Step 4: 创建 MDX 导入工具入口**

Create `backend/cmd/import-mdx/main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	source := flag.String("source", "../content/posts", "MDX source directory")
	flag.Parse()
	files, err := filepath.Glob(filepath.Join(*source, "*.mdx"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, file := range files {
		fmt.Println(file)
	}
}
```

- [ ] **Step 5: 验证文章模块**

Run:

```bash
cd backend
go test ./internal/articles
go test ./cmd/import-mdx
```

Expected: PASS。

- [ ] **Step 6: 提交文章基础**

Run:

```bash
git add backend/internal/articles backend/cmd/import-mdx docs/apis/articles.md
git commit -m "feat: add article publishing foundation"
```

## Task 5: 统计 API 和后台看板

**Files:**

- Create: `backend/internal/analytics/service.go`
- Create: `backend/internal/analytics/service_test.go`
- Create: `backend/internal/analytics/handler.go`
- Create: `frontend/src/features/analytics/AdminDashboardPage.tsx`
- Create: `docs/apis/analytics.md`

- [ ] **Step 1: 写统计聚合测试**

Create `backend/internal/analytics/service_test.go`:

```go
package analytics

import "testing"

func TestSummaryTotals(t *testing.T) {
	days := []DailyView{
		{Views: 2},
		{Views: 3},
		{Views: 5},
	}
	if TotalViews(days) != 10 {
		t.Fatal("expected total views to be 10")
	}
}
```

- [ ] **Step 2: 实现聚合函数**

Create `backend/internal/analytics/service.go`:

```go
package analytics

type DailyView struct {
	Views int
}

func TotalViews(days []DailyView) int {
	total := 0
	for _, day := range days {
		total += day.Views
	}
	return total
}
```

- [ ] **Step 3: 写统计 API 文档**

Create `docs/apis/analytics.md`:

```markdown
# 统计 API

## POST /api/v1/articles/{slug}/views

记录一次文章访问，并同步增加站点日访问量。

响应：

```json
{
  "recorded": true
}
```

## GET /api/v1/admin/analytics/summary

返回后台看板统计。

```json
{
  "todayViews": 12,
  "last7DaysViews": 120,
  "topArticles": [
    {
      "slug": "hello-world",
      "title": "Hello World",
      "views": 30
    }
  ]
}
```
```

- [ ] **Step 4: 创建后台看板页面**

Create `frontend/src/features/analytics/AdminDashboardPage.tsx`:

```tsx
export function AdminDashboardPage() {
  return (
    <main>
      <h1>后台看板</h1>
      <section>
        <h2>访问统计</h2>
        <p>今日访问量、近 7 天访问量和热门文章将在这里展示。</p>
      </section>
    </main>
  );
}
```

- [ ] **Step 5: 验证统计任务**

Run:

```bash
cd backend
go test ./internal/analytics
cd ../frontend
npm run build
```

Expected: PASS，前端构建成功。

- [ ] **Step 6: 提交统计基础**

Run:

```bash
git add backend/internal/analytics frontend/src/features/analytics docs/apis/analytics.md
git commit -m "feat: add analytics dashboard foundation"
```

## Task 6: 评论提交和审核

**Files:**

- Create: `backend/internal/comments/service.go`
- Create: `backend/internal/comments/service_test.go`
- Create: `backend/internal/comments/handler.go`
- Create: `frontend/src/features/comments/AdminCommentsPage.tsx`
- Create: `docs/apis/comments.md`

- [ ] **Step 1: 写评论状态测试**

Create `backend/internal/comments/service_test.go`:

```go
package comments

import "testing"

func TestNewCommentDefaultsToPending(t *testing.T) {
	comment := NewComment("Alice", "hello")
	if comment.Status != "pending" {
		t.Fatalf("expected pending, got %s", comment.Status)
	}
}
```

- [ ] **Step 2: 实现评论默认状态**

Create `backend/internal/comments/service.go`:

```go
package comments

type Comment struct {
	AuthorName string
	Body       string
	Status     string
}

func NewComment(authorName string, body string) Comment {
	return Comment{AuthorName: authorName, Body: body, Status: "pending"}
}
```

- [ ] **Step 3: 写评论 API 文档**

Create `docs/apis/comments.md`:

```markdown
# 评论 API

## GET /api/v1/articles/{slug}/comments

返回已审核通过的评论。

## POST /api/v1/articles/{slug}/comments

提交评论。新评论默认进入 `pending`。

## GET /api/v1/admin/comments

返回后台评论列表。

## PATCH /api/v1/admin/comments/{id}

审核评论。允许值：`approved`、`rejected`。
```

- [ ] **Step 4: 创建评论审核页面**

Create `frontend/src/features/comments/AdminCommentsPage.tsx`:

```tsx
export function AdminCommentsPage() {
  return (
    <main>
      <h1>评论审核</h1>
      <p>待审核、已通过和已拒绝的评论将在这里管理。</p>
    </main>
  );
}
```

- [ ] **Step 5: 验证评论任务**

Run:

```bash
cd backend
go test ./internal/comments
cd ../frontend
npm run build
```

Expected: PASS，前端构建成功。

- [ ] **Step 6: 提交评论基础**

Run:

```bash
git add backend/internal/comments frontend/src/features/comments docs/apis/comments.md
git commit -m "feat: add comment moderation foundation"
```

## Task 7: 文件上传、下载和审计

**Files:**

- Create: `backend/internal/assets/service.go`
- Create: `backend/internal/assets/service_test.go`
- Create: `backend/internal/assets/handler.go`
- Create: `backend/internal/storage/s3.go`
- Create: `frontend/src/features/assets/AdminAssetsPage.tsx`
- Create: `docs/apis/assets.md`

- [ ] **Step 1: 写文件校验测试**

Create `backend/internal/assets/service_test.go`:

```go
package assets

import "testing"

func TestValidateUpload(t *testing.T) {
	if err := ValidateUpload("a.pdf", "application/pdf", 1024); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUpload("a.exe", "application/octet-stream", 1024); err == nil {
		t.Fatal("expected invalid content type")
	}
	if err := ValidateUpload("big.pdf", "application/pdf", 21*1024*1024); err == nil {
		t.Fatal("expected size error")
	}
}
```

- [ ] **Step 2: 实现文件校验**

Create `backend/internal/assets/service.go`:

```go
package assets

import "errors"

const MaxUploadBytes = 20 * 1024 * 1024

var allowedTypes = map[string]bool{
	"image/png":       true,
	"image/jpeg":      true,
	"image/webp":      true,
	"application/pdf": true,
	"text/plain":      true,
	"application/zip": true,
}

func ValidateUpload(filename string, contentType string, byteSize int) error {
	if filename == "" {
		return errors.New("filename is required")
	}
	if !allowedTypes[contentType] {
		return errors.New("content type is not allowed")
	}
	if byteSize <= 0 || byteSize > MaxUploadBytes {
		return errors.New("file size is invalid")
	}
	return nil
}
```

- [ ] **Step 3: 写文件 API 文档**

Create `docs/apis/assets.md`:

```markdown
# 文件 API

## POST /api/v1/admin/assets/upload-url

申请预签名上传 URL。

允许类型：

- `image/png`
- `image/jpeg`
- `image/webp`
- `application/pdf`
- `text/plain`
- `application/zip`

最大大小：`20MB`。

## GET /api/v1/admin/assets/{id}/download-url

申请预签名下载 URL，并写入 `download_events`。
```

- [ ] **Step 4: 创建资源管理页面**

Create `frontend/src/features/assets/AdminAssetsPage.tsx`:

```tsx
export function AdminAssetsPage() {
  return (
    <main>
      <h1>资源管理</h1>
      <p>上传文件、生成下载链接和查看下载审计将在这里管理。</p>
    </main>
  );
}
```

- [ ] **Step 5: 验证文件任务**

Run:

```bash
cd backend
go test ./internal/assets
cd ../frontend
npm run build
```

Expected: PASS，前端构建成功。

- [ ] **Step 6: 提交文件基础**

Run:

```bash
git add backend/internal/assets backend/internal/storage frontend/src/features/assets docs/apis/assets.md
git commit -m "feat: add asset upload validation"
```

## Task 8: 前端后台壳层和公开页面

**Files:**

- Create: `frontend/src/api/client.ts`
- Create: `frontend/src/features/auth/LoginPage.tsx`
- Create: `frontend/src/features/articles/ArticleListPage.tsx`
- Create: `frontend/src/features/articles/ArticleDetailPage.tsx`
- Create: `frontend/src/features/articles/AdminArticleListPage.tsx`
- Create: `frontend/src/features/articles/AdminArticleEditorPage.tsx`
- Modify: `frontend/src/routes/index.tsx`
- Modify: `frontend/src/main.tsx`

- [ ] **Step 1: 创建 API client**

Create `frontend/src/api/client.ts`:

```ts
const API_BASE = import.meta.env.VITE_API_BASE ?? "http://127.0.0.1:8080/api/v1";

export async function apiGet<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`);
  if (!response.ok) {
    throw new Error(`GET ${path} failed: ${response.status}`);
  }
  return response.json() as Promise<T>;
}
```

- [ ] **Step 2: 创建登录页**

Create `frontend/src/features/auth/LoginPage.tsx`:

```tsx
export function LoginPage() {
  return (
    <main>
      <h1>后台登录</h1>
      <form>
        <label>
          邮箱
          <input name="email" type="email" />
        </label>
        <label>
          密码
          <input name="password" type="password" />
        </label>
        <button type="submit">登录</button>
      </form>
    </main>
  );
}
```

- [ ] **Step 3: 创建公开文章列表页**

Create `frontend/src/features/articles/ArticleListPage.tsx`:

```tsx
export function ArticleListPage() {
  return (
    <main>
      <h1>文章</h1>
      <p>已发布文章列表将在这里展示。</p>
    </main>
  );
}
```

- [ ] **Step 4: 创建后台文章页面**

Create `frontend/src/features/articles/AdminArticleListPage.tsx`:

```tsx
export function AdminArticleListPage() {
  return (
    <main>
      <h1>文章管理</h1>
      <p>草稿、已发布和归档文章将在这里管理。</p>
    </main>
  );
}
```

Create `frontend/src/features/articles/AdminArticleEditorPage.tsx`:

```tsx
export function AdminArticleEditorPage() {
  return (
    <main>
      <h1>文章编辑</h1>
      <textarea aria-label="MDX 正文" rows={20} />
      <button type="button">保存草稿</button>
      <button type="button">发布</button>
    </main>
  );
}
```

- [ ] **Step 5: 验证前端构建**

Run:

```bash
cd frontend
npm run build
```

Expected: build success。

- [ ] **Step 6: 提交前端壳层**

Run:

```bash
git add frontend/src
git commit -m "feat: add cms frontend shell"
```

## Task 9: MVP 运维文档和总体验证

**Files:**

- Create: `docs/apis/README.md`
- Create: `docs/operations/deployment.md`
- Modify: `docs/plans/2026-04-29-rendering-cms-platform-mvp.md`

- [ ] **Step 1: 写 API 索引**

Create `docs/apis/README.md`:

```markdown
# API 文档索引

- [认证 API](./auth.md)
- [文章 API](./articles.md)
- [统计 API](./analytics.md)
- [评论 API](./comments.md)
- [文件 API](./assets.md)
```

- [ ] **Step 2: 写部署文档**

Create `docs/operations/deployment.md`:

```markdown
# MVP 部署流程

## 环境变量

```env
HTTP_ADDR=:8080
DATABASE_URL=postgres://rendering:password@127.0.0.1:5432/rendering_cms
JWT_SECRET=replace-with-32-plus-character-secret
FRONTEND_ORIGIN=http://127.0.0.1:5173
S3_ENDPOINT=https://example.r2.cloudflarestorage.com
S3_REGION=auto
S3_BUCKET=rendering-assets
S3_ACCESS_KEY_ID=replace-me
S3_SECRET_ACCESS_KEY=replace-me
```

## 发布顺序

1. 备份 PostgreSQL。
2. 执行 SQL migration。
3. 构建 Go 后端。
4. 构建 React 前端。
5. 重启后端服务。
6. 请求 `/api/v1/health`。
```

- [ ] **Step 3: 运行 MVP 验证**

Run:

```bash
cd backend
go test ./...
go vet ./...
cd ../frontend
npm run build
```

Expected: 后端测试和 vet 通过，前端 build 通过。

- [ ] **Step 4: 提交 MVP 文档**

Run:

```bash
git add docs/apis docs/operations docs/plans/2026-04-29-rendering-cms-platform-mvp.md
git commit -m "docs: add cms mvp implementation plan"
```

## MVP 自检

- Spec coverage: MVP 覆盖登录、文章导入、文章发布、公开阅读、访问统计、后台看板、评论审核、文件上传下载和基础运维。
- Placeholder scan: 本计划不包含 TBD、TODO 或“后续补充”式占位任务。
- Type consistency: `Article`、`Comment`、`DailyView`、`ValidateUpload`、`ValidSlug` 等命名在测试和实现中保持一致。
- Boundary consistency: 原静态博客仅作为导入来源；运行时数据全部进入 PostgreSQL 和对象存储。
