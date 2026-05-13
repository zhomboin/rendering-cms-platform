# 生产访问与登录 SOP

本文档记录生产环境中需要登录或进入的系统入口，包括 CMS 后台、PostgreSQL、MinIO Console 和宿主机 Nginx。命令默认在 Ubuntu 服务器执行，应用目录默认是 `/opt/rendering-cms-platform`。

## 入口总览

| 系统 | 访问方式 | 用途 |
| --- | --- | --- |
| CMS 后台 | `https://cms.rendering.me` | 文章、评论、统计、文件等后台操作 |
| PostgreSQL | Docker Compose `postgres` 服务内 `psql` | 数据查询、用户初始化、紧急数据核查 |
| MinIO Console | `https://minio-console.rendering.me` | 对象存储 bucket、对象和凭据检查 |
| MinIO API | `https://minio.rendering.me` | 预签名上传和下载 URL 访问 |
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

## MinIO Console 登录

访问地址：

```text
https://minio-console.rendering.me
```

登录凭据来自 `deploy/production.env`：

```env
MINIO_ROOT_USER=rendering
MINIO_ROOT_PASSWORD=replace-with-strong-minio-password
```

检查当前配置：

```bash
cd /opt/rendering-cms-platform/deploy
grep -E '^(MINIO_ROOT_USER|MINIO_BUCKET|MINIO_SERVER_URL|MINIO_BROWSER_REDIRECT_URL)=' production.env
```

不要把 `MINIO_ROOT_PASSWORD` 打印到共享终端截图或日志里。

## MinIO CLI 访问

使用临时 `mc` 容器连接生产 MinIO：

```bash
cd /opt/rendering-cms-platform/deploy
set -a
. ./production.env
set +a

docker run --rm --network rendering_cms \
  -e MINIO_ROOT_USER \
  -e MINIO_ROOT_PASSWORD \
  -e MINIO_BUCKET \
  minio/mc:latest \
  sh -c 'mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" && mc ls "local/$MINIO_BUCKET"'
```

如果需要检查 bucket 是否存在：

```bash
docker run --rm --network rendering_cms \
  -e MINIO_ROOT_USER \
  -e MINIO_ROOT_PASSWORD \
  -e MINIO_BUCKET \
  minio/mc:latest \
  sh -c 'mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" && mc stat "local/$MINIO_BUCKET"'
```

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
minio.rendering.me       -> 127.0.0.1:9000
minio-console.rendering.me -> 127.0.0.1:9001
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

MinIO Console 无法登录：

1. 确认 `minio` 容器运行。
2. 确认 `MINIO_ROOT_USER` 和 `MINIO_ROOT_PASSWORD` 未改错。
3. 确认 `minio-console.rendering.me` 的 Nginx 代理指向 `127.0.0.1:9001`。
4. 查看 MinIO 日志：

   ```bash
   docker compose --env-file production.env -f docker-compose.prod.yml logs --tail=100 minio
   ```
