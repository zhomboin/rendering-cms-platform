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
select view_date, views
from site_view_daily
where view_date >= current_date - interval '6 days'
order by view_date asc;

-- name: ListHotArticles :many
select
  a.article_id,
  a.slug,
  a.title,
  coalesce(sum(v.views), 0)::int as views
from articles a
left join article_view_daily v on v.article_id = a.article_id
where a.status = 'published'
group by a.article_id, a.slug, a.title
order by views desc, a.published_at desc nulls last
limit $1;
