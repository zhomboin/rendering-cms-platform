# 生产访问与登录 SOP

本文档记录生产 SSH、CMS 后台、PostgreSQL、Cloudflare R2 和宿主机 Nginx 的访问方式。SSH 从本地发起，其余命令默认在 Ubuntu 服务器执行，应用目录默认是 `/opt/rendering-cms-platform`。

## 本地连接配置

本机专用配置统一记录在主工作区的 `docs/operations/local-config.local.md`。该文件仅供本地使用，已通过 `.gitignore` 和 `.dockerignore` 排除；仓库、PR 和生产部署不保存或分发其内容。新工作区不会自动获得此文件，需要回到主工作区读取或由使用者在本机维护。

本地记录包括生产 SSH 地址、端口、登录用户、私钥及主机公钥文件位置、可信主机指纹、网关配置与固定操作清单、开发环境变量及其它本机配置入口。秘密值仍保存在原始配置文件或凭据管理工具中，本地记录只说明读取位置，不重复保存密码、令牌或密钥内容。不得以公共文档、提交消息、PR 正文或检视评论代替本地记录。

连接前核对本地记录与原始配置文件。指定唯一认证密钥，启用严格主机校验和非交互认证，关闭代理转发、端口转发及后台连接复用，并设置连接和失联超时。主机指纹或认证异常时停止核查，不删除主机记录、不自动接受新指纹、不关闭校验、不放宽私钥权限。只读验证完成后立即退出连接。

连接权限不代表生产写入授权。操作规范不会在技术上降低登录账号已有的系统权限；所有操作仍须遵守下方授权和固定操作要求。

## 生产操作边界

- 诊断默认只读，限任务相关的服务、容器、有限日志、资源及数据库汇总；数据库查询须使用只读事务并限制返回量，禁止借诊断导出私有正文、用户凭据或完整生产配置。
- 生产写操作即使已写入文档或网关清单，也必须属于用户明确授权的任务范围；已批准的修复只按批准范围实施，不重复索要已有授权。
- 修复权限或单元配置后，分别验证部署用户的操作权限、systemd 加载结果、自动部署日志及网站访问。
- 触发自动部署前，确认脚本是否重置工作区，保留服务器本地定制；不得把配置副本、凭据或备份提交进 Git。
- 未获批准不得顺带调整备份策略、清理数据或修改其它服务；任务结束后关闭临时诊断进程。

### 固定操作与高级操作

固定操作的实际清单、目标标识和调用身份统一记录在本地配置文档，调用前核对服务实际加载的清单和参数限制；本地文件存在不代表服务已经加载。不得把本地配置复制进 Git，也不得把端口可连接当成调用授权。

| 范围 | 允许方式与边界 |
| --- | --- |
| 只读诊断 | 仅使用已登记的固定目标与参数，遵守输出上限和超时；不得临时拼装命令扩大读取范围 |
| 服务重启、部署和定时器变更 | 仅在用户明确授权的任务范围内，按对应运维 SOP 和已登记固定操作执行；不得跨服务借用操作 |
| 未定义高级操作 | 先补齐操作说明并获批，不能以直接 SSH 或任意 Shell 绕过登记限制 |

未定义的高级操作一律停止，不临时拼装执行。高级操作包括任意 Shell 或脚本执行、部署或回滚、数据库写入或恢复、文件删除或归档、权限或服务配置变更、安装软件、网络或 SSH 配置变更、备份策略调整及其它改变生产状态的操作。

新增操作必须先在对应运维文档写清：目标与执行身份、固定入口和参数、影响范围、前置检查、备份或回退方式、超时、验收及清理条件。公共文档只描述通用规则；具体主机、账号、路径和实际参数只记录在本地配置文档。用户批准该具体范围后才能执行；通过网关执行时还须完成固定操作登记和校验。不得通过直接 SSH、修改白名单、替换固定程序或哈希、传入任意命令参数来绕过限制。工具缺失或访问被拒绝时停止受影响操作并说明原因。

执行后检查退出码、相关服务和健康状态；取消网关任务不等于生产回滚，停止定时器不等于停止正在运行的部署。只结束本次临时启动的连接、日志跟踪和辅助进程，不停止原有业务服务或已有定时任务。

## 入口总览

| 系统 | 访问方式 | 用途 |
| --- | --- | --- |
| 生产主机 | 从主工作区本地配置记录获取连接参数 | 连接验证及获准的生产运维 |
| CMS 后台 | `https://cms.rendering.me` | 文章、评论、统计、文件等后台操作 |
| PostgreSQL | Docker Compose `postgres` 服务内 `psql` | 数据查询、用户初始化、紧急数据核查 |
| Cloudflare R2 | Cloudflare Dashboard 或 S3 API 工具 | 对象存储 bucket、对象、CORS 和访问密钥检查 |
| 宿主机 Nginx | SSH 登录服务器后使用 `sudo nginx` | 反向代理、证书和域名入口配置 |

## 基础准备

进入生产部署目录并加载环境变量：

```bash
cd /opt/rendering-cms-platform/deploy
set -a
. /etc/rendering-cms/production.env
set +a
```

确认生产容器状态：

```bash
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml ps
```

## CMS 后台登录

访问地址：

```text
https://cms.rendering.me
```

登录用户来自 PostgreSQL 的 `users` 表。允许登录后台的角色只有：

```text
admin
editor
```

如果需要新增或重置后台用户，先进入 PostgreSQL，再执行本文档中的“新增或重置后台用户”步骤。

## 登录 PostgreSQL

交互式进入生产 PostgreSQL：

```bash
cd /opt/rendering-cms-platform/deploy
set -a
. /etc/rendering-cms/production.env
set +a

docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml exec postgres \
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"
```

无交互执行 SQL：

```bash
docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml exec -T postgres \
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "BEGIN TRANSACTION READ ONLY; SELECT now() LIMIT 1; COMMIT;"
```

退出 `psql`：

```sql
\q
```

## 新增或重置后台用户

进入 `psql` 后执行：

```sql
insert into users (
  email,
  name,
  password_hash,
  role
) values (
  'admin@rendering.me',
  'Admin',
  crypt('替换成强密码', gen_salt('bf', 12)),
  'admin'
)
on conflict (email) do update set
  name = excluded.name,
  password_hash = excluded.password_hash,
  role = excluded.role,
  updated_at = now();
```

检查用户：

```sql
BEGIN TRANSACTION READ ONLY;
select user_id, email, name, role, created_at, updated_at
from users
order by created_at desc
limit 20;
COMMIT;
```

安全要求：

- 不要把真实密码写入文档、脚本或 Git。
- 不要把包含真实密码的 SQL 通过聊天工具或工单系统明文传播。
- 生产用户创建完成后，应使用一次性强密码登录，并尽快切换到正式密码管理流程。
- 如果怀疑密码泄露，直接使用同一条 `insert ... on conflict ... do update` SQL 重置密码。

## Cloudflare R2 访问

R2 bucket、对象、CORS 和访问密钥在 Cloudflare Dashboard 中管理。生产环境后端使用 `/etc/rendering-cms/production.env` 中的 S3 兼容配置访问 R2：

```env
APP_ENV=production
DEV_BOOTSTRAP_ADMIN=false
S3_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com
S3_REGION=auto
S3_BUCKET=rendering-assets
S3_ACCESS_KEY_ID=<r2-access-key-id>
S3_SECRET_ACCESS_KEY=<r2-secret-access-key>
S3_USE_PATH_STYLE=false
S3_PUBLIC_BASE_URL=https://assets.rendering.me
S3_BLOG_IMAGE_PREFIX=blog
S3_ASSET_FILE_PREFIX=assets
```

不要把 `S3_SECRET_ACCESS_KEY` 打印到共享终端截图或日志里。需要在服务器上检查 bucket 时，优先使用只读或临时凭据。
生产环境必须保持 `APP_ENV=production` 和 `DEV_BOOTSTRAP_ADMIN=false`，不要启用 dev 默认管理员账号填充。

使用 AWS CLI 检查 R2 bucket 示例：

```bash
cd /opt/rendering-cms-platform/deploy
set -a
. /etc/rendering-cms/production.env
set +a

aws s3api list-objects-v2 \
  --endpoint-url "$S3_ENDPOINT" \
  --bucket "$S3_BUCKET" \
  --max-items 10
```

R2 bucket 必须配置 CORS，允许 `https://cms.rendering.me` 对预签名 URL 发起 `PUT` 和 `GET`，并允许 `Content-Type` 请求头。
`S3_PUBLIC_BASE_URL` 应配置为 R2 公开访问域名或自定义域名，用于文章正文图片 URL；不要把它和 `S3_ENDPOINT` 混用。
文章正文图片对象 key 使用 `S3_BLOG_IMAGE_PREFIX/YYYY/MM/<uuid>.<ext>`，普通资源文件对象 key 使用 `S3_ASSET_FILE_PREFIX/YYYY/MM/<uuid>.<ext>`。

推荐的 R2 bucket CORS JSON：

```json
[
  {
    "AllowedOrigins": [
      "https://cms.rendering.me"
    ],
    "AllowedMethods": [
      "PUT",
      "GET",
      "HEAD"
    ],
    "AllowedHeaders": [
      "Content-Type"
    ],
    "ExposeHeaders": [
      "ETag"
    ],
    "MaxAgeSeconds": 3600
  }
]
```

配置后重新上传文件。如果浏览器仍命中旧的预检缓存，等待缓存过期或打开无痕窗口重新验证。

## Nginx 访问与验证

Nginx 没有后台登录入口。所有操作都通过 SSH 登录服务器后执行。

查看当前加载配置：

```bash
sudo nginx -T | grep -n -E "server_name|listen|proxy_pass"
```

验证配置并重载：

```bash
sudo nginx -t
sudo systemctl reload nginx
```

查看 Nginx 服务状态：

```bash
sudo systemctl status nginx --no-pager
```

当前推荐域名和本机端口映射：

```text
rendering.me             -> 127.0.0.1:3000
www.rendering.me         -> 127.0.0.1:3000
cms.rendering.me         -> 127.0.0.1:3001
```

## 登录故障排查

CMS 登录失败：

1. 确认用户存在且角色为 `admin` 或 `editor`。
2. 确认 `JWT_SECRET` 已配置且后端容器已重启。
3. 检查是否触发登录失败锁定。
4. 查看后端日志：

   ```bash
   docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml logs --tail=200 backend
   ```

登录要求邮箱验证码但收不到邮件：

1. 确认 `SMTP_HOST`、`SMTP_USERNAME`、`SMTP_PASSWORD` 配置齐全；缺任一项时验证码功能整体关闭（登录不会要求验证码，也不会报错）。
2. 检查收件箱垃圾邮件文件夹；验证码邮件发件人为 `SMTP_FROM`（留空时为 `SMTP_USERNAME`）。
3. 确认 `SMTP_PASSWORD` 使用的是邮箱服务商的 SMTP 授权码，不是邮箱登录密码。
4. 确认 `SMTP_PORT` 与服务商匹配：465 为隐式 TLS，587 走 STARTTLS。
5. 触发了发送频率限制（60 秒重发间隔、每邮箱每小时 5 封）时接口返回 `429`，等待后重试。
6. 后端日志出现 `验证码邮件发送失败` 时，检查 SMTP 地址、授权码和网络连通性。

验证码输入后仍无法登录：

1. 验证码有效期 10 分钟，过期后需点击重新发送。
2. 单个验证码最多输错 5 次，超限作废后需重新发送。
3. `challengeToken` 有效期与验证码一致；超时后返回重新走密码登录。

PostgreSQL 无法进入：

1. 确认 `postgres` 容器运行。
2. 确认 `POSTGRES_USER` 和 `POSTGRES_DB` 与初始化配置一致。
3. 查看 PostgreSQL 日志：

   ```bash
   docker compose --env-file /etc/rendering-cms/production.env -f docker-compose.prod.yml logs --tail=100 postgres
   ```

R2 文件上传或下载失败：

1. 确认 `S3_ENDPOINT` 是 R2 S3 API 端点。
2. 确认 `S3_REGION=auto` 且 `S3_USE_PATH_STYLE=false`。
3. 确认 R2 API Token 具备目标 bucket 的对象读写权限。
4. 确认 R2 bucket CORS 允许生产前端来源、`PUT`、`GET` 和 `Content-Type`。
5. 查看后端日志中生成预签名 URL 或写入下载审计的错误。
