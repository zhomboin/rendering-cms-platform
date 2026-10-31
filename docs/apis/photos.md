# 相册 API

本文档记录相册功能的公开查询接口（供 Rendering 博客 `/photos` 页面服务端读取）与后台管理接口。所有接口前缀为 `/api/v1`。

公开接口的可见性、排序、缓存与错误约定：

- 只返回 `published` 状态的相册；下架后源站查询不再返回，相册的公开可见性仍受客户端、代理及博客页面缓存和再验证影响，不承诺固定消失时限。404 响应携带 `Cache-Control: public, max-age=15`。
- 相册按 `publishedAt` 倒序（`nulls last`，`createdAt` 决胜）；相册内照片按后台定义的相册内顺序（`position` 升序）返回。
- 公开 GET 携带 `ETag`、`Cache-Control: public, max-age=60, stale-while-revalidate=300`，支持 `If-None-Match` 协商返回 `304`；与文章公开接口一致，并共用同一公开读限流组。
- 相册不存在或未发布返回 `404`；服务异常返回 `5xx`；错误响应结构为 `{"error": "错误说明"}`。

## 后台页面约定

后台新建入口为 /admin/albums/new，不带相册 ID，只显示创建表单，不查询详情或照片，不显示发布和上传操作；创建失败保留表单并允许重试。创建成功后使用响应中的 albumId 进入 /admin/albums/{id}/edit。

基本信息页 /admin/albums/{id}/edit 维护标题、slug、导语、标签和发布状态，不查询照片列表。图片管理页 /admin/albums/{id}/photos 独立负责上传、排序、封面、照片说明编辑和删除；相册列表提供两个入口，两页可互相跳转并直接刷新。

图片卡片显示接口返回的完整 url，并提供复制按钮；复制失败时提示手动复制，无 url 时说明公开访问配置需要检查。图片 URL 为公开资源地址，草稿相册状态不改变对象存储图片的可读性。

## 博客消费要求

Rendering 相册列表和照片清单必须按页读取，默认分别为 12 本和 24 张；首屏仅请求目标页，翻页或接近加载入口后再读取后续页，不在页面渲染时循环获取所有分页。图片文件另行按展示位置懒加载，不能仅用图片懒加载属性代替数据分页。列表已有封面，不需要逐本获取照片来生成卡片。空封面使用占位，不过滤相册；接口故障与空列表分别处理。

博客现有接入位置、完整字段、封面处理和验收见 [相册、照片与文章封面对接](../operations/rendering-blog-media-integration.md)。

## 分页约定

相册列表与照片清单共用以下分页规则（本平台首个分页式公开接口）：

| 参数 | 类型 | 默认 | 说明 |
| --- | --- | --- | --- |
| `page` | int | 按接口默认 | 页码，从 1 开始 |
| `limit` | int | 按接口默认 | 每页条数；超过 100 时按 100 处理 |

- `page` / `limit` 非整数或小于 1 返回 `400`。
- 页码超出范围返回空 `data` 与正确的分页元数据，不返回 `404`。
- 分页元数据：`page`、`limit`、`total`、`totalPages`（`totalPages = ceil(total / limit)`）。

## 相册数据模型（公开字段）

```json
{
  "slug": "city-nights",
  "title": "城市夜色",
  "titleEn": "City Nights",
  "description": "一组夜景。",
  "descriptionEn": "A set of night shots.",
  "coverUrl": "https://files.example.com/photos/2026/10/xxxx.jpg",
  "coverWidth": 2400,
  "coverHeight": 1600,
  "publishedAt": "2026-10-23T00:00:00Z",
  "tags": ["photography", "city"],
  "photoCount": 87
}
```

字段说明：

- `slug`：相册 URL 标识，对应博客路由 `/photos/{slug}`；格式 `^[a-z0-9]+(-[a-z0-9]+)*$`，全站唯一，发布后不可修改。
- `title` / `description`：中文标题与导语，后台必填。
- `titleEn` / `descriptionEn`：英文镜像页文案，可选；未填时返回 `null`，由博客回退中文字段。
- `coverUrl` / `coverWidth` / `coverHeight`：封面图绝对 URL 与像素尺寸。封面为后台指定的相册内照片，未指定时取第一张；相册无照片时三个字段均为 `null`。尺寸为服务端提取的权威值。
- `publishedAt`：ISO 8601 时间；首次发布写入并保持稳定，下架再发布沿用原时间。
- `tags`：仅展示用途，不并入博客标签筛选体系。
- `photoCount`：相册照片总数，由服务端按 `album_photos` 实时计算。

## 公开接口 1：相册分页列表

```http
GET /api/v1/photos?page=1&limit=12
```

说明：

- `limit` 默认 `12`。
- 返回已发布相册的分页列表，每项字段同上方数据模型。

响应示例：

```json
{
  "data": [ { "slug": "city-nights", "...": "..." } ],
  "page": 1,
  "limit": 12,
  "total": 57,
  "totalPages": 5
}
```

## 公开接口 2：相册元数据

```http
GET /api/v1/photos/{slug}
```

说明：

- 返回单个相册的完整元数据，字段同列表项。
- 本接口不包含照片数组；照片清单由接口 3 分页获取。
- 相册不存在或未发布返回 `404`。

## 公开接口 3：相册照片分页清单

```http
GET /api/v1/photos/{slug}/photos?page=1&limit=24
```

说明：

- `limit` 默认 `24`；相册不存在或未发布返回 `404`。
- 每张照片字段：

```json
{
  "url": "https://files.example.com/photos/2026/10/01.jpg",
  "alt": "夜色中的天桥",
  "caption": "拍于江边",
  "width": 2400,
  "height": 1600,
  "takenAt": "2026-10-01T18:42:00Z"
}
```

- `url`：绝对 URL、公开可读、上传后内容不可变；对象响应携带 `Cache-Control: public, max-age=31536000, immutable` 长缓存头。
- `alt`：必填，无障碍描述；`caption` 可选，灯箱展示。
- `width` / `height`：服务端从图片字节提取的权威像素值。
- `takenAt`：拍摄时间，可选；上传时服务端从 EXIF `DateTimeOriginal` 提取（按 UTC 解析），表单显式提供的值优先；未提供且无 EXIF 时为 `null`。

## 公开接口 4：相册浏览上报

```http
POST /api/v1/photos/{slug}/views
```

说明：

- 与 `POST /api/v1/articles/{slug}/views` 对称：仅对已发布相册计数，成功与每日去重命中均返回 `204`。
- 每日每 IP 哈希每相册只计一次（`analytics_events` 唯一索引去重）；计数同时写入 `album_view_daily` 与 `site_view_daily` 日聚合。
- 相册不存在或未发布返回 `404`。

## 图片内容与隐私规则

- 照片上传采用后端代理链路：后端完成魔数校验（只接受 PNG / JPEG / WebP，不信任客户端 Content-Type）、像素尺寸提取、元数据剥离后再写入对象存储；文件本体与元数据不经过浏览器直传。
- 存储前剥离 EXIF 与 XMP 等元数据段（含 GPS 等隐私信息）：JPEG 丢弃 APP1（EXIF/XMP）与 APP3–APP13、APP15 厂商元数据段及 COM 注释段（保留 APP0 JFIF、APP2 ICC 色彩描述、APP14 Adobe 色彩变换），PNG 丢弃 `eXIf` 与 tEXt/zTXt/iTXt 文本 chunk（XMP 位于 iTXt 的 `XML:com.adobe.xmp`），WebP 删除 EXIF/XMP chunk 并清除 VP8X 对应标志位；不做像素重编码，画质无损。
- 对象 key 形如 `photos/{年}/{月}/{uuid}.{权威扩展名}`，写入时携带长缓存头；key 含随机 UUID，内容永不覆盖。
- 照片对象前缀由 `S3_PHOTO_PREFIX` 配置（默认 `photos`），公开 URL 由 `S3_PUBLIC_BASE_URL` 拼接并写入 `assets.public_url`；未配置公共基地址时上传接口直接拒绝（`503`）。
- 单张照片上限 `20MB`。

## 后台接口

后台接口均需 `Authorization: Bearer <jwt-token>`（admin / editor），路径前缀 `/api/v1/admin`。后台相册对象在公开字段之外还包含：`albumId`、`status`（draft / published）、`coverPhotoId`、`createdBy`、`createdAt`、`updatedAt`。

### 相册列表

```http
GET /api/v1/admin/albums
```

- 返回全部状态的相册，按 `updatedAt` 倒序；含 `photoCount` 与封面信息，前端本地分页。

### 创建相册

```http
POST /api/v1/admin/albums
```

请求体：`slug`、`title`、`description` 必填；`titleEn`、`descriptionEn`、`tags` 可选。`slug` 冲突返回 `400`。

### 相册详情 / 更新

```http
GET  /api/v1/admin/albums/{id}
PATCH /api/v1/admin/albums/{id}
```

- 更新同创建的字段校验；已发布相册修改 `slug` 返回 `409`。
- 相册不存在返回 `404`。

### 发布 / 下架 / 删除

```http
POST   /api/v1/admin/albums/{id}/publish
POST   /api/v1/admin/albums/{id}/unpublish
DELETE /api/v1/admin/albums/{id}
```

- 发布要求相册至少有一张照片，否则 `409`；重复发布 / 重复下架返回 `409`。
- 删除相册会级联删除照片记录、软删关联 `assets`（`status=deleted`）、尽力删除对象存储文件；返回 `204`。

### 照片上传（后端代理）

前端提交 FormData 时须清除共用客户端的 JSON Content-Type 默认值，由浏览器生成 multipart/form-data 与 boundary；不得将文件序列化为 JSON，也不得手工拼接 boundary。上传失败显示后端错误说明，可重新选择或拖入文件重试。

```http
POST /api/v1/admin/albums/{id}/photos
Content-Type: multipart/form-data
```

表单字段：`file`（必填）、`alt`（必填）、`caption`（可选）、`takenAt`（可选，RFC 3339）。

说明：

- 后端处理链：魔数校验 → 尺寸提取 → `takenAt` 解析（表单优先，EXIF 兜底）→ 元数据剥离 → 写对象存储（长缓存头）→ 写 `assets`（含宽高）与 `album_photos`（position 追加到相册末尾）。
- 非图片类型、超过 20MB、alt 缺失等返回 `400`；未配置 `S3_PUBLIC_BASE_URL` 返回 `503`。
- 成功返回 `201` 与照片对象（`photoId`、`url`、`alt`、`caption`、`width`、`height`、`takenAt`、`position`、`filename`、`contentType`、`byteSize`）。

### 照片管理

```http
GET    /api/v1/admin/albums/{id}/photos
PATCH  /api/v1/admin/albums/{id}/photos/{photoId}
DELETE /api/v1/admin/albums/{id}/photos/{photoId}
PUT    /api/v1/admin/albums/{id}/photos/order
POST   /api/v1/admin/albums/{id}/cover
```

- `PATCH` 编辑 `alt`（必填）、`caption`、`takenAt`。
- `DELETE` 删除照片：清除指向它的封面引用（回退第一张）、软删关联资源、尽力删除对象；返回 `204`。
- `PUT .../order` 请求体 `{"photoIds": ["...", "..."]}`，必须包含相册内全部照片各一次，否则 `400`；顺序即新 `position`（1 起）。
- `POST .../cover` 请求体 `{"photoId": "uuid 或 null"}`；照片不属于该相册返回 `404`，`null` 恢复默认封面（第一张照片）。

## CORS

Rendering 博客一期仅服务端读取上述公开接口，不依赖 CORS；后续博客前端直连（相册页"加载更多"等增强）时，再将博客域名加入 CORS 白名单（与统计上报要求一致）。
