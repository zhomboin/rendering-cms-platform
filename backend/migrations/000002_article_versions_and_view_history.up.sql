alter table articles
  add column version integer not null default 1;

create table article_logs (
  article_id uuid not null references articles(article_id) on delete cascade,
  slug text not null,
  title text not null,
  summary text not null,
  body_mdx text not null,
  status article_status not null,
  tags text[] not null default '{}',
  featured boolean not null default false,
  cover_image_url text,
  published_at timestamptz,
  author_id uuid not null references users(user_id),
  created_at timestamptz not null,
  updated_at timestamptz not null,
  version integer not null,
  primary key (article_id, version)
);

with numbered_revisions as (
  select
    r.article_id,
    a.slug,
    r.title,
    r.summary,
    r.body_mdx,
    r.status,
    row_number() over (
      partition by r.article_id
      order by r.created_at asc, r.revision_id asc
    )::integer as version,
    a.tags,
    a.featured,
    a.cover_image_url,
    a.published_at,
    r.created_by as author_id,
    r.created_at,
    r.created_at as updated_at
  from article_revisions r
  join articles a on a.article_id = r.article_id
)
insert into article_logs (
  article_id,
  slug,
  title,
  summary,
  body_mdx,
  status,
  tags,
  featured,
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version
)
select
  article_id,
  slug,
  title,
  summary,
  body_mdx,
  status,
  tags,
  featured,
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version
from numbered_revisions;

insert into article_logs (
  article_id,
  slug,
  title,
  summary,
  body_mdx,
  status,
  tags,
  featured,
  cover_image_url,
  published_at,
  author_id,
  created_at,
  updated_at,
  version
)
select
  a.article_id,
  a.slug,
  a.title,
  a.summary,
  a.body_mdx,
  a.status,
  a.tags,
  a.featured,
  a.cover_image_url,
  a.published_at,
  a.author_id,
  a.created_at,
  a.updated_at,
  1
from articles a
where not exists (
  select 1 from article_logs l where l.article_id = a.article_id
);

update articles a
set version = greatest(1, coalesce((
  select max(l.version)
  from article_logs l
  where l.article_id = a.article_id
), 1));

drop table article_revisions;

create index idx_article_logs_slug_version on article_logs (slug, version desc);

create table article_view_history (
  article_id uuid not null references articles(article_id) on delete cascade,
  view_date date not null,
  views integer not null default 0,
  archived_at timestamptz not null default now(),
  primary key (article_id, view_date)
);

create table site_view_history (
  view_date date primary key,
  views integer not null default 0,
  archived_at timestamptz not null default now()
);

create or replace function set_article_update_version()
returns trigger
language plpgsql
as $$
begin
  new.version := old.version + 1;
  new.updated_at := now();
  return new;
end;
$$;

create or replace function insert_article_log()
returns trigger
language plpgsql
as $$
begin
  insert into article_logs (
    article_id,
    slug,
    title,
    summary,
    body_mdx,
    status,
    tags,
    featured,
    cover_image_url,
    published_at,
    author_id,
    created_at,
    updated_at,
    version
  ) values (
    new.article_id,
    new.slug,
    new.title,
    new.summary,
    new.body_mdx,
    new.status,
    new.tags,
    new.featured,
    new.cover_image_url,
    new.published_at,
    new.author_id,
    new.created_at,
    new.updated_at,
    new.version
  )
  on conflict (article_id, version) do update set
    slug = excluded.slug,
    title = excluded.title,
    summary = excluded.summary,
    body_mdx = excluded.body_mdx,
    status = excluded.status,
    tags = excluded.tags,
    featured = excluded.featured,
    cover_image_url = excluded.cover_image_url,
    published_at = excluded.published_at,
    author_id = excluded.author_id,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at;

  return new;
end;
$$;

create trigger trg_articles_set_update_version
before update of slug, title, summary, body_mdx, status, tags, featured, cover_image_url, published_at, author_id
on articles
for each row
execute function set_article_update_version();

create trigger trg_articles_insert_log
after insert on articles
for each row
execute function insert_article_log();

create trigger trg_articles_update_log
after update of slug, title, summary, body_mdx, status, tags, featured, cover_image_url, published_at, author_id
on articles
for each row
execute function insert_article_log();
