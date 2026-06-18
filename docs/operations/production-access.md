# 生产访问与登录 SOP

本文档记录生产环境中需要登录或进入的系统入口，包括 CMS 后台、PostgreSQL、Cloudflare R2 和宿主机 Nginx。命令默认在 Ubuntu 服务器执行，应用目录默认是 `/opt/rendering-cms-platform`。

## 入口总览

| 系统 | 访问方式 | 用途 |
| --- | --- | --- |
| CMS 后台 | `https://cms.rendering.me` | 文章、评论、统计、文件等后台操作 |
| PostgreSQL | Docker Compose `postgres` 服务内 `psql` | 数据查询、用户初始化、紧急数据核查 |
| Cloudflare R2 | Cloudflare Dashboard 或 S3 API 工具 | 对象存储 bucket、对象、CORS 和访问密钥检查 |
| 宿主机 Nginx | SSH 登录服务器后使用 `sudo nginx` | 反向代理、证书和域名入口配置 |

## 基础准备

进入生产部署目录并加载环境变量：

```bash
cd /opt/rendering-cms-platform/deploy
set -a
. ./production.env
set +a
```

确认生产容器状态：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml ps
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
. ./production.env
set +a

docker compose --env-file production.env -f docker-compose.prod.yml exec postgres \
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"
```

无交互执行 SQL：

```bash
docker compose --env-file production.env -f docker-compose.prod.yml exec -T postgres \
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -c "select now();"
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
select user_id, email, name, role, created_at, updated_at
from users
order by created_at desc;
```

安全要求：

- 不要把真实密码写入文档、脚本或 Git。
- 不要把包含真实密码的 SQL 通过聊天工具或工单系统明文传播。
- 生产用户创建完成后，应使用一次性强密码登录，并尽快切换到正式密码管理流程。
- 如果怀疑密码泄露，直接使用同一条 `insert ... on conflict ... do update` SQL 重置密码。

## Cloudflare R2 访问

R2 bucket、对象、CORS 和访问密钥在 Cloudflare Dashboard 中管理。生产环境后端使用 `deploy/production.env` 中的 S3 兼容配置访问 R2：

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
. ./production.env
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
   docker compose --env-file production.env -f docker-compose.prod.yml logs --tail=200 backend
   ```

PostgreSQL 无法进入：

1. 确认 `postgres` 容器运行。
2. 确认 `POSTGRES_USER` 和 `POSTGRES_DB` 与初始化配置一致。
3. 查看 PostgreSQL 日志：

   ```bash
   docker compose --env-file production.env -f docker-compose.prod.yml logs --tail=100 postgres
   ```

R2 文件上传或下载失败：

1. 确认 `S3_ENDPOINT` 是 R2 S3 API 端点。
2. 确认 `S3_REGION=auto` 且 `S3_USE_PATH_STYLE=false`。
3. 确认 R2 API Token 具备目标 bucket 的对象读写权限。
4. 确认 R2 bucket CORS 允许生产前端来源、`PUT`、`GET` 和 `Content-Type`。
5. 查看后端日志中生成预签名 URL 或写入下载审计的错误。
