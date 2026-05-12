-- name: CreateComment :one
insert into comments (
  article_id,
  author_name,
  author_email,
  body,
  ip_hash,
  user_agent
)
select
  article_id,
  sqlc.arg(author_name),
  sqlc.narg(author_email),
  sqlc.arg(body),
  sqlc.arg(ip_hash),
  sqlc.narg(user_agent)
from articles
where slug = sqlc.arg(slug) and status = 'published'
returning
  comment_id,
  article_id,
  author_name,
  author_email,
  body,
  status,
  ip_hash,
  user_agent,
  created_at,
  reviewed_at;

-- name: ListPendingComments :many
select
  comment_id,
  article_id,
  author_name,
  author_email,
  body,
  status,
  ip_hash,
  user_agent,
  created_at,
  reviewed_at
from comments
where status = 'pending'
order by created_at asc;

-- name: ListApprovedCommentsByArticle :many
select
  comment_id,
  article_id,
  author_name,
  author_email,
  body,
  status,
  ip_hash,
  user_agent,
  created_at,
  reviewed_at
from comments
where article_id = $1 and status = 'approved'
order by created_at asc;

-- name: ListApprovedCommentsByArticleSlug :many
select
  c.comment_id,
  c.author_name,
  c.body,
  c.created_at
from comments c
join articles a on a.article_id = c.article_id
where a.slug = $1
  and a.status = 'published'
  and c.status = 'approved'
order by c.created_at asc;

-- name: ListRecentCommentTimesByIPHash :many
select created_at
from comments
where ip_hash = $1
  and created_at >= $2
order by created_at desc;

-- name: ListAdminComments :many
select
  c.comment_id,
  c.article_id,
  a.slug as article_slug,
  a.title as article_title,
  c.author_name,
  c.author_email,
  c.body,
  c.status,
  c.user_agent,
  c.created_at,
  c.reviewed_at
from comments c
join articles a on a.article_id = c.article_id
order by c.created_at desc;

-- name: ReviewComment :one
update comments
set
  status = sqlc.arg(status)::comment_status,
  reviewed_at = now()
where comment_id = sqlc.arg(comment_id)
  and sqlc.arg(status)::comment_status in ('approved', 'rejected')
returning
  comment_id,
  article_id,
  author_name,
  author_email,
  body,
  status,
  ip_hash,
  user_agent,
  created_at,
  reviewed_at;
