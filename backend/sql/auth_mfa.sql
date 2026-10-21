-- name: CreateLoginEmailCode :one
insert into login_email_codes (
  user_id,
  email,
  code_hash,
  expires_at
) values (
  sqlc.arg(user_id),
  sqlc.arg(email),
  sqlc.arg(code_hash),
  sqlc.arg(expires_at)
)
returning code_id, user_id, email, code_hash, expires_at, attempts, used_at, created_at;

-- name: GetLatestActiveLoginEmailCode :one
select code_id, user_id, email, code_hash, expires_at, attempts, used_at, created_at
from login_email_codes
where email = sqlc.arg(email)
  and used_at is null
order by created_at desc, code_id desc
limit 1;

-- name: ConsumeLoginEmailCodeAttempt :one
update login_email_codes
set attempts = attempts + 1
where code_id = sqlc.arg(code_id)
  and used_at is null
  and attempts < sqlc.arg(max_attempts)
returning attempts;

-- name: ClaimLoginEmailCode :execrows
update login_email_codes
set used_at = now()
where code_id = sqlc.arg(code_id)
  and used_at is null;

-- name: CountRecentLoginEmailCodes :one
select count(*)
from login_email_codes
where email = sqlc.arg(email)
  and created_at >= sqlc.arg(created_at);

-- name: ListRecentLoginAttemptOutcomes :many
select success
from login_attempts
where email = sqlc.arg(email)
order by created_at desc
limit sqlc.arg(row_limit);

-- name: UpdateUserLastLoginIPHash :exec
update users
set last_login_ip_hash = sqlc.arg(ip_hash),
    updated_at = now()
where user_id = sqlc.arg(user_id);
