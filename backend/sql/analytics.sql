-- name: UpsertArticleViewDaily :exec
insert into article_view_daily (article_id, view_date, views)
values ($1, $2, $3)
on conflict (article_id, view_date)
do update set views = article_view_daily.views + excluded.views;

-- name: UpsertSiteViewDaily :exec
insert into site_view_daily (view_date, views)
values ($1, $2)
on conflict (view_date)
do update set views = site_view_daily.views + excluded.views;

-- name: GetTodaySiteViews :one
select coalesce(views, 0)::int as views
from site_view_daily
where view_date = current_date;

-- name: ListSiteViewsLast7Days :many
with days as (
  select generate_series(current_date - interval '6 days', current_date, interval '1 day')::date as view_date
), views as (
  select view_date, sum(views)::int as views
  from (
    select view_date, views
    from site_view_history
    where view_date >= current_date - interval '6 days'
      and view_date < current_date
    union all
    select view_date, views
    from site_view_daily
    where view_date >= current_date - interval '6 days'
  ) combined
  group by view_date
)
select days.view_date, coalesce(views.views, 0)::int as views
from days
left join views on views.view_date = days.view_date
order by days.view_date asc;

-- name: ListHotArticles :many
with article_views as (
  select article_id, sum(views)::int as views
  from (
    select article_id, views
    from article_view_history
    where view_date >= current_date - interval '6 days'
      and view_date < current_date
    union all
    select article_id, views
    from article_view_daily
    where view_date >= current_date - interval '6 days'
  ) combined
  group by article_id
)
select
  a.article_id,
  a.slug,
  a.title,
  coalesce(v.views, 0)::int as views
from articles a
left join article_views v on v.article_id = a.article_id
where a.status = 'published'
order by views desc, a.published_at desc nulls last
limit $1;

-- name: ListArticleAnalyticsRows :many
with period_views as (
  select article_id, sum(views)::int as views
  from (
    select article_id, views
    from article_view_history
    where view_date >= current_date - (($1::int - 1) * interval '1 day')
      and view_date < current_date
    union all
    select article_id, views
    from article_view_daily
    where view_date >= current_date - (($1::int - 1) * interval '1 day')
  ) combined
  group by article_id
), today_views as (
  select article_id, sum(views)::int as views
  from article_view_daily
  where view_date = current_date
  group by article_id
), total_views as (
  select article_id, sum(views)::int as views
  from (
    select article_id, views
    from article_view_history
    union all
    select article_id, views
    from article_view_daily
  ) combined
  group by article_id
)
select
  a.slug,
  a.title,
  coalesce(today_views.views, 0)::int as today_views,
  coalesce(period_views.views, 0)::int as period_views,
  coalesce(total_views.views, 0)::int as total_views,
  a.published_at
from articles a
left join today_views on today_views.article_id = a.article_id
left join period_views on period_views.article_id = a.article_id
left join total_views on total_views.article_id = a.article_id
where a.status = 'published'
order by period_views desc, today_views desc, a.published_at desc nulls last;

-- name: ArchiveArticleViewsBeforeDate :exec
with moved as (
  delete from article_view_daily
  where article_view_daily.view_date < $1
  returning article_view_daily.article_id, article_view_daily.view_date, article_view_daily.views
)
insert into article_view_history (article_id, view_date, views)
select moved.article_id, moved.view_date, moved.views
from moved
on conflict (article_id, view_date)
do update set
  views = article_view_history.views + excluded.views,
  archived_at = now();

-- name: ArchiveSiteViewsBeforeDate :exec
with moved as (
  delete from site_view_daily
  where site_view_daily.view_date < $1
  returning site_view_daily.view_date, site_view_daily.views
)
insert into site_view_history (view_date, views)
select moved.view_date, moved.views
from moved
on conflict (view_date)
do update set
  views = site_view_history.views + excluded.views,
  archived_at = now();
