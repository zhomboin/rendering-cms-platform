-- name: GetArticleBySlug :one
select
  article_id,
  slug,
  article_name,
  title,
  summary,
  body_mdx,
  status,
  tags,
  featured,
  featured_rank,
  featured_at,
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version,
  search_vector
from articles
where slug = $1 and status = 'published';

-- name: GetArticleByArticleName :one
select
  article_id,
  slug,
  article_name,
  title,
  summary,
  body_mdx,
  status,
  tags,
  featured,
  featured_rank,
  featured_at,
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version,
  search_vector
from articles
where article_name = $1 and status = 'published';

-- name: ListPublishedArticles :many
select
  article_id,
  slug,
  article_name,
  title,
  summary,
  body_mdx,
  status,
  tags,
  featured,
  featured_rank,
  featured_at,
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version,
  search_vector
from articles
where status = 'published'
order by published_at desc nulls last, created_at desc;
