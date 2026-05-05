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

-- name: ArchiveArticleViewsForDate :exec
insert into article_view_history (article_id, view_date, views)
select d.article_id, d.view_date, d.views
from article_view_daily d
where d.view_date = $1
on conflict (article_id, view_date)
do update set
  views = excluded.views,
  archived_at = now();

-- name: DeleteArticleViewDailyForDate :exec
delete from article_view_daily
where view_date = $1;

-- name: ArchiveSiteViewsForDate :exec
insert into site_view_history (view_date, views)
select d.view_date, d.views
from site_view_daily d
where d.view_date = $1
on conflict (view_date)
do update set
  views = excluded.views,
  archived_at = now();

-- name: DeleteSiteViewDailyForDate :exec
delete from site_view_daily
where view_date = $1;
