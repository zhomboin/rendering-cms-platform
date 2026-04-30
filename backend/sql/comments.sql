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
