-- name: CreateAsset :one
insert into assets (
  filename,
  content_type,
  byte_size,
  storage_key,
  public_url,
  created_by
) values (
  $1, $2, $3, $4, $5, $6
)
returning asset_id, filename, content_type, byte_size, storage_key, public_url, created_by, created_at, status, deleted_at;

-- name: GetAssetByID :one
select asset_id, filename, content_type, byte_size, storage_key, public_url, created_by, created_at, status, deleted_at
from assets
where asset_id = $1;

-- name: ListAssets :many
select asset_id, filename, content_type, byte_size, storage_key, public_url, created_by, created_at, status, deleted_at
from assets
order by created_at desc;

-- name: UpdateAssetStatus :one
update assets
set
  status = sqlc.arg(status)::asset_status,
  deleted_at = sqlc.arg(deleted_at)
where asset_id = sqlc.arg(asset_id)
returning asset_id, filename, content_type, byte_size, storage_key, public_url, created_by, created_at, status, deleted_at;

-- name: CreateDownloadEvent :one
insert into download_events (
  asset_id,
  ip_hash,
  user_agent
) values (
  $1, $2, $3
)
returning event_id, asset_id, ip_hash, user_agent, created_at;
