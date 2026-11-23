# 接入 AI 写作客户端

## 本地启动

1. 在 WSL 仓库根目录按[本地开发](development-environment.md#推荐开发启动顺序)准备本地配置、启动依赖并执行全部迁移，包括 `000020_mcp_console`。
2. 启动 CMS，在浏览器登录管理员账号，打开侧栏 **MCP 管理 → 服务设置**。

   ```bash
   bash scripts/env/start-dev-stack.sh
   ```

3. 填写 CMS API、管理后台和博客的来源（协议、域名和可选端口），开启服务并保存。后台与 API 同源时，后台来源可留空。接入地址由 CMS 自动拼上 `/api/v1/mcp`。修改地址时同步调整实际域名、反向代理和客户端配置。生产使用 HTTPS，开发环境的回环地址支持 HTTP。
4. 核对公开入口。服务开启后无凭据请求应返回 401，关闭后应返回 404。

   ```bash
   export MCP_ORIGIN='<CMS API 来源>'
   curl -i -X POST "$MCP_ORIGIN/api/v1/mcp"
   ```

初次使用时可直接在页面配置。部署变量 `MCP_ORIGIN`、`MCP_ADMIN_ORIGIN`、`MCP_BLOG_ORIGIN` 可提供初始来源；配置完整时初始开启，留空时初始关闭。管理员保存的地址和开关存入 PostgreSQL，对所有实例的后续请求生效。内网或本地博客需要服务端探测时，在部署配置中设置受信任的 `MCP_BLOG_ORIGIN`。后台填写的其他博客地址仅可探测公网。

## 签发和使用连接

1. 在 **AI 连接 → 新建连接** 为每个 AI 客户端单独命名。选择 Bearer 令牌，创建后复制接入地址和只显示一次的令牌。
2. 在 AI 客户端添加 Streamable HTTP 服务，粘贴 URL 和令牌。客户端若要求手工填写请求头，名称为 `Authorization`，值为 `Bearer <连接令牌>`。妥善保存凭据；撤销后重新签发会建立新的文章归属。
3. 让 AI “把这份笔记整理成草稿，返回预览链接”。打开预览核对内容。在 **AI 连接** 点击刷新，可以看到连接权限、最近上报的客户端名称和访问时间。无状态 HTTP 以请求记录反映使用情况。
4. 需要自动发文时，找到该连接并点击 **允许发布**。让 AI 刷新工具、读取草稿并按确认的版本调用 `publish_article`。需要恢复审稿时点击 **收回发布权**。**撤销** 会使该连接的凭据永久失效。
5. 打开 **操作记录**，刷新后可查看本人连接的调用时间、工具、结果和成功涉及的文章。结果待确认时先核对稿件状态。记录从启用该能力后的真实调用产生；日志仅保留操作元数据。

## OAuth 客户端

部署已有 OAuth 授权服务器时，可以让 AI 客户端完成委托授权。

1. 为 MCP 注册资源标识 `<MCP_ORIGIN>/api/v1/mcp`，在授权服务器登记客户端回调和范围 `articles:draft`、`articles:publish`。草稿范围是每个连接必需的基础权限。
2. 给 CMS 配置 `MCP_OAUTH_ISSUER` 和 `MCP_OAUTH_PUBLIC_KEY`。后者是至少 2048 位 RSA 验签公钥的 SPKI DER 的单行 Base64，可用 `openssl pkey -pubin -in '<公钥文件>' -outform DER | openssl base64 -A` 生成。签名私钥由授权服务器持有。验签公钥轮换时同步更新 CMS 配置并重启服务。
3. 管理员在「新建连接」选择 OAuth，填写名称以及授权服务器签发的客户端 ID 和主体。客户端连接 MCP URL，按 401 返回的资源元数据发现授权服务器。访问令牌使用 RS256、`typ=at+jwt`，包含 `iss`、目标 `aud`、`exp`、`sub`、`client_id` 和空格分隔的 `scope`。发布需要令牌范围与 CMS 连接授权同时允许。
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
2. 在获准的发布窗口按部署文档执行 migration、启动后端和前端、更新并校验代理。在后台核对服务设置，获准开启后，确认 `/api/v1/mcp` 未授权返回 401。启用 OAuth 时，`curl -fsS "$MCP_ORIGIN/.well-known/oauth-protected-resource/api/v1/mcp"` 应返回对应资源和发行方。
3. 创建独立草稿连接，完成试写、人工预览、重复请求、私有隔离与撤销。发布和搜索收录用另行授权的公开样稿验收。环境验收登记在[当前任务](../plans/current-tasks.md)。
4. 只需关闭某个客户端时撤销对应连接。需要关闭整个入口时，在「服务设置」关闭并保存。若需回退到仅依赖环境变量的应用镜像，先核对其 MCP 环境变量，防止回退时意外开放入口。业务文章保留；数据库恢复按[恢复说明](restore.md)单独审批执行。
