-- 事件去重和所有日计数在同一条语句中提交，任一步失败均完整回滚。
-- name: RecordAnalyticsView :one
with recorded as (
  insert into analytics_events (article_id, album_id, event_type, ip_hash, user_agent)
  values (
    sqlc.narg(article_id), sqlc.narg(album_id), sqlc.arg(event_type),
    sqlc.arg(ip_hash), sqlc.narg(user_agent)
  )
  on conflict do nothing
  returning event_id, event_date, article_id, album_id, event_type
), article_count as (
  insert into article_view_daily (article_id, view_date, views)
  select article_id, event_date, 1 from recorded where event_type = 'article_view'
  on conflict (article_id, view_date)
  do update set views = article_view_daily.views + excluded.views
  returning view_date
), album_count as (
  insert into album_view_daily (album_id, view_date, views)
  select album_id, event_date, 1 from recorded where event_type = 'album_view'
  on conflict (album_id, view_date)
  do update set views = album_view_daily.views + excluded.views
  returning view_date
), site_count as (
  insert into site_view_daily (view_date, views)
  select event_date, 1 from recorded
  on conflict (view_date)
  do update set views = site_view_daily.views + excluded.views
  returning view_date
)
select exists(select 1 from recorded) as counted;
