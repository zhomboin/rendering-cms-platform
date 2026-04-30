-- name: GetUserByID :one
select user_id, email, name, password_hash, role, created_at, updated_at
from users
where user_id = $1;

-- name: GetUserByEmail :one
select user_id, email, name, password_hash, role, created_at, updated_at
from users
where email = $1;
