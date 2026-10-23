alter table analytics_events
  add column album_id uuid references photo_albums(album_id) on delete cascade;

-- 每日去重从（事件类型, IP 哈希, 文章）扩展为（事件类型, IP 哈希, 文章, 相册），
-- album_view 与 article_view 事件类型不同，天然互不干扰。
drop index analytics_events_unique_daily_idx;

create unique index analytics_events_unique_daily_idx
  on analytics_events (
    event_type,
    ip_hash,
    coalesce(article_id, '00000000-0000-0000-0000-000000000000'::uuid),
    coalesce(album_id, '00000000-0000-0000-0000-000000000000'::uuid),
    event_date
  );

create table album_view_daily (
  album_id uuid not null references photo_albums(album_id) on delete cascade,
  view_date date not null,
  views integer not null default 0,
  primary key (album_id, view_date)
);
