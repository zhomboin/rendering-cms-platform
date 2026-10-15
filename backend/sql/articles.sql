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
  featured_rank,
  featured_at,
  cover_image_url,
  author_id
) values (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
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
  featured_rank,
  featured_at,
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
  featured_rank = $9,
  featured_at = $10,
  cover_image_url = $11,
  updated_at = now()
where article_id = $1
  and status = 'draft'
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
  featured_rank,
  featured_at,
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
  published_at = now(),
  updated_at = now()
where article_id = $1
  and status = 'draft'
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
  featured_rank,
  featured_at,
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version,
  search_vector;

-- name: UnpublishArticle :one
update articles
set
  status = 'draft',
  updated_at = now()
where article_id = $1
  and status = 'published'
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
  featured_rank,
  featured_at,
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version,
  search_vector;

-- name: PruneArticleLogs :execrows
-- 每篇文章保留最近 @keep_revisions 个修订快照和首版（version = 1），删除更早的快照。
-- articles.version 是触发器维护的当前版本号，等价于该文章最大的 log version。
delete from article_logs log
using articles art
where log.article_id = art.article_id
  and log.version > 1
  and log.version <= art.version - @keep_revisions;

-- name: ListArticleRevisions :many
-- 版本列表不取 body_mdx，保持响应瘦身；单版本详情单独查询。
select
  article_id,
  slug,
  article_name,
  title,
  summary,
  status,
  tags,
  featured,
  featured_rank,
  featured_at,
  cover_image_url,
  published_at,
  version,
  updated_at
from article_logs
where article_id = $1
order by version desc;

-- name: GetArticleRevision :one
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
  version
from article_logs
where article_id = $1
  and version = $2;

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
  featured_rank,
  featured_at,
  cover_image_url,
  published_at,
  author_id
) values (
  $1, $2, $3, $4, $5, 'published', $6, $7, $8, $9, $10, $11, $12
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
  featured_rank = excluded.featured_rank,
  featured_at = excluded.featured_at,
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
  featured_rank,
  featured_at,
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version,
  search_vector;
