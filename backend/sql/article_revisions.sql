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
