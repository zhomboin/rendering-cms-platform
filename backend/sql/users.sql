-- name: GetUserByID :one
select user_id, email, name, password_hash, role, created_at, updated_at
from users
where user_id = $1;

-- name: GetUserByEmail :one
select user_id, email, name, password_hash, role, created_at, updated_at
from users
where email = $1;

-- name: GetDefaultImportAuthor :one
select user_id, email, name, password_hash, role, created_at, updated_at
from users
where role in ('admin', 'editor')
order by (role = 'admin') desc, created_at asc
limit 1;
