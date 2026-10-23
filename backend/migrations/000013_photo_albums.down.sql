drop index if exists idx_album_photos_album_position;
drop index if exists idx_photo_albums_status_published_at;

alter table assets
  drop column if exists height,
  drop column if exists width;

alter table photo_albums
  drop constraint if exists photo_albums_cover_photo_id_fkey;

drop table if exists album_photos;
drop table if exists photo_albums;
drop type if exists photo_album_status;
