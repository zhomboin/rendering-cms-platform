create type asset_status as enum ('active', 'archived', 'deleted');

alter table assets
  add column status asset_status not null default 'active',
  add column deleted_at timestamptz;
