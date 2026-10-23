-- 相册公开读取：仅返回 published 状态，按 published_at 倒序。
-- 封面解析：优先 cover_photo_id 指定的照片，未指定时回退相册内第一张照片。
-- photo_count 由关联子查询实时计算，保证与 album_photos 永远一致。

-- name: CountPublishedPhotoAlbums :one
select count(*)::int as total
from photo_albums
where status = 'published';

-- name: ListPublishedPhotoAlbums :many
select
  a.album_id,
  a.slug,
  a.title,
  a.title_en,
  a.description,
  a.description_en,
  a.tags,
  a.published_at,
  a.created_at,
  a.updated_at,
  (select count(*) from album_photos p where p.album_id = a.album_id)::int as photo_count,
  coalesce(cover.public_url, first_photo.public_url) as cover_url,
  coalesce(cover.width, first_photo.width, 0)::int as cover_width,
  coalesce(cover.height, first_photo.height, 0)::int as cover_height
from photo_albums a
left join album_photos cover_photo on cover_photo.photo_id = a.cover_photo_id
left join assets cover on cover.asset_id = cover_photo.asset_id
left join lateral (
  select ast.public_url, ast.width, ast.height
  from album_photos ap
  join assets ast on ast.asset_id = ap.asset_id
  where ap.album_id = a.album_id
  order by ap.position asc, ap.created_at asc
  limit 1
) first_photo on true
where a.status = 'published'
order by a.published_at desc nulls last, a.created_at desc
limit $1 offset $2;

-- name: GetPublishedPhotoAlbumBySlug :one
select
  a.album_id,
  a.slug,
  a.title,
  a.title_en,
  a.description,
  a.description_en,
  a.tags,
  a.published_at,
  a.created_at,
  a.updated_at,
  (select count(*) from album_photos p where p.album_id = a.album_id)::int as photo_count,
  coalesce(cover.public_url, first_photo.public_url) as cover_url,
  coalesce(cover.width, first_photo.width, 0)::int as cover_width,
  coalesce(cover.height, first_photo.height, 0)::int as cover_height
from photo_albums a
left join album_photos cover_photo on cover_photo.photo_id = a.cover_photo_id
left join assets cover on cover.asset_id = cover_photo.asset_id
left join lateral (
  select ast.public_url, ast.width, ast.height
  from album_photos ap
  join assets ast on ast.asset_id = ap.asset_id
  where ap.album_id = a.album_id
  order by ap.position asc, ap.created_at asc
  limit 1
) first_photo on true
where a.slug = $1 and a.status = 'published';

-- name: CountPublishedAlbumPhotosBySlug :one
select count(*)::int as total
from album_photos p
join photo_albums a on a.album_id = p.album_id
where a.slug = $1 and a.status = 'published';

-- name: ListPublishedAlbumPhotosBySlug :many
select
  p.photo_id,
  p.alt,
  p.caption,
  p.width,
  p.height,
  p.taken_at,
  p.position,
  ast.public_url as url
from album_photos p
join photo_albums a on a.album_id = p.album_id
join assets ast on ast.asset_id = p.asset_id
where a.slug = $1 and a.status = 'published'
order by p.position asc, p.created_at asc
limit $2 offset $3;

-- 相册后台管理。

-- name: ListAdminPhotoAlbums :many
select
  a.album_id,
  a.slug,
  a.title,
  a.title_en,
  a.description,
  a.description_en,
  a.status,
  a.tags,
  a.cover_photo_id,
  a.published_at,
  a.created_by,
  a.created_at,
  a.updated_at,
  (select count(*) from album_photos p where p.album_id = a.album_id)::int as photo_count,
  coalesce(cover.public_url, first_photo.public_url) as cover_url,
  coalesce(cover.width, first_photo.width, 0)::int as cover_width,
  coalesce(cover.height, first_photo.height, 0)::int as cover_height
from photo_albums a
left join album_photos cover_photo on cover_photo.photo_id = a.cover_photo_id
left join assets cover on cover.asset_id = cover_photo.asset_id
left join lateral (
  select ast.public_url, ast.width, ast.height
  from album_photos ap
  join assets ast on ast.asset_id = ap.asset_id
  where ap.album_id = a.album_id
  order by ap.position asc, ap.created_at asc
  limit 1
) first_photo on true
order by a.updated_at desc, a.created_at desc;

-- name: GetPhotoAlbumByID :one
select
  a.album_id,
  a.slug,
  a.title,
  a.title_en,
  a.description,
  a.description_en,
  a.status,
  a.tags,
  a.cover_photo_id,
  a.published_at,
  a.created_by,
  a.created_at,
  a.updated_at,
  (select count(*) from album_photos p where p.album_id = a.album_id)::int as photo_count,
  coalesce(cover.public_url, first_photo.public_url) as cover_url,
  coalesce(cover.width, first_photo.width, 0)::int as cover_width,
  coalesce(cover.height, first_photo.height, 0)::int as cover_height
from photo_albums a
left join album_photos cover_photo on cover_photo.photo_id = a.cover_photo_id
left join assets cover on cover.asset_id = cover_photo.asset_id
left join lateral (
  select ast.public_url, ast.width, ast.height
  from album_photos ap
  join assets ast on ast.asset_id = ap.asset_id
  where ap.album_id = a.album_id
  order by ap.position asc, ap.created_at asc
  limit 1
) first_photo on true
where a.album_id = $1;

-- name: CreatePhotoAlbum :one
insert into photo_albums (
  slug,
  title,
  title_en,
  description,
  description_en,
  tags,
  created_by
) values (
  $1, $2, $3, $4, $5, $6, $7
)
returning album_id, slug, title, title_en, description, description_en, status, tags, cover_photo_id, published_at, created_by, created_at, updated_at;

-- name: UpdatePhotoAlbum :one
update photo_albums
set
  slug = $2,
  title = $3,
  title_en = $4,
  description = $5,
  description_en = $6,
  tags = $7,
  updated_at = now()
where album_id = $1
returning album_id, slug, title, title_en, description, description_en, status, tags, cover_photo_id, published_at, created_by, created_at, updated_at;

-- 首次发布写入 published_at 并保持稳定；重新发布沿用原时间。
-- name: PublishPhotoAlbum :one
update photo_albums
set
  status = 'published',
  published_at = coalesce(published_at, now()),
  updated_at = now()
where album_id = $1
  and status = 'draft'
returning album_id, slug, title, title_en, description, description_en, status, tags, cover_photo_id, published_at, created_by, created_at, updated_at;

-- name: UnpublishPhotoAlbum :one
update photo_albums
set
  status = 'draft',
  updated_at = now()
where album_id = $1
  and status = 'published'
returning album_id, slug, title, title_en, description, description_en, status, tags, cover_photo_id, published_at, created_by, created_at, updated_at;

-- name: SetPhotoAlbumCover :one
update photo_albums
set
  cover_photo_id = $2,
  updated_at = now()
where album_id = $1
returning album_id, slug, title, title_en, description, description_en, status, tags, cover_photo_id, published_at, created_by, created_at, updated_at;

-- name: DeletePhotoAlbum :exec
delete from photo_albums
where album_id = $1;

-- name: CountAlbumPhotosByID :one
select count(*)::int as total
from album_photos
where album_id = $1;

-- 相册照片管理。

-- name: ListAlbumPhotos :many
select
  p.photo_id,
  p.album_id,
  p.asset_id,
  p.alt,
  p.caption,
  p.width,
  p.height,
  p.taken_at,
  p.position,
  p.created_at,
  ast.public_url,
  ast.filename,
  ast.content_type,
  ast.byte_size
from album_photos p
join assets ast on ast.asset_id = p.asset_id
where p.album_id = $1
order by p.position asc, p.created_at asc;

-- name: GetAlbumPhotoByID :one
select
  p.photo_id,
  p.album_id,
  p.asset_id,
  p.alt,
  p.caption,
  p.width,
  p.height,
  p.taken_at,
  p.position,
  p.created_at,
  ast.public_url,
  ast.storage_key,
  ast.filename,
  ast.content_type,
  ast.byte_size
from album_photos p
join assets ast on ast.asset_id = p.asset_id
where p.album_id = $1 and p.photo_id = $2;

-- 照片上传后端代理写库：先建 assets 元数据（含服务端提取的宽高与公开 URL），再建 album_photos。
-- name: CreateAlbumPhotoAsset :one
insert into assets (
  filename,
  content_type,
  byte_size,
  storage_key,
  public_url,
  created_by,
  width,
  height
) values (
  $1, $2, $3, $4, $5, $6, $7, $8
)
returning asset_id, filename, content_type, byte_size, storage_key, public_url, created_by, created_at, status, deleted_at, width, height;

-- position 用同一语句内的子查询追加到相册末尾，避免并发下的重复位次。
-- name: CreateAlbumPhoto :one
insert into album_photos (
  album_id,
  asset_id,
  alt,
  caption,
  width,
  height,
  taken_at,
  position
) values (
  $1, $2, $3, $4, $5, $6, $7,
  (select coalesce(max(position), 0) + 1 from album_photos where album_id = $1)
)
returning photo_id, album_id, asset_id, alt, caption, width, height, taken_at, position, created_at;

-- name: UpdateAlbumPhoto :one
update album_photos
set
  alt = $3,
  caption = $4,
  taken_at = $5
where album_id = $1 and photo_id = $2
returning photo_id, album_id, asset_id, alt, caption, width, height, taken_at, position, created_at;

-- name: DeleteAlbumPhoto :execrows
delete from album_photos
where album_id = $1 and photo_id = $2;

-- 被删除的照片若正是封面，清空封面引用回退第一张照片。
-- name: ClearPhotoAlbumCoverIfRemoved :exec
update photo_albums
set cover_photo_id = null, updated_at = now()
where album_id = $1 and cover_photo_id = $2;

-- 整体重排：按前端给出的顺序数组一次性写回 position，两条数组按下标对齐。
-- name: ReorderAlbumPhotos :exec
update album_photos p
set position = ord.position
from (
  select ids.photo_id, pos.position
  from unnest(sqlc.arg(photo_ids)::uuid[]) with ordinality as ids(photo_id, idx)
  join unnest(sqlc.arg(positions)::int[]) with ordinality as pos(position, idx) on pos.idx = ids.idx
) ord
where p.photo_id = ord.photo_id
  and p.album_id = sqlc.arg(album_id);

-- 删除相册前收集对象存储 key，用于尽力删除对象并软删 assets。
-- name: ListAlbumAssetKeys :many
select ast.asset_id, ast.storage_key
from album_photos p
join assets ast on ast.asset_id = p.asset_id
where p.album_id = $1;

-- name: SoftDeleteAlbumAssets :execrows
update assets
set status = 'deleted', deleted_at = now()
where status <> 'deleted'
  and asset_id in (select asset_id from album_photos where album_id = $1);
