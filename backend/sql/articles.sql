-- name: GetArticleBySlug :one
select
  article_id,
  slug,
  title,
  summary,
  body_mdx,
  status,
  tags,
  featured,
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at
from articles
where slug = $1;

-- name: ListPublishedArticles :many
select
  article_id,
  slug,
  title,
  summary,
  body_mdx,
  status,
  tags,
  featured,
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at
from articles
where status = 'published'
order by published_at desc nulls last, created_at desc;
