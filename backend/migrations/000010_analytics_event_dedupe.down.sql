drop index if exists analytics_events_unique_daily_idx;

alter table analytics_events
  drop column if exists event_date;
