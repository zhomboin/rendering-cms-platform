create extension if not exists pgcrypto;

create type user_role as enum ('admin', 'editor');
create type article_status as enum ('draft', 'published', 'archived');
create type comment_status as enum ('pending', 'approved', 'rejected');

create table users (
  user_id uuid primary key default gen_random_uuid(),
  email text not null unique,
  name text not null,
  password_hash text not null,
  role user_role not null default 'admin',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table articles (
  article_id uuid primary key default gen_random_uuid(),
  slug text not null unique,
  title text not null,
  summary text not null,
  body_mdx text not null,
  status article_status not null default 'draft',
  tags text[] not null default '{}',
  featured boolean not null default false,
  cover_image_url text,
  published_at timestamptz,
  author_id uuid not null references users(user_id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table article_revisions (
  revision_id uuid primary key default gen_random_uuid(),
  article_id uuid not null references articles(article_id) on delete cascade,
  title text not null,
  summary text not null,
  body_mdx text not null,
  status article_status not null,
  created_by uuid not null references users(user_id),
  created_at timestamptz not null default now()
);

create table comments (
  comment_id uuid primary key default gen_random_uuid(),
  article_id uuid not null references articles(article_id) on delete cascade,
  author_name text not null,
  author_email text,
  body text not null,
  status comment_status not null default 'pending',
  ip_hash text not null,
  user_agent text,
  created_at timestamptz not null default now(),
  reviewed_at timestamptz
);

create table article_view_daily (
  article_id uuid not null references articles(article_id) on delete cascade,
  view_date date not null,
  views integer not null default 0,
  primary key (article_id, view_date)
);

create table site_view_daily (
  view_date date primary key,
  views integer not null default 0
);

create table assets (
  asset_id uuid primary key default gen_random_uuid(),
  filename text not null,
  content_type text not null,
  byte_size integer not null,
  storage_key text not null unique,
  public_url text,
  created_by uuid not null references users(user_id),
  created_at timestamptz not null default now()
);

create table download_events (
  event_id uuid primary key default gen_random_uuid(),
  asset_id uuid not null references assets(asset_id) on delete cascade,
  ip_hash text not null,
  user_agent text,
  created_at timestamptz not null default now()
);

create index idx_articles_status_published_at on articles (status, published_at desc);
create index idx_articles_author_id on articles (author_id);
create index idx_article_revisions_article_id on article_revisions (article_id);
create index idx_comments_article_status on comments (article_id, status);
create index idx_assets_created_by on assets (created_by);
create index idx_download_events_asset_id_created_at on download_events (asset_id, created_at desc);
