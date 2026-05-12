-- name: CreateLoginAttempt :one
insert into login_attempts (
  email,
  ip_hash,
  success,
  failure_reason
) values (
  sqlc.arg(email),
  sqlc.arg(ip_hash),
  sqlc.arg(success),
  sqlc.narg(failure_reason)
)
returning attempt_id, email, ip_hash, success, failure_reason, created_at;

-- name: ListRecentFailedLoginAttemptsByEmail :many
select created_at
from login_attempts
where success = false
  and created_at >= sqlc.arg(created_at)
  and email = sqlc.arg(email)
order by created_at desc;

-- name: ListRecentFailedLoginAttemptsByIP :many
select created_at
from login_attempts
where success = false
  and created_at >= sqlc.arg(created_at)
  and ip_hash = sqlc.arg(ip_hash)
order by created_at desc;
