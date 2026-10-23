# 相册功能（CMS 侧）实施计划

**目标：** 在 CMS 平台新增相册能力：后台管理相册与照片（上传图片到对象存储、维护相册元数据与发布状态），并向 Rendering 博客 `/photos` 页面提供公开查询接口。图片管理全部在 CMS 后台完成，Rendering 博客只通过公开接口读取已发布相册，不提供任何编辑能力。

**已确认决策：**

- 接口契约以《CMS 照片相册公开接口要求》（Rendering 博客侧需求文档）为准，落盘为本仓库 `docs/apis/photos.md`。
- 公开接口三个：`GET /api/v1/photos`（相册分页列表）、`GET /api/v1/photos/{slug}`（相册元数据）、`GET /api/v1/photos/{slug}/photos`（相册照片分页清单）；另实现可选的相册浏览上报 `POST /api/v1/photos/{slug}/views`（与文章上报对称）。
- 错误响应结构（`{"error": "..."}`）、公开读取缓存约定（ETag/304、`Cache-Control: public, max-age=60, stale-while-revalidate=300`、404 `max-age=15`）、公开读限流沿用文章公开接口既有约定。
- 分页规则（本仓库首个分页实现）：`page`/`limit` 非整数或小于 1 返回 `400`；`limit` 超过 100 按上限 100 处理；页码超出范围返回空 `data` 与正确分页元数据，不返回 `404`；相册列表默认 `limit=12`，照片清单默认 `limit=24`。
- 相册状态机：`draft` ⇄ `published`；仅 `published` 相册对外可见；发布要求相册至少有一张照片；`published_at` 首次发布时写入并保持稳定，相册按 `published_at` 倒序排列；下架后公开接口立即返回 `404`（受 60 秒响应缓存影响，最长约 2 分钟生效）。
- 相册 `slug` 由管理员创建时指定（`^[a-z0-9]+(-[a-z0-9]+)*$`，全库唯一），发布后不可修改，保证 Rendering 路由 `/photos/{slug}` 稳定。
- 封面：`cover_photo_id` 指向相册内某张照片，未指定时回退第一张照片（按相册内顺序）；封面 URL 与尺寸由该照片解析。
- 照片上传链路采用**后端代理上传**：前端将图片以 multipart 直传后端，后端完成魔数校验（仅 png/jpeg/webp）、服务端提取像素尺寸（权威值）、剥离 EXIF/XMP 等元数据段（含 GPS 等隐私信息）、提取 EXIF 拍摄时间（`takenAt` 默认值），再写入对象存储并带上 `Cache-Control: public, max-age=31536000, immutable` 长缓存头。现有 assets 预签名直传链路保持不变。
- 照片对象存储在独立前缀 `photos/`（配置 `S3_PHOTO_PREFIX`，默认 `photos`），对象 key 含 UUID、上传后不可变（不改名、不覆盖）；公开 URL 沿用 `S3_PUBLIC_BASE_URL` 机制写入 `assets.public_url`，未配置公共基地址时照片上传直接拒绝。
- 照片元数据（alt、尺寸、拍摄时间、顺序）存于 `album_photos` 表；文件本体元数据沿用 `assets` 表（新增 `width`/`height` 可空列）。相册照片总数 `photoCount` 不落库，由查询按 `album_photos` 实时计算，保证与照片表永远一致。
- 相册浏览上报沿用既有统计架构：`analytics_events` 增加 `album_id` 与 `album_view` 事件类型、每日 IP 哈希去重，新增 `album_view_daily` 日聚合表；后台看板的相册浏览展示不在本期。

**架构：** PostgreSQL migration `000013_photo_albums`（`photo_albums`、`album_photos`、`photo_album_status` 枚举、`assets` 宽高列）与 `000014_album_view_analytics`（相册浏览统计）；后端新增 `internal/imaging`（纯 Go 图片魔数/尺寸/EXIF 处理）与 `internal/albums`（公开 + 后台接口），`internal/storage` 扩展 `PutObject`/`DeleteObject`，`internal/analytics` 增加相册上报；前端后台新增相册列表页与编辑页。

**技术栈：** Go、Chi、pgx/sqlc、golang-migrate、PostgreSQL、S3 兼容对象存储（MinIO/R2）、React + TypeScript + Vite + Ant Design。

## 执行状态（2026-10-23）

- [x] migration `000013_photo_albums`：up/down 新建 `photo_albums`、`album_photos` 表与 `photo_album_status` 枚举，`assets` 增加 `width`/`height` 可空列。
- [x] migration `000014_album_view_analytics`：up/down 为 `analytics_events` 增加 `album_id` 并扩展每日去重唯一索引，新建 `album_view_daily` 日聚合表。
- [x] sqlc 查询 `sql/albums.sql`：公开相册分页列表/详情/照片分页清单 + 后台相册与照片全套管理查询。
- [x] 后端 `internal/imaging`：魔数嗅探、像素尺寸提取（png/jpeg/webp）、EXIF/XMP 等元数据剥离（JPEG 丢弃 APP1/APP3–APP13/APP15/COM 元数据段、PNG 丢弃 eXIf 与文本 chunk、WebP 删除 EXIF/XMP chunk 并清 VP8X 标志位）、EXIF 拍摄时间提取，附 fixture 单测。
- [x] 后端 `internal/storage`：`PutObject`（带 CacheControl/ContentType）与 `DeleteObject`。
- [x] 后端 `internal/albums`：公开三个查询接口（分页元数据、越界空 data、未发布 404、字段白名单、ETag/304）与后台管理接口（创建/更新/发布/下架/删除、照片上传/编辑/排序/删除/设封面），公开读挂入现有限流组。
- [x] 后端 `internal/analytics`：`POST /api/v1/photos/{slug}/views` 相册浏览上报（每日 IP 哈希去重、204）。
- [x] 后端测试：分页解析与上限、404 语义、字段白名单、slug 发布后不可改、发布需有照片、401 未登录、上传校验链。
- [x] 前端后台：`api/albums.ts`、相册列表页（新建/发布/下架/删除）、相册编辑页（元数据表单 + 照片墙：上传、alt/caption 编辑、排序、设为封面、删除）、菜单与路由接入。
- [x] 文档同步：`docs/apis/photos.md`、`docs/apis/README.md`、`docs/sql/schema.md`、`backend/README.md`、`frontend/README.md`、`scripts/env/dev.env.example`、`deploy/production.env.example`、根 README、`AGENTS.md` 功能范围、技术架构建议文档。
- [x] 验证：`go test ./...`、`go vet ./...`、`npm run build`；migration 本地 up/down/up 回环验证；本地 PostgreSQL + MinIO 完整冒烟：创建相册 → 上传带 EXIF 的 JPEG（服务端剥离 EXIF、提取 120×80 尺寸、对象写入长缓存头）→ 发布 → 三个公开接口（分页元数据、越界空 data、未发布 404、图片 URL 直访 200 且带 immutable 缓存头）→ 浏览上报与每日去重 → 下架 404 → 发布后改 slug 409 → 删除链路（对象清理、资源软删、级联清空）。
- [x] PR 代码检视、合入 main、更新 `docs/progress.md`。

## 行为说明

- 公开接口只返回 `published` 相册；相册内照片按后台定义的相册内顺序（`position` 升序）返回。
- `photoCount` 为相册照片总数，由查询按 `album_photos` 实时计算，与照片表永远一致。
- 图片 URL 一律为 `S3_PUBLIC_BASE_URL` 下的绝对 URL，公开可读、内容不可变，对象响应带长缓存头。
- 每张照片必须包含 `alt`（上传时必填）与 `width`/`height`（服务端提取，与实际像素一致）；`caption`、`takenAt` 可选。
- 删除相册会级联删除照片记录，关联 `assets` 置为软删（`status=deleted`），对象存储文件尽力删除。

## 非目标

- Rendering 博客侧任何页面改动（由博客仓库实现）。
- CORS 白名单（一期博客为服务端读取，不需要；博客前端直连时再开放）。
- 照片裁剪、缩略图生成、后台看板相册浏览展示、相册标签筛选、访客侧交互。
- 不改动现有 assets 预签名直传链路与文章模块行为。

## 边界与已知影响

- WebP EXIF 剥离通过 RIFF chunk 手术实现；若个别文件兼容性问题，按尽力剥离处理并保持尺寸解析正确。
- 后端代理上传占用后端带宽与内存（单文件 ≤ 20MB，在现有 25MB 请求体上限内），个人博客量级可忽略。
- 公开响应缓存 60 秒 + 博客 ISR 60 秒：相册下架后公开侧最长约 2 分钟内消失（404 响应 `max-age=15`）。
- 照片上传要求已配置 `S3_PUBLIC_BASE_URL`；未配置时上传接口返回明确错误，不产生半公开状态。
