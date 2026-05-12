create table analytics_events (
  event_id uuid primary key default gen_random_uuid(),
  article_id uuid references articles(article_id) on delete cascade,
  event_type text not null,
  ip_hash text not null,
  user_agent text,
  created_at timestamptz not null default now()
);

create index analytics_events_created_at_idx on analytics_events (created_at desc);
create index analytics_events_article_id_created_at_idx on analytics_events (article_id, created_at desc);
