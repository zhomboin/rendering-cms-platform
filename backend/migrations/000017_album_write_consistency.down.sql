do $$
begin
  if exists(select 1 from album_object_cleanup) then
    raise exception 'pending album object cleanup prevents rollback';
  end if;
end;
$$;
drop table album_object_cleanup;
drop index idx_album_photos_asset_id;
alter table album_photos drop constraint album_photos_album_position_key;
