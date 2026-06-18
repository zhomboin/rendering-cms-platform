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
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version,
  search_vector
from articles
where article_name = $1 and status = 'published';

-- name: GetArticleByID :one
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
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version,
  search_vector
from articles
where article_id = $1;

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

-- name: ListAdminArticles :many
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
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version,
  search_vector
from articles
order by updated_at desc, created_at desc;

-- name: CreateDraftArticle :one
insert into articles (
  slug,
  article_name,
  title,
  summary,
  body_mdx,
  tags,
  featured,
  cover_image_url,
  author_id
) values (
  $1, $2, $3, $4, $5, $6, $7, $8, $9
)
returning
  article_id,
  slug,
  article_name,
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
  updated_at,
  version,
  search_vector;

-- name: UpdateDraftArticle :one
update articles
set
  slug = $2,
  article_name = $3,
  title = $4,
  summary = $5,
  body_mdx = $6,
  tags = $7,
  featured = $8,
  cover_image_url = $9,
  updated_at = now()
where article_id = $1
returning
  article_id,
  slug,
  article_name,
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
  updated_at,
  version,
  search_vector;

-- name: PublishArticle :one
update articles
set
  status = 'published',
  published_at = coalesce(published_at, now()),
  updated_at = now()
where article_id = $1
returning
  article_id,
  slug,
  article_name,
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
  updated_at,
  version,
  search_vector;

-- name: UpsertPublishedArticleFromImport :one
insert into articles (
  slug,
  article_name,
  title,
  summary,
  body_mdx,
  status,
  tags,
  featured,
  cover_image_url,
  published_at,
  author_id
) values (
  $1, $2, $3, $4, $5, 'published', $6, $7, $8, $9, $10
)
on conflict (slug)
do update set
  article_name = excluded.article_name,
  title = excluded.title,
  summary = excluded.summary,
  body_mdx = excluded.body_mdx,
  status = 'published',
  tags = excluded.tags,
  featured = excluded.featured,
  cover_image_url = excluded.cover_image_url,
  published_at = excluded.published_at,
  author_id = excluded.author_id,
  updated_at = now()
returning
  article_id,
  slug,
  article_name,
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
  updated_at,
  version,
  search_vector;
