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
