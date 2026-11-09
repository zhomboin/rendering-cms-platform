-- 与公开图片来源变更使用相同锁顺序，先锁来源，再锁相册。
-- name: LockAlbumImageWrites :exec
select postbridge_lock_image_provenance();

-- name: LockPhotoAlbum :one
select album_id from photo_albums where album_id = $1 for update;

-- name: EnqueueAlbumObjectCleanup :exec
insert into album_object_cleanup (storage_key, next_attempt_at)
values ($1, $2)
on conflict (storage_key) do update
set next_attempt_at = least(album_object_cleanup.next_attempt_at, excluded.next_attempt_at);

-- name: CancelAlbumObjectCleanup :execrows
delete from album_object_cleanup where storage_key = $1 and lease_token = '';

-- 不删除仍关联照片的对象，也不接管未过期的租约。
-- name: ClaimAlbumObjectCleanup :one
with candidate as (
  select c.storage_key
  from album_object_cleanup c
  where (sqlc.arg(storage_key)::text = '' or c.storage_key = sqlc.arg(storage_key))
    and (sqlc.arg(storage_key)::text <> '' or c.next_attempt_at <= now())
    and c.lease_until < now()
    and not exists (
      select 1 from assets a join album_photos p on p.asset_id = a.asset_id
      where a.storage_key = c.storage_key
    )
  order by c.next_attempt_at, c.storage_key
  for update of c skip locked
  limit 1
)
update album_object_cleanup c
set attempts = c.attempts + 1,
    lease_token = gen_random_uuid()::text,
    lease_until = now() + interval '1 minute'
from candidate
where c.storage_key = candidate.storage_key
returning c.storage_key, c.lease_token, c.attempts;

-- name: CompleteAlbumObjectCleanup :exec
delete from album_object_cleanup where storage_key = $1 and lease_token = $2;

-- name: RetryAlbumObjectCleanup :exec
update album_object_cleanup
set next_attempt_at = $3, lease_token = '', lease_until = '-infinity'
where storage_key = $1 and lease_token = $2;
