# 接入 AI 写作客户端

## 本地启动

1. 在 WSL 仓库根目录按[本地开发](development-environment.md#推荐开发启动顺序)准备被 Git 忽略的 `.env`、启动依赖，并执行全部 migration，包含 `000019_article_mcp`。
2. 在同一份 `.env` 设置 `MCP_ORIGIN` 为 AI 客户端能访问的 CMS API 来源，`MCP_ADMIN_ORIGIN` 为浏览器访问后台的来源，`MCP_BLOG_ORIGIN` 为 Go 服务能访问的 Rendering 博客来源。三个值均只含协议、主机和可选端口。后台和 API 同源时可以省略 `MCP_ADMIN_ORIGIN`。开发模式允许回环 HTTP，容器内的回环指向容器自身。
3. 启动开发栈，检查未携带令牌时的响应。请把占位符替换成本地配置中的值。

   ```bash
   bash scripts/env/start-dev-stack.sh
   export MCP_ORIGIN='<CMS API 来源>'
   curl -i -X POST "$MCP_ORIGIN/api/v1/mcp"
   ```

   预期返回 401。所有 MCP 变量留空时两个 MCP 入口均关闭。

## 签发和使用连接

1. 通过正常后台登录并完成需要的邮箱验证。仅在管理员自己的终端输入当前会话的访问令牌。以下命令从仓库根目录执行，使用 Bash、curl 和 jq。

   ```bash
   read -rsp '管理员访问令牌：' CMS_ADMIN_TOKEN
   printf '\n'
   umask 077
   mkdir -p temp
   grant=$(mktemp temp/mcp-grant.XXXXXXXX)
   connections="$MCP_ORIGIN/api/v1/admin/mcp/connections"
   curl --fail-with-body -sS -H "Authorization: Bearer $CMS_ADMIN_TOKEN" \
     --json '{"label":"博客写作"}' "$connections" > "$grant"
   id=$(jq -er '.id' "$grant")
   ```

2. 在支持自定义 Bearer 请求头的 AI 客户端添加远程 MCP 服务。URL 取 `jq -r '.endpoint' "$grant"`，传输选 Streamable HTTP，令牌取该文件的 `token` 并写入客户端的凭据存储。管理员登录令牌仅用于管理连接。个人连接令牌持续有效至撤销，应按连接独立保管；轮换令牌会创建新的连接和文章归属。
3. 在客户端说“把这份笔记整理成草稿，返回预览链接”。打开后台预览后，可以指示 AI 修改，或在 CMS 中手动发布。无确认机制的客户端保持草稿权限即可。
4. 对已选定的自动发布任务开启发布权。AI 客户端重新列举工具后可调用 `publish_article`；任务按 `get_article` 返回的版本发布。无人值守的触发时间和选题在外部调度器设置。

   ```bash
   curl --fail-with-body -sS -X PATCH -H "Authorization: Bearer $CMS_ADMIN_TOKEN" \
     --json '{"allowPublish":true}' "$connections/$id"
   ```

5. 需要恢复审稿时，把上述请求中的值改为 `false`。要结束整条连接，可执行以下撤销命令。撤销后新请求返回 401。

   ```bash
   curl --fail-with-body -sS -X DELETE -H "Authorization: Bearer $CMS_ADMIN_TOKEN" "$connections/$id"
   unset CMS_ADMIN_TOKEN
   ```

## OAuth 客户端

部署已有 OAuth 授权服务器时，可以让 AI 客户端完成委托授权。

1. 为 MCP 注册资源标识 `<MCP_ORIGIN>/api/v1/mcp`，在授权服务器登记客户端回调和范围 `articles:draft`、`articles:publish`。草稿范围是每个连接必需的基础权限。
2. 给 CMS 配置 `MCP_OAUTH_ISSUER` 和 `MCP_OAUTH_PUBLIC_KEY`。后者是至少 2048 位 RSA 验签公钥的 SPKI DER 的单行 Base64，可用 `openssl pkey -pubin -in '<公钥文件>' -outform DER | openssl base64 -A` 生成。签名私钥由授权服务器持有。验签公钥轮换时同步更新 CMS 配置并重启服务。
3. 管理员按上节步骤调用 `POST /connections`，请求体同时填写 `label`、`oauthClient` 与 `oauthSubject`，值与授权服务器签发的客户端 ID、主体完全一致。客户端连接 MCP URL，按 401 返回的资源元数据发现授权服务器。访问令牌使用 RS256、`typ=at+jwt`，包含 `iss`、目标 `aud`、`exp`、`sub`、`client_id` 和空格分隔的 `scope`。发布需要令牌范围与 CMS 连接授权同时允许。
4. 用真实客户端核对登录、工具发现、草稿隔离、增减权限和撤销。撤销 CMS 连接立即阻断后续调用；仅在发行方撤销已签发的自包含令牌时，CMS 本地验签以令牌过期时间为界。

## 核验博客搜索

从有权核验该文章的管理员终端运行下面的脚本。它从 CMS 读取已发布版本，用独立 Chromium 查询博客的 `/pagefind/pagefind.js`，逐项比对结果 URL，并把结果和版本送回 CMS。浏览器只接触公开索引，管理员凭据仅发给 CMS。真实结果可供 AI 下一次 `article_status` 查询。

```bash
(cd frontend && npm ci && npx playwright install chromium)
export MCP_ORIGIN='<CMS API 来源>'
read -rsp '管理员访问令牌：' CMS_ADMIN_TOKEN
printf '\n'
CMS_ADMIN_TOKEN="$CMS_ADMIN_TOKEN" node scripts/env/verify-article-index.mjs '<文章 UUID>'
unset CMS_ADMIN_TOKEN
```

当博客重建或更新索引后应再次核验。索引不可读或验证期间文章版本变化时脚本以非零状态退出。页面和静态索引的发布联动按[博客发布对接](rendering-blog-publishing-integration.md)验收。

## 生产接入与回退

1. 按[部署流程](deployment.md#首次部署)核对候选提交与镜像，按[备份流程](backup.md)备份 PostgreSQL 并验证备份。在不入库的生产配置中填写 HTTPS 来源及可选 OAuth 配置。
2. 在获准的发布窗口按部署文档执行 migration、启动后端和前端、更新并校验代理。确认 `/api/v1/mcp` 未授权返回 401。启用 OAuth 时，`curl -fsS "$MCP_ORIGIN/.well-known/oauth-protected-resource/api/v1/mcp"` 应返回对应资源和发行方。
3. 创建独立草稿连接，完成试写、人工预览、重复请求、私有隔离与撤销。发布和搜索收录用另行授权的公开样稿验收。环境验收登记在[当前任务](../plans/current-tasks.md)。
4. 只需关闭某个客户端时撤销对应连接。需要关闭整个入口时清空全部 MCP 配置，并按部署流程重新创建后端容器。业务文章保留；数据库恢复按[恢复说明](restore.md)单独审批执行。
