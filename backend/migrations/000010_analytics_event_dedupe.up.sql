alter table analytics_events
  add column event_date date not null default current_date;

create unique index analytics_events_unique_daily_idx
  on analytics_events (
    event_type,
    ip_hash,
    coalesce(article_id, '00000000-0000-0000-0000-000000000000'::uuid),
    event_date
  );
