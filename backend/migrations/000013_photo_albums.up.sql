create type photo_album_status as enum ('draft', 'published');

create table photo_albums (
  album_id uuid primary key default gen_random_uuid(),
  slug text not null,
  title text not null,
  title_en text,
  description text not null,
  description_en text,
  status photo_album_status not null default 'draft',
  tags text[] not null default '{}',
  cover_photo_id uuid,
  published_at timestamptz,
  created_by uuid not null references users(user_id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint photo_albums_slug_key unique (slug),
  constraint photo_albums_slug_format_check check (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

create table album_photos (
  photo_id uuid primary key default gen_random_uuid(),
  album_id uuid not null references photo_albums(album_id) on delete cascade,
  asset_id uuid not null references assets(asset_id),
  alt text not null,
  caption text,
  width integer not null,
  height integer not null,
  taken_at timestamptz,
  position integer not null default 0,
  created_at timestamptz not null default now(),
  constraint album_photos_size_check check (width > 0 and height > 0)
);

-- 封面指向相册内照片；两表互相引用，表建齐后再补外键。
alter table photo_albums
  add constraint photo_albums_cover_photo_id_fkey
  foreign key (cover_photo_id) references album_photos(photo_id);

alter table assets
  add column width integer,
  add column height integer;

create index idx_photo_albums_status_published_at on photo_albums (status, published_at desc);
create index idx_album_photos_album_position on album_photos (album_id, position);
