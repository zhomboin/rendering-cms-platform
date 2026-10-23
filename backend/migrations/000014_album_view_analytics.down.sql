drop table if exists album_view_daily;

drop index analytics_events_unique_daily_idx;

create unique index analytics_events_unique_daily_idx
  on analytics_events (
    event_type,
    ip_hash,
    coalesce(article_id, '00000000-0000-0000-0000-000000000000'::uuid),
    event_date
  );

alter table analytics_events
  drop constraint if exists analytics_events_album_id_fkey,
  drop column if exists album_id;
