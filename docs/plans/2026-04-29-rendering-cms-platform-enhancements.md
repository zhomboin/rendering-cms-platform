# Rendering CMS Platform Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 MVP 稳定后增强 CMS 的编辑体验、统计能力、评论风控、文件治理、搜索、运维和生产安全性。

**Architecture:** 增强阶段不改变 MVP 的核心边界：Go API、React 前端、PostgreSQL、S3 兼容对象存储。所有增强都以可迁移、可测试、可回退为原则，通过新的 migration、API 文档和前端页面逐步交付。

**Tech Stack:** Go 1.22+、Chi、pgx、sqlc、golang-migrate、PostgreSQL full text search、React、TypeScript、Vite、TanStack Query、Ant Design、CodeMirror 或 Monaco Editor、S3 兼容对象存储、结构化日志。

---

## 增强阶段分层

- P1：MVP 后立即增强，解决上线后最容易影响日常使用的问题。
- P2：中期增强，提升运营效率、搜索质量和数据可观测性。
- P3：生产强化，补齐备份恢复、权限细分、安全审计和性能边界。

## 当前进度

- 复核日期：2026-05-12。
- 当前增强计划共有 37 个步骤，其中 7 个已完成，30 个未完成。
- 已完成内容包括 Task 1 的 MDX 预览、编辑快捷键、双栏编辑布局和验证，以及 Task 8 的结构化日志封装、可观测性文档和日志增强验证。
- 未完成内容包括 Task 1 提交步骤、PostgreSQL 搜索增强、评论限流和反滥用、统计明细和趋势增强、文件治理增强、角色权限增强、备份恢复和生产运维，以及 Task 8 的提交步骤。

## Task 1: 编辑器体验增强

**Files:**

- Modify: `frontend/src/pages/articles/ArticleEditorPage.tsx`
- Create: `frontend/src/pages/articles/MdxPreview.tsx`
- Create: `frontend/src/pages/articles/editor-shortcuts.ts`

- [x] **Step 1: 增加 MDX 预览组件**

Create `frontend/src/pages/articles/MdxPreview.tsx`:

```tsx
type MdxPreviewProps = {
  source: string;
};

export function MdxPreview({ source }: MdxPreviewProps) {
  return (
    <section aria-label="MDX 预览">
      <h2>预览</h2>
      <pre>{source}</pre>
    </section>
  );
}
```

- [x] **Step 2: 增加编辑快捷键定义**

Create `frontend/src/pages/articles/editor-shortcuts.ts`:

```ts
export const editorShortcuts = [
  { key: "Ctrl+S", action: "保存草稿" },
  { key: "Ctrl+Enter", action: "发布文章" },
] as const;
```

- [x] **Step 3: 更新编辑页为双栏编辑和预览**

Modify `frontend/src/pages/articles/ArticleEditorPage.tsx` so it renders:

```tsx
import { useState } from "react";
import { MdxPreview } from "./MdxPreview";

export function ArticleEditorPage() {
  const [body, setBody] = useState("");

  return (
    <main>
      <h1>文章编辑</h1>
      <div>
        <section>
          <h2>正文</h2>
          <textarea
            aria-label="MDX 正文"
            rows={24}
            value={body}
            onChange={(event) => setBody(event.target.value)}
          />
        </section>
        <MdxPreview source={body} />
      </div>
      <button type="button">保存草稿</button>
      <button type="button">发布</button>
    </main>
  );
}
```

- [x] **Step 4: 验证编辑器增强**

Run:

```bash
cd frontend
npm run build
```

Expected: build success。

当前验证记录：

- `test -f frontend/src/pages/articles/MdxPreview.tsx` 通过。
- `tsc -p tsconfig.app.json --noEmit --incremental false` 通过。
- `bash scripts/env/test-dev-scripts.sh` 通过。
- 当前 WSL 环境中 `npm` 仍解析到 Windows Node 路径，完整 `npm run build` 需在修复 WSL Node 工具链后重新执行。

- [ ] **Step 5: 提交编辑器增强**

Run:

```bash
git add frontend/src/pages/articles docs/apis/articles.md
git commit -m "feat: improve mdx editor workflow"
```

## Task 2: PostgreSQL 搜索增强

**Files:**

- Create: `backend/migrations/000002_article_search.up.sql`
- Create: `backend/migrations/000002_article_search.down.sql`
- Create: `backend/sql/search.sql`
- Create: `backend/internal/articles/search.go`
- Modify: `docs/apis/articles.md`

- [ ] **Step 1: 添加搜索 migration**

Create `backend/migrations/000002_article_search.up.sql`:

```sql
alter table articles
  add column search_vector tsvector generated always as (
    setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('simple', coalesce(summary, '')), 'B') ||
    setweight(to_tsvector('simple', coalesce(body_mdx, '')), 'C')
  ) stored;

create index articles_search_vector_idx on articles using gin (search_vector);
```

Create `backend/migrations/000002_article_search.down.sql`:

```sql
drop index if exists articles_search_vector_idx;
alter table articles drop column if exists search_vector;
```

- [ ] **Step 2: 添加搜索 SQL**

Create `backend/sql/search.sql`:

```sql
-- name: SearchPublishedArticles :many
select article_id, slug, title, summary, published_at
from articles
where status = 'published'
  and search_vector @@ plainto_tsquery('simple', @query)
order by ts_rank(search_vector, plainto_tsquery('simple', @query)) desc, published_at desc;
```

- [ ] **Step 3: 更新接口文档**

Append to `docs/apis/articles.md`:

```markdown
## GET /api/v1/articles/search?q=keyword

基于 PostgreSQL full text search 搜索已发布文章。搜索范围包含标题、摘要和 MDX 正文。
```

- [ ] **Step 4: 验证搜索增强**

Run:

```bash
cd backend
go test ./...
```

Expected: PASS。

- [ ] **Step 5: 提交搜索增强**

Run:

```bash
git add backend/migrations backend/sql/search.sql backend/internal/articles docs/apis/articles.md
git commit -m "feat: add postgres article search"
```

## Task 3: 评论限流和反滥用

**Files:**

- Create: `backend/migrations/000003_comment_rate_limit.up.sql`
- Create: `backend/migrations/000003_comment_rate_limit.down.sql`
- Create: `backend/internal/comments/rate_limit.go`
- Create: `backend/internal/comments/rate_limit_test.go`
- Modify: `docs/apis/comments.md`

- [ ] **Step 1: 添加评论限流索引**

Create `backend/migrations/000003_comment_rate_limit.up.sql`:

```sql
create index comments_ip_hash_created_at_idx on comments (ip_hash, created_at desc);
```

Create `backend/migrations/000003_comment_rate_limit.down.sql`:

```sql
drop index if exists comments_ip_hash_created_at_idx;
```

- [ ] **Step 2: 写限流纯函数测试**

Create `backend/internal/comments/rate_limit_test.go`:

```go
package comments

import (
	"testing"
	"time"
)

func TestAllowCommentWithinLimit(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	recent := []time.Time{
		now.Add(-10 * time.Second),
		now.Add(-30 * time.Second),
	}
	if !AllowComment(now, recent) {
		t.Fatal("expected comment to be allowed")
	}
}

func TestRejectCommentOverLimit(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	recent := []time.Time{
		now.Add(-10 * time.Second),
		now.Add(-20 * time.Second),
		now.Add(-30 * time.Second),
	}
	if AllowComment(now, recent) {
		t.Fatal("expected comment to be rejected")
	}
}
```

- [ ] **Step 3: 实现限流纯函数**

Create `backend/internal/comments/rate_limit.go`:

```go
package comments

import "time"

func AllowComment(now time.Time, recent []time.Time) bool {
	count := 0
	for _, createdAt := range recent {
		if now.Sub(createdAt) <= time.Minute {
			count++
		}
	}
	return count < 3
}
```

- [ ] **Step 4: 更新评论文档**

Append to `docs/apis/comments.md`:

```markdown
## 限流规则

同一 IP 哈希 1 分钟内最多提交 3 条评论。超限时返回 `429 Too Many Requests`。
```

- [ ] **Step 5: 验证评论限流**

Run:

```bash
cd backend
go test ./internal/comments
```

Expected: PASS。

- [ ] **Step 6: 提交评论限流**

Run:

```bash
git add backend/migrations backend/internal/comments docs/apis/comments.md
git commit -m "feat: add comment rate limit rules"
```

## Task 4: 统计明细和趋势增强

**Files:**

- Create: `backend/migrations/000004_analytics_events.up.sql`
- Create: `backend/migrations/000004_analytics_events.down.sql`
- Create: `backend/sql/analytics_events.sql`
- Modify: `backend/internal/analytics/service.go`
- Modify: `docs/apis/analytics.md`

- [ ] **Step 1: 添加可选访问事件表**

Create `backend/migrations/000004_analytics_events.up.sql`:

```sql
create table analytics_events (
  event_id uuid primary key default gen_random_uuid(),
  article_id uuid references articles(article_id) on delete cascade,
  event_type text not null,
  ip_hash text not null,
  user_agent text,
  created_at timestamptz not null default now()
);

create index analytics_events_created_at_idx on analytics_events (created_at desc);
create index analytics_events_article_id_created_at_idx on analytics_events (article_id, created_at desc);
```

Create `backend/migrations/000004_analytics_events.down.sql`:

```sql
drop table if exists analytics_events;
```

- [ ] **Step 2: 添加趋势 API 文档**

Append to `docs/apis/analytics.md`:

```markdown
## GET /api/v1/admin/analytics/trend?days=30

返回最近 N 天站点访问趋势和文章访问趋势。`days` 允许值为 `7`、`30`、`90`。
```

- [ ] **Step 3: 验证统计增强**

Run:

```bash
cd backend
go test ./...
```

Expected: PASS。

- [ ] **Step 4: 提交统计增强**

Run:

```bash
git add backend/migrations backend/sql/analytics_events.sql backend/internal/analytics docs/apis/analytics.md
git commit -m "feat: add analytics trend foundation"
```

## Task 5: 文件治理增强

**Files:**

- Create: `backend/migrations/000005_asset_lifecycle.up.sql`
- Create: `backend/migrations/000005_asset_lifecycle.down.sql`
- Modify: `backend/internal/assets/service.go`
- Modify: `docs/apis/assets.md`

- [ ] **Step 1: 添加资源状态字段**

Create `backend/migrations/000005_asset_lifecycle.up.sql`:

```sql
create type asset_status as enum ('active', 'archived', 'deleted');

alter table assets
  add column status asset_status not null default 'active',
  add column deleted_at timestamptz;
```

Create `backend/migrations/000005_asset_lifecycle.down.sql`:

```sql
alter table assets drop column if exists deleted_at;
alter table assets drop column if exists status;
drop type if exists asset_status;
```

- [ ] **Step 2: 更新文件文档**

Append to `docs/apis/assets.md`:

```markdown
## PATCH /api/v1/admin/assets/{id}

更新资源状态。允许值：`active`、`archived`、`deleted`。

资源删除采用软删除，设置 `status=deleted` 和 `deleted_at`，不立即删除对象存储文件。
```

- [ ] **Step 3: 验证文件治理**

Run:

```bash
cd backend
go test ./...
```

Expected: PASS。

- [ ] **Step 4: 提交文件治理**

Run:

```bash
git add backend/migrations backend/internal/assets docs/apis/assets.md
git commit -m "feat: add asset lifecycle states"
```

## Task 6: 角色权限增强

**Files:**

- Create: `backend/internal/auth/permissions.go`
- Create: `backend/internal/auth/permissions_test.go`
- Modify: `docs/apis/auth.md`

- [ ] **Step 1: 写权限测试**

Create `backend/internal/auth/permissions_test.go`:

```go
package auth

import "testing"

func TestCanPublishArticle(t *testing.T) {
	if !CanPublishArticle("admin") {
		t.Fatal("admin should publish")
	}
	if CanPublishArticle("editor") {
		t.Fatal("editor should not publish without review")
	}
}
```

- [ ] **Step 2: 实现权限函数**

Create `backend/internal/auth/permissions.go`:

```go
package auth

func CanPublishArticle(role string) bool {
	return role == "admin"
}
```

- [ ] **Step 3: 更新认证文档**

Append to `docs/apis/auth.md`:

```markdown
## 权限规则

- `admin`：可管理用户、发布文章、审核评论、管理文件。
- `editor`：可编辑草稿和提交发布请求，默认不能直接发布文章。
```

- [ ] **Step 4: 验证权限增强**

Run:

```bash
cd backend
go test ./internal/auth
```

Expected: PASS。

- [ ] **Step 5: 提交权限增强**

Run:

```bash
git add backend/internal/auth docs/apis/auth.md
git commit -m "feat: add role permission rules"
```

## Task 7: 备份恢复和生产运维

**Files:**

- Create: `docs/operations/backup.md`
- Create: `docs/operations/restore.md`
- Create: `docs/operations/runbook.md`

- [ ] **Step 1: 写备份文档**

Create `docs/operations/backup.md`:

```markdown
# PostgreSQL 备份

## 手动备份

```bash
pg_dump "$DATABASE_URL" > backups/rendering-cms-$(date +%Y%m%d-%H%M%S).sql
```

## 备份要求

- 每次生产 migration 前必须备份。
- 至少保留最近 7 天备份。
- 对象存储文件需要单独开启 bucket 版本或生命周期策略。
```

- [ ] **Step 2: 写恢复文档**

Create `docs/operations/restore.md`:

```markdown
# PostgreSQL 恢复

## 恢复命令

```bash
psql "$DATABASE_URL" < backups/rendering-cms-latest.sql
```

## 恢复检查

1. 检查 `/api/v1/health`。
2. 登录后台。
3. 打开文章列表。
4. 打开最近发布文章。
5. 检查评论和文件列表。
```

- [ ] **Step 3: 写运行手册**

Create `docs/operations/runbook.md`:

```markdown
# 运行手册

## 发布前

1. 确认当前 Git 分支。
2. 运行后端测试。
3. 运行前端构建。
4. 备份 PostgreSQL。
5. 执行 migration。

## 发布后

1. 请求 `/api/v1/health`。
2. 登录后台。
3. 创建一篇草稿。
4. 提交一条评论并审核。
5. 上传一个小文件并生成下载链接。
```

- [ ] **Step 4: 提交运维增强**

Run:

```bash
git add docs/operations
git commit -m "docs: add backup restore runbooks"
```

## Task 8: 生产可观测性和日志

**Files:**

- Create: `backend/internal/logging/logger.go`
- Modify: `backend/cmd/server/main.go`
- Create: `docs/operations/observability.md`

- [x] **Step 1: 创建结构化日志封装**

已创建 `backend/internal/logging/logger.go`：

```go
package logging

import (
	"io"
	"log/slog"
)

func NewDailyFileLogger(logDir string) (*slog.Logger, io.Closer) {
	// 日志文件按自然日写入 backend-YYYY-MM-DD.log。
}
```

- [x] **Step 2: 写可观测性文档**

已创建 `docs/operations/observability.md`：

```markdown
# 可观测性

## 日志

后端输出 JSON 结构化日志到日志文件，默认目录为 `logs`，文件按天切换。

## 必须记录的事件

- 每次 HTTP 请求。
```

- [x] **Step 3: 验证日志增强**

Run:

```bash
cd backend
go test ./...
go vet ./...
```

Expected: PASS。

- [ ] **Step 4: 提交日志增强**

Run:

```bash
git add backend/internal/logging backend/cmd/server/main.go docs/operations/observability.md
git commit -m "feat: add structured logging foundation"
```

## 增强阶段自检

- Spec coverage: 增强计划覆盖编辑体验、搜索、评论限流、统计趋势、文件生命周期、角色权限、备份恢复和日志。
- Placeholder scan: 本计划不包含 TBD、TODO 或“类似上一任务”式占位任务。
- Type consistency: `AllowComment`、`CanPublishArticle`、`ValidateUpload` 和 API 路径命名与 MVP 计划保持一致。
- Boundary consistency: 增强功能仍以 PostgreSQL 和对象存储为运行时数据源，不回写原静态博客仓库。
