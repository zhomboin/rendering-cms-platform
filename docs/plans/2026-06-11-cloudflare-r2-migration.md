# Cloudflare R2 对象存储迁移计划

> 状态：仓库侧已执行，Cloudflare 控制台配置、存量对象迁移和生产实际切换待执行
> 创建日期：2026-06-11
> 仓库侧执行日期：2026-06-15
> 关联系统：对象存储、文件上传下载、资产管理

## 一、背景

当前项目使用 S3 兼容对象存储抽象（`aws-sdk-go-v2/service/s3`），开发环境和生产环境均部署本地 MinIO 服务。根据项目架构基线（`docs/cms-platform-technical-recommendation.zh-CN.md`）、部署文档和运维文档，已将 MinIO 标记为"当前临时方案"，明确后续可迁移到 Cloudflare R2、AWS S3 或同类托管对象存储。

本次计划将生产对象存储从**自建 MinIO** 切换到 **Cloudflare R2**，开发环境保留 MinIO 作为本地回退。

## 二、影响范围分析

### 2.1 代码层（仅 1 个文件需修改）

| 文件 | 变更类型 | 说明 |
|---|---|---|
| `backend/internal/storage/s3.go:38` | **必须修改** | 将 `UsePathStyle: true`（MinIO 路径风格）改为 `false` 或改为可由配置控制 |

**说明**：Cloudflare R2 使用虚拟主机风格（virtual-hosted-style）寻址，即 bucket 在域名中（例：`https://<bucket>.<accountid>.r2.cloudflarestorage.com`），与 MinIO 的路径风格（`http://host:port/bucket/key`）不同。当前代码硬编码了 MinIO 的路径风格。

#### 推荐实现方案

将 `UsePathStyle` 改为由 `S3Config` 配置控制，兼顾 MinIO 开发环境和 R2 生产环境：

```go
// backend/internal/config/config.go - S3Config 新增字段
type S3Config struct {
    Endpoint        string
    Region          string
    Bucket          string
    AccessKeyID     string
    SecretAccessKey string
    UsePathStyle    bool   // 新增：true=路径风格(MinIO), false=虚拟主机风格(R2)
}
```

```go
// backend/internal/storage/s3.go - 使用配置值
UsePathStyle: cfg.UsePathStyle,
```

环境变量新增 `S3_USE_PATH_STYLE`（默认 `false`），开发环境 `.env` 显式设为 `true`。

### 2.2 配置层（5 个环境变量文件）

| 文件 | 变更 |
|---|---|
| `.env`（开发） | 新增 `S3_USE_PATH_STYLE=true`，保留 MinIO 的 `S3_ENDPOINT=http://127.0.0.1:9000` |
| `scripts/env/dev.env.example` | 同上 |
| `scripts/env/.env` | 同上 |
| `deploy/production.env.example` | 修改 `S3_ENDPOINT` 为 R2 端点、新增 `S3_USE_PATH_STYLE=false`、更新 `S3_ACCESS_KEY_ID`/`S3_SECRET_ACCESS_KEY` 为 R2 API Token、移除 MinIO 专属环境变量 |

#### 生产环境变量变更（production.env.example）

```diff
- # Local MinIO object storage.
- MINIO_ROOT_USER=rendering
- MINIO_ROOT_PASSWORD=replace-with-strong-minio-password
- MINIO_BUCKET=rendering-assets
- MINIO_SERVER_URL=https://minio.rendering.me
- MINIO_BROWSER_REDIRECT_URL=https://minio-console.rendering.me
- MINIO_API_BIND=127.0.0.1:9000
- MINIO_CONSOLE_BIND=127.0.0.1:9001

+ # Cloudflare R2 对象存储
+ S3_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com
+ S3_REGION=auto
+ S3_BUCKET=rendering-assets
+ S3_ACCESS_KEY_ID=<r2-access-key-id>
+ S3_SECRET_ACCESS_KEY=<r2-secret-access-key>
+ S3_USE_PATH_STYLE=false

- S3_ENDPOINT=https://minio.rendering.me
- S3_REGION=us-east-1
- S3_BUCKET=rendering-assets
- S3_ACCESS_KEY_ID=rendering
- S3_SECRET_ACCESS_KEY=replace-with-strong-minio-password
```

### 2.3 基础设施层（3 个文件）

| 文件 | 变更类型 | 说明 |
|---|---|---|
| `deploy/docker-compose.prod.yml` | **修改** | 移除 `minio` 服务、`minio-init` 服务、`minio_data` volume；移除 `backend` 对 `minio`/`minio-init` 的 `depends_on` |
| `deploy/nginx/rendering.me.conf` | **修改** | 移除 `minio.rendering.me` 和 `minio-console.rendering.me` 的 server 块及 upstream |
| `scripts/env/docker-compose.dev.yml` | **不修改** | 开发环境保留 MinIO |
| 开发启动脚本（`start-prerequisites.sh` 等） | **不修改** | 开发环境保留 MinIO |

#### docker-compose.prod.yml 变更摘要

```diff
-   minio:
-     image: minio/minio:latest
-     ...
-   minio-init:
-     image: minio/mc:latest
-     ...

  backend:
    depends_on:
      postgres:
        condition: service_healthy
-       minio:
-         condition: service_started
-       minio-init:
-         condition: service_completed_successfully

volumes:
-   minio_data:
-     name: rendering_cms_minio_data
```

#### Nginx 变更摘要（rendering.me.conf）

```diff
- upstream rendering_minio_api {
-   server 127.0.0.1:9000;
-   keepalive 32;
- }
- upstream rendering_minio_console {
-   server 127.0.0.1:9001;
-   keepalive 32;
- }

  server {
    listen 80;
-   server_name rendering.me www.rendering.me cms.rendering.me minio.rendering.me minio-console.rendering.me;
+   server_name rendering.me www.rendering.me cms.rendering.me;
    return 301 https://$host$request_uri;
  }

- server {
-   listen 443 ssl http2;
-   server_name minio.rendering.me;
-   ...
- }
- server {
-   listen 443 ssl http2;
-   server_name minio-console.rendering.me;
-   ...
- }
```

### 2.4 数据库层（无变更）

- `assets` 表结构不变：`storage_key`、`public_url` 等字段在 MinIO 和 R2 之间的语义完全一致
- `download_events` 表不变
- 无 migration 需求

### 2.5 前端层（无必要变更）

| 文件 | 现有行为 | R2 是否受影响 |
|---|---|---|
| `frontend/src/api/assets.ts` | 获取预签名 URL → 直接 PUT 上传 | ✅ 不受影响（签名 URL 对客户端透明） |
| `frontend/src/pages/assets/AssetsPage.tsx` | 调用 API 上传/下载 | ✅ 不受影响 |
| `frontend/src/pages/articles/ArticleEditorPage.tsx:354` | 检查 `asset.publicUrl` 非空 | ⚠️ 该字段当前始终为 null（与 MinIO/R2 无关），是预存问题 |

#### 关于 `publicUrl` 字段

`ArticleEditorPage.tsx` 在封面图片上传后检查 `asset.publicUrl` 不为 null，但当前代码中 `CreateAsset` 从未填充 `public_url`。这意味着在 MinIO 下封面图片上传功能本身也存在问题。

已在计划中列为"附带修复"——为 R2 bucket 启用自定义域名（如 `https://assets.rendering.me`），并在创建资源时填充 `public_url`。

### 2.6 文档层（6 份文档需更新）

| 文档 | 变更说明 |
|---|---|
| `docs/operations/deployment.md` | 将 MinIO 替换为 R2；更新环境变量配置说明；移除 MinIO domain/port 相关描述 |
| `docs/operations/backup.md` | 替换 MinIO bucket 备份章节为 R2 备份策略；移除 `mc mirror` 命令 |
| `docs/operations/restore.md` | 替换 MinIO 恢复章节为 R2 恢复策略 |
| `docs/operations/runbook.md` | 移除 MinIO 巡检命令（`minio/health/ready`）；更新文件上传下载故障排查步骤 |
| `docs/operations/production-access.md` | 移除 MinIO Console/CLI 访问章节；更新入口总览表 |
| `docs/operations/development-environment.md` | 标注 MinIO 仅用于本地开发 |

## 三、执行步骤

### Phase 1：代码变更（低风险）

- [x] **1.1** `backend/internal/config/config.go`：`S3Config` 新增 `UsePathStyle bool` 字段，从 `S3_USE_PATH_STYLE` 环境变量读取（默认 `false`）
- [x] **1.2** `backend/internal/storage/s3.go:38`：将 `UsePathStyle: true` 改为 `UsePathStyle: cfg.UsePathStyle`
- [x] **1.3** 开发环境 `.env` / `scripts/env/dev.env.example` / `scripts/env/.env`：新增 `S3_USE_PATH_STYLE=true`
- [x] **1.4** 运行 `go test ./...` 验证无回归

### Phase 2：Cloudflare R2 准备（一次性）

- [ ] **2.1** 在 Cloudflare Dashboard 创建 R2 bucket（名称：`rendering-assets`）
- [ ] **2.2** 创建 R2 API Token（权限：Object Read & Write）
- [ ] **2.3** （可选）为 bucket 绑定自定义域名（如 `assets.rendering.me`），用于 `public_url`
- [ ] **2.4** 记录 R2 端点 URL、Access Key ID、Secret Access Key

### Phase 3：存量数据迁移

- [ ] **3.1** 导出当前 MinIO bucket 文件清单及 storage_key
- [ ] **3.2** 使用 `rclone` 或 AWS CLI（配置 R2 endpoint）将 MinIO 中对象同步到 R2
- [ ] **3.3** 验证数据完整性：抽样对比 MinIO 与 R2 中的对象数量和文件大小
- [ ] **3.4** 在 R2 中验证 `assets` 表中所有 `storage_key` 对应的对象均存在

### Phase 4：生产环境变更

- [ ] **4.1** 备份当前 PostgreSQL 和 MinIO 数据（参考 `docs/operations/backup.md`）
- [ ] **4.2** 修改 `deploy/production.env`，替换 S3 环境变量为 R2 配置
- [x] **4.3** 修改 `deploy/docker-compose.prod.yml`，移除 `minio`、`minio-init` 服务和 `minio_data` volume，移除 `backend` 对 MinIO 的 `depends_on`
- [x] **4.4** 修改 `deploy/nginx/rendering.me.conf`，移除 `minio.rendering.me` 和 `minio-console.rendering.me` 的 upstream 及 server 块
- [ ] **4.5** 重新构建并部署后端镜像
- [ ] **4.6** 执行 smoke test（见下方验收清单）

### Phase 5：文档更新

- [x] **5.1** `docs/operations/deployment.md`——更新对象存储描述、环境变量和部署拓扑
- [x] **5.2** `docs/operations/backup.md`——更新对象存储备份章节
- [x] **5.3** `docs/operations/restore.md`——更新对象存储恢复章节
- [x] **5.4** `docs/operations/runbook.md`——更新日常巡检和故障排查步骤
- [x] **5.5** `docs/operations/production-access.md`——更新入口总览，移除 MinIO 相关章节
- [x] **5.6** `docs/operations/development-environment.md`——标注 MinIO 仅用于本地开发

### Phase 6：附带优化（可选）

- [ ] **6.1** 实现 `public_url` 填充：在 `CreateAsset` 时根据 bucket 自定义域名生成公开 URL
- [ ] **6.2** 修复 `ArticleEditorPage.tsx` 封面图片上传后 `publicUrl` 为空的问题

## 四、验收清单

- [ ] 开发环境（MinIO）：上传文件成功，下载正常，`S3_USE_PATH_STYLE=true` 生效
- [ ] 生产环境（R2）：上传文件成功，文件进入 R2 bucket
- [ ] 生产环境：下载链接生成正常，预签名 URL 指向 R2 端点（而非 MinIO 域名）
- [ ] 生产环境：`download_events` 审计记录正常写入
- [ ] 生产环境：R2 中 `assets/<uuid>/<filename>` 路径结构与数据库 `storage_key` 一致
- [ ] 生产环境：存量 MinIO 对象已完整迁移到 R2 且可正常下载
- [ ] Docker Compose 中不再包含 `minio`、`minio-init` 服务
- [ ] Nginx 配置中不再包含 MinIO 域名的代理块
- [ ] 后端日志无对象存储连接错误

## 五、回滚方案

如果 R2 迁移后发现严重问题：

1. **恢复 MinIO 服务**：还原 `docker-compose.prod.yml` 中的 `minio`/`minio-init` 服务定义和 `depends_on`
2. **恢复 Nginx 配置**：还原 `minio.rendering.me` 和 `minio-console.rendering.me` 的代理块
3. **恢复环境变量**：将 `production.env` 中的 S3 配置改回 MinIO 端点
4. **重新构建部署**：`docker compose build && docker compose up -d`
5. **数据验证**：确认 MinIO volume 中存量对象完整，`storage_key` 对应对象可访问

R2 上已上传的新对象不会丢失——待问题解决后可重新切换。

## 六、风险与注意事项

| 风险 | 缓解措施 |
|---|---|
| R2 预签名 URL 格式与 MinIO 不同导致前端无法上传 | Phase 4 前先在开发环境用 R2 端点验证完整上传-下载流程 |
| `UsePathStyle` 差异导致 bucket 寻址失败 | 通过 `S3_USE_PATH_STYLE` 环境变量分别控制开发/生产 |
| 存量 MinIO 对象迁移丢失 | `rclone sync` 后执行 `rclone check` 校验；抽样验证 |
| 迁移期间文件上传中断 | 选择低流量时段执行；在维护窗口内完成 |
| R2 跨区域延迟高于本地 MinIO | R2 使用 Cloudflare 全球 CDN 加速，实测延迟在可接受范围内 |
| R2 API Token 权限过大 | 创建专用 Token，仅授予 `rendering-assets` bucket 的 Object Read & Write |

## 七、预估工时

| 阶段 | 预估时间 |
|---|---|
| Phase 1：代码变更 | 30 分钟 |
| Phase 2：R2 准备 | 15 分钟（Cloudflare 控制台操作） |
| Phase 3：存量数据迁移 | 取决于数据量（通常 < 30 分钟） |
| Phase 4：生产环境变更 | 1 小时（含部署、重启、smoke test） |
| Phase 5：文档更新 | 1 小时 |
| Phase 6：附带优化 | 1 小时 |
| **合计** | **约 4 小时** |
