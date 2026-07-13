# CMS 公开文章 API 抗滥用与 Rendering 集成实施计划

> **供代理执行：** 必须使用 `superpowers:test-driven-development` 执行；每个行为变更先写并运行失败测试。实施环境固定为 WSL2 Ubuntu 24.04，命令使用 Bash。完成前使用 `superpowers:verification-before-completion` 运行完整验证。

**目标：** 缩小 CMS 公开文章接口的响应和查询成本，为 Rendering 提供稳定、可缓存、可限流的读取契约，并降低随机标识符、超长搜索词和高并发请求对 Go 服务与 PostgreSQL 的放大作用。

**架构：** handler 分离“公开列表摘要”“公开详情”“后台完整文章”三种响应模型；公共读接口输出明确缓存头和条件请求信息；Chi 中间件对公开读取按路由组施加有界并发和速率限制；Nginx 作为第一层限流与响应缓存。数据库层只做必要的有索引查询，并限制搜索结果数量。

**技术栈：** Go、Chi、pgx/sqlc、PostgreSQL、Nginx、Docker Compose。

## 执行状态（2026-07-13）

- [x] 公开列表摘要与详情 DTO 已拆分，列表不再返回正文和后台字段。
- [x] 标识符、搜索字符数和 SQL 结果数已设置硬边界。
- [x] 公开列表/详情已实现 Cache-Control、ETag、If-None-Match、304、404 负缓存和 5xx no-store。
- [x] 应用层令牌桶、并发限制、客户端状态 TTL、硬容量和共享溢出桶已实现并测试。
- [x] Go Server 已增加 `ReadHeaderTimeout`，生产环境变量与 Compose 传递已同步。
- [x] Nginx 公网限流、公开文章缓存、可信代理头、超时和缓冲配置已完成容器语法检查。
- [x] API、部署、可观测性和 Rendering Pagefind 发布触发责任已同步到中文文档。
- [ ] 待预发布环境执行高并发、高基数随机标识符、数据库 P95、goroutine 和 RSS 容量验收。
- [ ] 待生产机安装真实 Rendering location snippet 后执行 `sudo nginx -t`、`nginx -T` 和真实 `curl -I` 缓存验收。
- [ ] 待 Rendering 仓库安全的 CI/webhook 机制就绪后执行 Pagefind 构建联调；本任务未新增无认证 webhook。

---

## 一、现状证据与必须保持的契约

### 已确认问题

- `GET /api/v1/articles` 使用 `mapArticles()`，而 `mapArticle()` 包含 `bodyMdx`，导致公开列表返回全部文章正文。
- `GET /api/v1/articles/search` 只校验查询非空，没有字符数上限；`backend/sql/search.sql` 没有显式结果数上限。
- `GET /api/v1/articles/{slug}` 对合法短链接查询一次；非短链接直接按 `articleName` 查询，缺少长度和格式拒绝分支。
- 公共文章响应没有明确 `Cache-Control`、`ETag` 或 `Last-Modified`。
- Go `http.Server` 已有读写和空闲超时，但尚未设置 `ReadHeaderTimeout`。
- 生产 Nginx 已区分博客与 CMS upstream，但没有针对公开读、搜索、写入和登录的差异化限流。

### 保持不变

- `slug` 继续是 `^[0-9A-Za-z]{6}$` 的 canonical 短链接。
- 兼容期内详情接口继续接受合法 `articleName`，并返回 `resolvedBy: "articleName"` 和真实 `canonicalSlug`。
- 列表继续返回 `isFeatured`、`featuredRank`、`featuredAt`。
- 详情继续返回完整 `bodyMdx`。
- 未发布文章和不存在文章继续返回 404。
- 不修改数据库 schema；本计划不需要 migration。

## 二、文件职责

### 新增

- `backend/internal/http/rate_limit.go`：可测试的令牌桶限流器和有界客户端状态清理。
- `backend/internal/http/rate_limit_test.go`：限流、突发、清理和可信客户端地址测试。

### 修改

- `backend/internal/articles/handler.go`：公共 DTO、输入限制、缓存头和条件请求。
- `backend/internal/articles/handler_test.go`：列表瘦身、标识符、搜索长度和缓存语义。
- `backend/sql/search.sql`：限制搜索结果数量。
- `backend/internal/database/dbgen/search.sql.go`：仅通过 `sqlc generate` 重新生成。
- `backend/internal/http/router.go`、`backend/internal/http/router_test.go`：公共路由限流装配。
- `backend/internal/config/config.go`、`backend/internal/config/config_test.go`：限流参数与安全默认值。
- `backend/cmd/server/main.go`：传入限流配置并补充 Header 超时。
- `deploy/nginx/rendering.me.conf`、`deploy/nginx/frontend.conf`：边界限流、缓存和代理超时。
- `deploy/production.env.example`：限流环境变量示例。
- `docs/apis/articles.md`：精确记录公开列表与详情字段差异、缓存和错误码。
- `docs/operations/` 下现有生产部署文档：限流调优、监控与回滚。

## 三、任务 1：拆分公共列表摘要与完整详情 DTO

**文件：**

- 修改：`backend/internal/articles/handler.go`
- 修改：`backend/internal/articles/handler_test.go`
- 修改：`docs/apis/articles.md`

- [ ] **步骤 1：先写失败测试**

扩展 `TestListPublishedArticlesReturnsHomeSectionFields`，明确断言列表包含：

```text
slug, canonicalSlug, articleName, title, summary, tags,
publishedAt, updatedAt, isFeatured, featuredRank, featuredAt, coverImageUrl
```

同时断言列表不包含：

```text
bodyMdx, authorId, version, articleId
```

详情测试必须继续断言 `bodyMdx`、`canonicalSlug` 和 `resolvedBy` 存在。

- [ ] **步骤 2：运行失败测试**

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
go test ./internal/articles -run 'TestListPublishedArticles|TestGetPublishedArticle' -count=1
```

预期：列表仍包含 `bodyMdx`，测试失败。

- [ ] **步骤 3：实现独立映射函数**

在 `handler.go` 增加：

```go
func mapPublicArticleSummary(article articleView) map[string]interface{} { /* 仅摘要字段 */ }
func mapPublicArticleDetail(article articleView) map[string]interface{} { /* 摘要字段 + bodyMdx */ }
```

`listPublishedArticles` 使用 summary 映射；`getPublishedArticle` 使用 detail 映射；后台接口继续使用包含管理字段的映射。不要通过“先完整映射再 delete 字段”实现，避免未来新增敏感字段时意外泄露。

- [ ] **步骤 4：运行测试并记录响应体下降**

```bash
go test ./internal/articles -count=1
```

在含真实文章的预发布数据上分别记录改动前后：

```bash
curl -sS -o /tmp/articles.json -w '%{size_download}\n' https://cms.rendering.me/api/v1/articles
```

- [ ] **步骤 5：提交**

```bash
git add backend/internal/articles/handler.go backend/internal/articles/handler_test.go docs/apis/articles.md
git commit -m "fix: slim public article list responses"
```

## 四、任务 2：限制文章标识符和搜索成本

**文件：**

- 修改：`backend/internal/articles/handler.go`
- 修改：`backend/internal/articles/handler_test.go`
- 修改：`backend/sql/search.sql`
- 生成：`backend/internal/database/dbgen/search.sql.go`
- 修改：`docs/apis/articles.md`

- [ ] **步骤 1：写标识符失败测试**

测试以下详情标识符直接返回 404 或 400，且 store 的两个查询方法都未调用：

- 超过 128 字符；
- 包含 `/`、反斜杠、控制字符、空格或大写 legacy 名；
- 不符合短链接且不符合 `^[a-z0-9]+(?:-[a-z0-9]+)*$`。

保留合法 6 位短链接和合法 `articleName` 的兼容测试。

- [ ] **步骤 2：写搜索边界失败测试**

约定：去除首尾空白后，查询长度为 1–100 个 Unicode 字符；超过上限返回 400，且不访问数据库。测试中文按 rune 而不是字节计数。

- [ ] **步骤 3：运行失败测试**

```bash
go test ./internal/articles -run 'TestGetPublishedArticle|TestSearchPublishedArticles' -count=1
```

- [ ] **步骤 4：实现 handler 校验**

`resolvePublishedArticle` 的顺序：

1. 长度和字符格式校验；
2. `ValidSlug` 为真时只按 slug 查；
3. `ValidArticleName` 为真时只按 articleName 查；
4. 其他输入返回专用无效标识符错误，handler 映射为 404，避免泄露内部规则。

搜索使用 `utf8.RuneCountInString` 限制 100 字符。

- [ ] **步骤 5：给 SQL 增加固定上限**

在 `backend/sql/search.sql` 的排序后增加：

```sql
limit 20;
```

然后只通过 sqlc 生成代码：

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
sqlc generate
git diff -- backend/internal/database/dbgen/search.sql.go
```

确认生成差异只对应 `LIMIT 20`，不要手改 dbgen 文件。

- [ ] **步骤 6：验证并提交**

```bash
go test ./internal/articles ./internal/database/... -count=1
git add backend/internal/articles/handler.go backend/internal/articles/handler_test.go backend/sql/search.sql backend/internal/database/dbgen/search.sql.go docs/apis/articles.md
git commit -m "fix: bound public article queries"
```

## 五、任务 3：增加公共读取缓存与条件请求

**文件：**

- 修改：`backend/internal/articles/handler.go`
- 修改：`backend/internal/articles/handler_test.go`
- 修改：`docs/apis/articles.md`

- [ ] **步骤 1：写缓存语义失败测试**

列表和详情成功响应应包含：

```http
Cache-Control: public, max-age=60, stale-while-revalidate=300
Vary: Accept-Encoding
ETag: "..."
```

请求携带匹配的 `If-None-Match` 时返回 304、无响应体。404 使用短负缓存，例如：

```http
Cache-Control: public, max-age=15
```

5xx 必须使用 `Cache-Control: no-store`。

- [ ] **步骤 2：验证测试失败**

```bash
go test ./internal/articles -run 'Cache|ETag|NotModified' -count=1
```

- [ ] **步骤 3：实现稳定 ETag**

ETag 必须基于最终 JSON 字节生成 SHA-256，而不是当前时间。重构 `writeJSON` 或新增公共响应专用 helper：先 `json.Marshal`，计算 ETag，检查 `If-None-Match`，再写 body。错误响应单独设置缓存策略。

不要让后台管理接口获得公共缓存头。

- [ ] **步骤 4：验证并提交**

```bash
go test ./internal/articles -count=1
git add backend/internal/articles/handler.go backend/internal/articles/handler_test.go docs/apis/articles.md
git commit -m "feat: cache public article reads"
```

## 六、任务 4：增加应用层速率与并发限制

**文件：**

- 新增：`backend/internal/http/rate_limit.go`
- 新增：`backend/internal/http/rate_limit_test.go`
- 修改：`backend/internal/http/router.go`
- 修改：`backend/internal/http/router_test.go`
- 修改：`backend/internal/config/config.go`
- 修改：`backend/internal/config/config_test.go`
- 修改：`backend/cmd/server/main.go`
- 修改：`deploy/production.env.example`

- [ ] **步骤 1：写限流器失败测试**

使用可注入时钟，覆盖：

- 同一客户端允许配置的持续速率和 burst，超限返回 429 与 `Retry-After`；
- 不同客户端桶隔离；
- 客户端状态在空闲 TTL 后清理，Map 不无界增长；
- 客户端状态达到硬上限后不再创建独立条目，而是进入有速率限制的共享溢出桶；持续输入新 IP 时独立状态数始终不超过上限；
- 并发超过上限时立即返回 503 或 429，不等待占用 goroutine；
- 客户端身份只使用已经过可信代理重写的 `X-Real-IP` 或 `RemoteAddr`，不信任任意 `X-Forwarded-For` 链。

- [ ] **步骤 2：验证测试失败**

```bash
go test ./internal/http -run 'RateLimit|ConcurrencyLimit' -count=1
```

- [ ] **步骤 3：实现有界限流中间件**

配置默认值建议：

```text
PUBLIC_READ_RATE_PER_SECOND=20
PUBLIC_READ_BURST=40
PUBLIC_SEARCH_RATE_PER_SECOND=5
PUBLIC_SEARCH_BURST=10
PUBLIC_MAX_IN_FLIGHT=128
PUBLIC_RATE_LIMIT_MAX_CLIENTS=10000
```

限流器必须同时具备空闲 TTL 清理和硬容量上限；不得使用只依赖 TTL、在窗口内仍可无界增长的 `map[ip]*Limiter`。达到 `PUBLIC_RATE_LIMIT_MAX_CLIENTS` 后，新客户端共用一个严格的 overflow limiter，且不写入客户端 Map；不得通过无上限扩容或为每次溢出创建新对象处理。搜索接口使用更严格桶。应用层参数是第二道防线，不能代替 Nginx/CDN。

- [ ] **步骤 4：路由分组装配**

不要对 health check 和后台管理路由套用同一公共桶。建议把公共文章路由注册到独立 Chi 子路由，按列表/详情和搜索分别装配中间件；登录、评论和分析继续使用各自安全策略。

- [ ] **步骤 5：补充 HTTP Server 超时**

在 `backend/cmd/server/main.go` 增加：

```go
ReadHeaderTimeout: 5 * time.Second,
```

保留现有 `ReadTimeout`、`WriteTimeout`、`IdleTimeout` 和 `MaxHeaderBytes`。

- [ ] **步骤 6：验证并提交**

```bash
go test ./internal/http ./internal/config ./cmd/server -count=1
git add backend/internal/http/rate_limit.go backend/internal/http/rate_limit_test.go backend/internal/http/router.go backend/internal/http/router_test.go backend/internal/config/config.go backend/internal/config/config_test.go backend/cmd/server/main.go deploy/production.env.example
git commit -m "fix: limit public CMS request pressure"
```

## 七、任务 5：强化 CMS Nginx 边界

**文件：**

- 修改：`deploy/nginx/rendering.me.conf`
- 修改：`deploy/nginx/frontend.conf`
- 修改：`docs/operations/` 下对应生产部署文档

- [ ] **步骤 1：设计差异化限流区**

建议初值：

```nginx
limit_req_zone $binary_remote_addr zone=cms_public_read:10m rate=10r/s;
limit_req_zone $binary_remote_addr zone=cms_public_search:10m rate=2r/s;
limit_req_zone $binary_remote_addr zone=cms_write:10m rate=1r/s;
limit_conn_zone $binary_remote_addr zone=cms_conn:10m;
limit_req_zone $binary_remote_addr zone=rendering_general:10m rate=10r/s;
limit_req_zone $binary_remote_addr zone=rendering_article:10m rate=3r/s;
limit_conn_zone $binary_remote_addr zone=rendering_conn:10m;
```

公开列表/详情、搜索、analytics view、评论、登录和后台写入必须按不同 location 或 method 策略处理，不能给昂贵搜索与普通 GET 相同额度。

该文件是 `rendering.me`、`www.rendering.me` 和 `cms.rendering.me` 公网 `server`/`server_name` 的唯一所有者。Rendering 仓库只提供 `deploy/nginx/rendering-locations.conf` snippet；在博客 HTTPS server 内显式加入：

```nginx
include /etc/nginx/snippets/rendering-locations.conf;
```

加入 include 时必须删除该博客 HTTPS server 中原有的 `location /`，由 snippet 提供唯一默认 location，避免重复定义。snippet 必须让 `/blog/` 和 `/en/blog/` 使用 `rendering_article`，让默认 `location /` 使用 `rendering_general`，并在每个代理 location 中显式保留 `proxy_http_version`、Host、可信客户端 IP、协议、Upgrade 和 Connection 头。不要安装第二个声明相同 `listen` 或 `server_name` 的完整虚拟主机。

- [ ] **步骤 2：配置代理超时与缓冲**

CMS location 至少设置：

```nginx
proxy_connect_timeout 3s;
proxy_send_timeout 15s;
proxy_read_timeout 30s;
proxy_buffering on;
proxy_request_buffering on;
limit_conn cms_conn 20;
limit_req_status 429;
limit_conn_status 429;
```

保留 `X-Real-IP $remote_addr` 和 `X-Forwarded-For $remote_addr` 的可信边界，不改回 `$proxy_add_x_forwarded_for` 后直接信任客户端头。

- [ ] **步骤 3：只缓存公开 GET**

如果启用 Nginx `proxy_cache`：

- 只缓存 `GET /api/v1/articles` 和 `GET /api/v1/articles/{identifier}` 的 200/404；
- 搜索可短缓存，后台、登录、评论、view 统计和带 Authorization/Cookie 的请求全部 bypass；
- cache key 包含 scheme、host、URI 和 query string；
- 尊重后端 `Cache-Control` 与 ETag；
- 设置 `proxy_cache_lock on`，避免热点文章缓存失效时请求惊群。

- [ ] **步骤 4：验证配置**

```bash
cd /home/ubuntu/workspace/rendering-cms-platform
docker compose -f deploy/docker-compose.prod.yml config
sudo nginx -t
sudo nginx -T | grep -n 'server_name rendering.me'
```

验证的是已经安装 Rendering snippet 与 CMS 主配置后的组合结果；`rendering.me` 的 HTTPS server 必须只有一个有效定义。使用 `curl -I` 验证公开接口缓存头，并连续请求观察 `X-Cache-Status`（若配置该调试头）。确认后台请求不命中公共缓存。

- [ ] **步骤 5：提交**

```bash
git add deploy/nginx/rendering.me.conf deploy/nginx/frontend.conf docs/operations
git commit -m "ops: harden CMS public API proxy"
```

## 八、任务 6：同步接口文档和 Rendering 发布触发

**文件：**

- 修改：`docs/apis/articles.md`
- 修改：`docs/apis/README.md`
- 修改：`docs/operations/` 下发布文档

- [ ] **步骤 1：记录精确响应分区**

文档必须用字段表明确：

- 列表返回摘要字段，不返回 `bodyMdx`；
- 详情返回 `bodyMdx`、`canonicalSlug`、`resolvedBy`；
- 搜索最多 100 Unicode 字符、最多 20 条结果、不返回正文；
- 400、404、429、500 的语义与 `Retry-After`；
- Cache-Control、ETag、If-None-Match 和 304 行为。

- [ ] **步骤 2：记录搜索更新边界**

Rendering 当前使用 Pagefind 构建期索引，不调用 CMS 搜索接口。CMS 发布成功后必须触发 Rendering 构建/部署 webhook 或 CI workflow；触发凭据必须放在 secrets，不能写入数据库或仓库。触发失败不得回滚已发布文章，但必须告警“文章已发布、搜索索引未更新”。

本任务只更新契约和运维流程；若仓库当前没有安全的跨仓库触发机制，应另开任务实现，不在本次硬化中临时加入无认证 webhook。

- [ ] **步骤 3：文档检查并提交**

```bash
git diff --check
git add docs/apis docs/operations
git commit -m "docs: define bounded public article contract"
```

## 九、任务 7：完整验证与容量验收

- [ ] **步骤 1：运行后端验证**

```bash
cd /home/ubuntu/workspace/rendering-cms-platform/backend
gofmt -w internal/http/rate_limit.go internal/http/rate_limit_test.go internal/http/router.go internal/http/router_test.go internal/articles/handler.go internal/articles/handler_test.go internal/config/config.go internal/config/config_test.go cmd/server/main.go
sqlc generate
go test ./...
go vet ./...
```

- [ ] **步骤 2：运行仓库级验证**

```bash
cd /home/ubuntu/workspace/rendering-cms-platform
docker compose -f deploy/docker-compose.prod.yml config
git diff --check
git status --short
```

执行前后都检查现有未提交的 `scripts/env/` 和 `scripts/ops/` 修改，禁止把它们混入本任务提交。

- [ ] **步骤 3：预发布压测**

仅在预发布环境执行：

1. 高并发请求同一真实短链接，确认 Nginx cache lock/后端缓存使数据库 QPS 不线性增长。
2. 高基数随机 6 位短链接和持续变化的模拟客户端 IP，确认 Nginx 与应用限流生效、独立 limiter 状态不超过 `PUBLIC_RATE_LIMIT_MAX_CLIENTS`、溢出请求进入共享桶，且 404 短缓存不导致内存无界增长。
3. 超长 articleName 和 100 字符以上搜索词，确认不访问数据库。
4. 并发搜索，确认结果最多 20 条，数据库 P95 和连接池使用率稳定。
5. 检查 429、5xx、数据库慢查询、Go goroutine、RSS 内存和 Nginx upstream 响应时间。

- [ ] **步骤 4：联调 Rendering**

确认：

- Rendering 列表页仍取得 featured/recent 字段；
- 文章详情仍取得 `bodyMdx`；
- legacy `articleName` 请求返回 canonicalSlug 并被 Rendering 永久重定向；
- 304 能被 Rendering/代理正确处理；
- CMS 发布后 Rendering 构建生成的新 Pagefind 索引能搜索到文章。

## 十、发布与回滚顺序

### 发布

1. 先部署 CMS handler 的列表瘦身、输入限制、缓存头与 SQL limit。
2. 再部署 CMS 应用层限流和 Header 超时。
3. 部署 Rendering 消费端有界读取与安全渲染。
4. 最后启用 Nginx 差异化限流和缓存；先灰度观察，再逐步收紧。

### 回滚

- Nginx 误伤：先提高 burst 或临时关闭单个 location 的 `limit_req`，不要回滚可信代理头修复。
- Rendering 不兼容列表瘦身：恢复旧 CMS 版本，同时修复 Rendering 不应依赖列表正文的问题。
- ETag/304 异常：临时关闭条件请求 helper，但保留 Cache-Control、输入限制和响应瘦身。
- 应用限流状态异常：关闭应用层限流配置并保留 Nginx 层，随后修复清理逻辑。

## 十一、非目标

- 本计划不修改文章表结构，不新增 migration。
- 不把 CMS 搜索接口直接接入 Rendering 前端。
- 不依赖单机限流抵御网络层大流量 DDoS；生产源站仍应置于 CDN/WAF 后，并限制非 CDN 回源流量。
- 不保存原始客户端 IP；日志和应用状态继续使用哈希或瞬时内存标识。
