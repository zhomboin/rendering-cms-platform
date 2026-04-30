-- name: GetAssetByID :one
select asset_id, filename, content_type, byte_size, storage_key, public_url, created_by, created_at
from assets
where asset_id = $1;

-- name: ListAssets :many
select asset_id, filename, content_type, byte_size, storage_key, public_url, created_by, created_at
from assets
order by created_at desc;
