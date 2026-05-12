-- name: SearchPublishedArticles :many
select
  article_id,
  slug,
  title,
  summary,
  published_at
from articles
where status = 'published'
  and search_vector @@ plainto_tsquery('simple', @query)
order by ts_rank(search_vector, plainto_tsquery('simple', @query)) desc, published_at desc;
