-- 公开查询。

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
