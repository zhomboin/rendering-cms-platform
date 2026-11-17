-- name: ListSiteViewTrend :many
with days as (
  select generate_series(current_date - ((sqlc.arg(days)::int - 1) * interval '1 day'), current_date, interval '1 day')::date as view_date
), views as (
  select view_date, sum(views)::int as views
  from (
    select view_date, views
    from site_view_history
    where view_date >= current_date - ((sqlc.arg(days)::int - 1) * interval '1 day')
      and view_date < current_date
    union all
    select view_date, views
    from site_view_daily
    where view_date >= current_date - ((sqlc.arg(days)::int - 1) * interval '1 day')
  ) combined
  group by view_date
)
select days.view_date, coalesce(views.views, 0)::int as views
from days
left join views on views.view_date = days.view_date
order by days.view_date asc;

-- name: ListArticleViewTrend :many
select
  combined.view_date,
  a.slug,
  a.title,
  sum(combined.views)::int as views
from (
  select article_id, view_date, views
  from article_view_history
  where view_date >= current_date - ((sqlc.arg(days)::int - 1) * interval '1 day')
    and view_date < current_date
  union all
  select article_id, view_date, views
  from article_view_daily
  where view_date >= current_date - ((sqlc.arg(days)::int - 1) * interval '1 day')
) combined
join articles a on a.article_id = combined.article_id
where a.status = 'published'
group by combined.view_date, a.slug, a.title
order by combined.view_date asc, views desc, a.slug asc;
