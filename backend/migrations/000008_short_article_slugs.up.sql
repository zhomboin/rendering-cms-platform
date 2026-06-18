create function pg_temp.base62_6(value bigint)
returns text
language plpgsql
as $$
declare
  alphabet constant text := '0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ';
  base constant bigint := 62;
  current_value bigint := value;
  result text := '';
  index integer;
begin
  for index in 1..6 loop
    result := substr(alphabet, (current_value % base)::integer + 1, 1) || result;
    current_value := current_value / base;
  end loop;
  return result;
end;
$$;

create temporary table article_short_slug_mapping (
  article_id uuid primary key,
  short_slug text not null unique
);

alter table articles
  add column article_name text;

update articles
set article_name = slug
where article_name is null;

alter table articles
  alter column article_name set not null;

alter table articles
  add constraint articles_article_name_key unique (article_name);

alter table article_logs
  add column article_name text;

update article_logs
set article_name = slug
where article_name is null;

alter table article_logs
  alter column article_name set not null;

insert into article_short_slug_mapping (article_id, short_slug)
select
  article_id,
  pg_temp.base62_6(
    (
      (('x' || substr(md5(slug), 1, 12))::bit(48)::bigint % 56800235584)
      + row_number() over (
          partition by pg_temp.base62_6((('x' || substr(md5(slug), 1, 12))::bit(48)::bigint % 56800235584))
          order by created_at asc, article_id asc
        )
      - 1
    ) % 56800235584
  ) as short_slug
from articles;

drop trigger if exists trg_articles_set_update_version on articles;
drop trigger if exists trg_articles_update_log on articles;
drop trigger if exists trg_articles_insert_log on articles;

update article_logs logs
set slug = mapping.short_slug
from article_short_slug_mapping mapping
where logs.article_id = mapping.article_id;

update articles articles
set slug = mapping.short_slug
from article_short_slug_mapping mapping
where articles.article_id = mapping.article_id;

alter table articles
  add constraint articles_slug_short_code_check check (slug ~ '^[0-9A-Za-z]{6}$');

alter table articles
  add constraint articles_article_name_check check (article_name ~ '^[a-z0-9]+(-[a-z0-9]+)*$');

alter table article_logs
  add constraint article_logs_slug_short_code_check check (slug ~ '^[0-9A-Za-z]{6}$');

alter table article_logs
  add constraint article_logs_article_name_check check (article_name ~ '^[a-z0-9]+(-[a-z0-9]+)*$');

create or replace function insert_article_log()
returns trigger
language plpgsql
as $$
begin
  insert into article_logs (
    article_id,
    slug,
    article_name,
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
    new.article_name,
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
    article_name = excluded.article_name,
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
before update of slug, article_name, title, summary, body_mdx, status, tags, featured, cover_image_url, published_at, author_id
on articles
for each row
execute function set_article_update_version();

create trigger trg_articles_insert_log
after insert on articles
for each row
execute function insert_article_log();

create trigger trg_articles_update_log
after update of slug, article_name, title, summary, body_mdx, status, tags, featured, cover_image_url, published_at, author_id
on articles
for each row
execute function insert_article_log();
