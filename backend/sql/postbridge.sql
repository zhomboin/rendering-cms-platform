-- name: LockPostBridgeArticle :one
select * from articles
where article_id = $1
for update;

-- name: EnsurePostBridgeArticleState :one
insert into postbridge_article_states (source_id, article_id, source_version, state_seq)
values ($1, $2, $3, nextval('postbridge_state_seq'))
on conflict (source_id, article_id) do update
set source_version = excluded.source_version,
    state_seq = case
      when postbridge_article_states.source_version = excluded.source_version
      then postbridge_article_states.state_seq
      else excluded.state_seq
    end
returning state_seq;

-- name: CreatePostBridgeAttempt :one
insert into postbridge_attempts (
  id, source_id, article_id, request_id, source_version, state_seq,
  target_account_id, authorization_id, authorization_expires_at, status, data
) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
returning *;

-- name: GetPostBridgeAttempt :one
select * from postbridge_attempts where id = $1;

-- name: GetPostBridgeAttemptByRequest :one
select * from postbridge_attempts
where source_id = $1 and article_id = $2 and request_id = $3;

-- name: ListPostBridgeAttempts :many
select a.* from postbridge_attempts a
where a.source_id = sqlc.arg(source_id)
  and (sqlc.narg(article_id)::uuid is null or a.article_id = sqlc.narg(article_id))
  and (sqlc.arg(status)::text = '' or a.status = sqlc.arg(status))
order by a.created_at desc, a.id desc
limit sqlc.arg(page_size)::int offset sqlc.arg(page_offset)::bigint;

-- name: CountPostBridgeAttempts :one
select count(*) from postbridge_attempts a
where a.source_id = sqlc.arg(source_id)
  and (sqlc.narg(article_id)::uuid is null or a.article_id = sqlc.narg(article_id))
  and (sqlc.arg(status)::text = '' or a.status = sqlc.arg(status));

-- name: ClaimPostBridgeAttempt :one
update postbridge_attempts
set lease_token = gen_random_uuid()::text,
    lease_expires_at = clock_timestamp() + interval '90 seconds'
where id = $1
  and (lease_expires_at is null or lease_expires_at <= clock_timestamp())
returning *;

-- name: ClaimNextPostBridgeAttempt :one
with candidate as (
  select pending.id from postbridge_attempts pending
  where pending.source_id = $1
    and pending.next_sync_at <= clock_timestamp()
    and (pending.lease_expires_at is null or pending.lease_expires_at <= clock_timestamp())
  order by pending.next_sync_at, pending.id
  limit 1
  for update skip locked
)
update postbridge_attempts a
set lease_token = gen_random_uuid()::text,
    lease_expires_at = clock_timestamp() + interval '90 seconds'
from candidate
where a.id = candidate.id
returning a.*;

-- name: SavePostBridgeAttempt :one
update postbridge_attempts a
set status = sqlc.arg(status),
    data = sqlc.arg(data),
    next_sync_at = case
      when a.revoked_at is not null and not sqlc.arg(revocation_observed)::boolean
      then clock_timestamp()
      else sqlc.narg(next_sync_at)
    end,
    cancel_pending = case
      when a.revoked_at is not null and not sqlc.arg(revocation_observed)::boolean
      then a.cancel_pending
      else sqlc.arg(cancel_pending)
    end,
    lease_token = case when sqlc.arg(release_lease)::boolean then '' else a.lease_token end,
    lease_expires_at = case when sqlc.arg(release_lease)::boolean then null else a.lease_expires_at end,
    updated_at = clock_timestamp()
where a.id = sqlc.arg(id)
  and a.lease_token = sqlc.arg(lease_token)
  and a.lease_expires_at > clock_timestamp()
returning a.*;

-- name: RevokePostBridgeAttempt :one
update postbridge_attempts
set revoked_at = clock_timestamp(),
    updated_at = clock_timestamp(),
    cancel_pending = true,
    next_sync_at = clock_timestamp()
where id = $1 and revoked_at is null
returning *;

-- name: EnqueuePostBridgeRevocation :exec
select postbridge_enqueue_event(
  sqlc.arg(source_id)::text,
  sqlc.arg(article_id)::uuid,
  sqlc.arg(source_version)::bigint,
  'authorization_revoked',
  sqlc.arg(article_status)::text,
  sqlc.arg(authorization_id)::text,
  null
);

-- name: ClaimPostBridgeEvent :one
with candidate as (
  select pending.id from postbridge_events pending
  where pending.source_id = $1
    and pending.delivered_at is null
    and pending.next_attempt_at <= clock_timestamp()
    and (pending.lease_expires_at is null or pending.lease_expires_at <= clock_timestamp())
  order by pending.next_attempt_at, pending.id
  limit 1
  for update skip locked
)
update postbridge_events e
set lease_token = gen_random_uuid()::text,
    lease_expires_at = clock_timestamp() + interval '90 seconds',
    attempts = e.attempts + 1
from candidate
where e.id = candidate.id
returning e.*;

-- name: FreezePostBridgeEvent :execrows
update postbridge_events e
set body = sqlc.arg(body),
    snapshot = null
where e.id = sqlc.arg(id)
  and e.lease_token = sqlc.arg(lease_token)
  and e.lease_expires_at > clock_timestamp()
  and e.snapshot is not null;

-- name: FinishPostBridgeEvent :execrows
update postbridge_events e
set delivered_at = case when sqlc.narg(next_attempt_at)::timestamptz is null then clock_timestamp() else null end,
    next_attempt_at = coalesce(sqlc.narg(next_attempt_at), e.next_attempt_at),
    last_error_code = sqlc.arg(error_code),
    snapshot = case when sqlc.narg(next_attempt_at)::timestamptz is null then null else e.snapshot end,
    lease_token = '',
    lease_expires_at = null
where e.id = sqlc.arg(id)
  and e.lease_token = sqlc.arg(lease_token)
  and e.lease_expires_at > clock_timestamp();

-- name: SavePostBridgeRegistration :exec
insert into postbridge_registrations (source_id, environment, registration_id, status)
values ($1, $2, $3, $4)
on conflict (source_id, environment) do update
set registration_id = excluded.registration_id,
    status = excluded.status,
    updated_at = clock_timestamp();

-- name: GetPostBridgeRegistration :one
select * from postbridge_registrations where source_id = $1 and environment = $2;


-- name: HasPendingPostBridgeArticleEvents :one
select exists (
  select 1 from postbridge_events
  where source_id = $1 and article_id = $2 and state_seq < $3
    and delivered_at is null and event_type <> 'authorization_revoked'
);

-- name: InsertPostBridgeConnection :exec
insert into postbridge_connections (source_id, environment, origin, client_id)
values ($1, $2, $3, $4)
on conflict do nothing;

-- name: GetPostBridgeConnection :one
select * from postbridge_connections;
