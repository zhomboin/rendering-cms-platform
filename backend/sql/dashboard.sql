-- 驾驶舱只读聚合：历史表只纳入今天之前的数据，日表保留归档后新到的增量。
-- name: GetDashboardOverview :one
with bounds as (
 select (current_date - (sqlc.arg(days)::int - 1))::date as start_date, current_date as end_date
), site_views as (
 select view_date, views from site_view_history, bounds where view_date >= start_date and view_date < end_date
 union all
 select view_date, views from site_view_daily, bounds where view_date between start_date and end_date
), article_views as (
 select h.article_id, h.views from article_view_history h, bounds where h.view_date >= start_date and h.view_date < end_date
 union all
 select d.article_id, d.views from article_view_daily d, bounds where d.view_date between start_date and end_date
)
select
 now()::timestamptz as generated_at,
 bounds.start_date::date as start_date, bounds.end_date::date as end_date,
 (select coalesce(sum(views), 0)::bigint from site_views where view_date = bounds.end_date) as today_views,
 (select coalesce(sum(views), 0)::bigint from site_views) as period_views,
 (select coalesce(sum(v.views), 0)::bigint from article_views v join articles a on a.article_id = v.article_id where a.status = 'published') as article_views,
 (select coalesce(sum(views), 0)::bigint from album_view_daily where view_date between bounds.start_date and bounds.end_date) as album_views,
 (select count(*) from comments where created_at >= bounds.start_date and created_at < bounds.end_date + 1) as new_comments,
 (select count(*) from comments where status = 'pending') as pending_comments,
 (select count(*) from articles where status = 'published') as published_articles,
 (select count(*) from articles where status = 'draft') as draft_articles,
 (select count(*) from articles where status = 'private') as private_articles,
 (select count(*) from articles where status = 'archived') as archived_articles,
 (select count(*) from photo_albums where status = 'published') as published_albums,
 (select count(*) from photo_albums where status = 'draft') as draft_albums,
 (select count(*) from album_photos p join assets a on a.asset_id = p.asset_id where a.status = 'active' and a.deleted_at is null) as photo_count,
 (select count(*) from articles where status = 'published' and nullif(btrim(cover_image_url), '') is null) as missing_cover_articles,
 (select count(*) from assets where status = 'active' and deleted_at is null) as asset_count,
 (select count(*) from assets where status = 'active' and deleted_at is null and content_type like 'image/%') as image_count,
 (select coalesce(sum(byte_size::bigint), 0)::bigint from assets where status = 'active' and deleted_at is null) as total_bytes
from bounds;

-- name: ListDashboardRecentArticles :many
select article_id, slug, title, status, coalesce(cover_image_url, '')::text as cover_image_url, tags, updated_at, published_at
from articles
where (sqlc.arg(status)::text = '' or status::text = sqlc.arg(status)::text)
order by updated_at desc, article_id
limit sqlc.arg(row_limit)::int;

-- name: ListDashboardPendingComments :many
select c.comment_id, c.article_id, a.title as article_title, c.author_name, left(c.body, 120)::text as body, c.created_at
from comments c
join articles a on a.article_id = c.article_id
where c.status = 'pending'
order by c.created_at desc, c.comment_id
limit 3;

-- name: ListDashboardRecentImages :many
select asset_id, filename, public_url::text as public_url, byte_size::bigint as byte_size, created_at
from assets
where status = 'active' and deleted_at is null and content_type like 'image/%'
 and public_url ~ '^https?://[^[:space:]/?#@]+([/?#][^[:space:]]*)?$'
order by created_at desc, asset_id
limit 6;

-- name: ListDashboardArticles :many
with all_views as (
 select article_id, view_date, views from article_view_history where view_date < current_date
 union all
 select article_id, view_date, views from article_view_daily where view_date <= current_date
), counts as (
 select article_id,
 coalesce(sum(views) filter (where view_date >= current_date - (sqlc.arg(days)::int - 1)), 0)::bigint as period_views,
 sum(views)::bigint as total_views
 from all_views group by article_id
)
select a.article_id, a.slug, a.title, coalesce(a.cover_image_url, '')::text as cover_image_url, a.tags,
 coalesce(c.period_views, 0)::bigint as period_views, coalesce(c.total_views, 0)::bigint as total_views, a.published_at
from articles a
left join counts c on c.article_id = a.article_id
where a.status = 'published'
order by period_views desc, a.published_at desc nulls last, a.article_id
limit 5;

-- name: ListDashboardAlbums :many
with counts as (
 select album_id, sum(views)::bigint as period_views from album_view_daily
 where view_date between current_date - (sqlc.arg(days)::int - 1) and current_date
 group by album_id
)
select a.album_id, a.slug, a.title, a.status,
 coalesce(cover.public_url, first_photo.public_url, '')::text as cover_url,
 (select count(*) from album_photos p join assets ast on ast.asset_id = p.asset_id where p.album_id = a.album_id and ast.status = 'active' and ast.deleted_at is null) as photo_count,
 coalesce(c.period_views, 0)::bigint as period_views, a.updated_at
from photo_albums a
left join counts c on c.album_id = a.album_id
left join album_photos cover_photo on cover_photo.photo_id = a.cover_photo_id and cover_photo.album_id = a.album_id
left join assets cover on cover.asset_id = cover_photo.asset_id and cover.status = 'active' and cover.deleted_at is null
 and cover.public_url ~ '^https?://[^[:space:]/?#@]+([/?#][^[:space:]]*)?$'
left join lateral (
 select ast.public_url from album_photos p join assets ast on ast.asset_id = p.asset_id
 where p.album_id = a.album_id and ast.status = 'active' and ast.deleted_at is null
 and ast.public_url ~ '^https?://[^[:space:]/?#@]+([/?#][^[:space:]]*)?$'
 order by p.position, p.created_at, p.photo_id limit 1
) first_photo on true
order by (a.status = 'published') desc, period_views desc, a.updated_at desc, a.album_id
limit 4;