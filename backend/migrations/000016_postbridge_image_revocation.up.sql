-- 公开来源写入与带图授权创建共用事务锁。写入锁行的修订号还会让持有旧快照的事务失败并重试。
create table postbridge_image_provenance_lock (
  singleton boolean primary key check (singleton),
  revision bigint not null
);
insert into postbridge_image_provenance_lock (singleton, revision) values (true, 0);

create function postbridge_lock_image_provenance() returns void language sql volatile as $$
  update postbridge_image_provenance_lock set revision = revision + 1 where singleton;
$$;

create function postbridge_image_write_lock() returns trigger language plpgsql as $$
begin
  perform postbridge_lock_image_provenance();
  return null;
end;
$$;

create trigger trg_postbridge_photo_albums_image_write_lock
  before insert or update of status or delete on photo_albums
  for each statement execute function postbridge_image_write_lock();

create trigger trg_postbridge_album_photos_image_write_lock
  before insert or update of album_id, asset_id or delete on album_photos
  for each statement execute function postbridge_image_write_lock();

create trigger trg_postbridge_assets_image_write_lock
  before insert or update of public_url, status, deleted_at, content_type or delete on assets
  for each statement execute function postbridge_image_write_lock();

create function postbridge_unavailable_image_urls(image_urls text[]) returns text[]
language sql stable as $$
  select coalesce(array_agg(requested.url), array[]::text[])
  from unnest(image_urls) as requested(url)
  where requested.url is not null and not exists (
    select 1
    from assets ast
    join album_photos p on p.asset_id = ast.asset_id
    join photo_albums a on a.album_id = p.album_id
    where ast.public_url collate "C" = requested.url
      and ast.status = 'active'
      and ast.deleted_at is null
      and ast.content_type in ('image/png', 'image/jpeg', 'image/webp')
      and a.status = 'published'
  );
$$;

create index idx_postbridge_attempts_active_image_urls
  on postbridge_attempts using gin ((data->'ImageURLs'))
  where revoked_at is null;

create function postbridge_revoke_unavailable_images(image_urls text[]) returns void
language plpgsql as $$
declare
  unavailable text[] := postbridge_unavailable_image_urls(image_urls);
  revoked record;
  current_status text;
  current_version bigint;
begin
  if cardinality(unavailable) = 0 then
    return;
  end if;

  for revoked in
    update postbridge_attempts a
    set revoked_at = clock_timestamp(),
        updated_at = clock_timestamp(),
        cancel_pending = a.status <> 'completed',
        next_sync_at = case when a.status <> 'completed' then clock_timestamp() else a.next_sync_at end
    where a.revoked_at is null
      and a.authorization_expires_at > clock_timestamp()
      and (a.data->'ImageURLs') ?| unavailable
    returning a.source_id, a.article_id, a.source_version, a.authorization_id
  loop
    -- 普通读取避免与已经锁定文章、等待图片事务锁的授权创建形成循环。
    select case when status = 'archived' then 'draft' else status::text end, version
      into current_status, current_version
      from articles where article_id = revoked.article_id;
    perform postbridge_enqueue_event(
      revoked.source_id, revoked.article_id, coalesce(current_version, revoked.source_version),
      'authorization_revoked', coalesce(current_status, 'deleted'), revoked.authorization_id, null
    );
  end loop;
end;
$$;

create function postbridge_revoke_changed_photo_sources() returns trigger language plpgsql as $$
declare
  changed_urls text[];
begin
  if tg_table_name = 'photo_albums' then
    if old.status = 'published' and new.status <> 'published' then
      select array_agg(ast.public_url) into changed_urls
        from album_photos p join assets ast on ast.asset_id = p.asset_id
        where p.album_id = old.album_id;
    end if;
  elsif tg_table_name = 'album_photos' then
    select array[public_url] into changed_urls from assets where asset_id = old.asset_id;
  else
    changed_urls := array[old.public_url];
  end if;
  if changed_urls is not null then
    perform postbridge_revoke_unavailable_images(changed_urls);
  end if;
  return null;
end;
$$;

create trigger trg_postbridge_photo_albums_revoke_image_grants
  after update of status on photo_albums
  for each row execute function postbridge_revoke_changed_photo_sources();

-- 删除相册时，照片关联的级联删除同样触发撤销。
create trigger trg_postbridge_album_photos_revoke_image_grants
  after update of album_id, asset_id or delete on album_photos
  for each row execute function postbridge_revoke_changed_photo_sources();

create trigger trg_postbridge_assets_revoke_image_grants
  after update of public_url, status, deleted_at, content_type or delete on assets
  for each row execute function postbridge_revoke_changed_photo_sources();

-- 核对升级前已登记的地址，补发已失效图片的授权撤销。
select postbridge_revoke_unavailable_images(array(
  select distinct images.url collate "C"
  from postbridge_attempts a
  cross join lateral jsonb_array_elements_text(a.data->'ImageURLs') as images(url)
  where a.revoked_at is null and a.authorization_expires_at > clock_timestamp()
));
