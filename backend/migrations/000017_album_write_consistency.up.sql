-- 先确定已有照片的稳定顺序，再约束同一相册的位次唯一。
with ranked as (
  select photo_id, row_number() over (
    partition by album_id order by position, created_at, photo_id
  )::int as position
  from album_photos
)
update album_photos p set position = ranked.position
from ranked where ranked.photo_id = p.photo_id;

alter table album_photos
  add constraint album_photos_album_position_key
  unique (album_id, position) deferrable initially deferred;

create index idx_album_photos_asset_id on album_photos (asset_id);

-- 上传前登记清理意图；关联照片与撤销清理意图在同一事务提交。
create table album_object_cleanup (
  storage_key text primary key,
  attempts integer not null default 0 check (attempts >= 0),
  next_attempt_at timestamptz not null default now(),
  lease_token text not null default '',
  lease_until timestamptz not null default '-infinity',
  created_at timestamptz not null default now()
);

create index idx_album_object_cleanup_due on album_object_cleanup (next_attempt_at);
