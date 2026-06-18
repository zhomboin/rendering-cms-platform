alter table article_logs
  drop constraint if exists article_logs_slug_short_code_check;

alter table articles
  drop constraint if exists articles_slug_short_code_check;

alter table article_logs
  drop constraint if exists article_logs_article_name_check;

alter table articles
  drop constraint if exists articles_article_name_check;

alter table articles
  drop constraint if exists articles_article_name_key;

drop trigger if exists trg_articles_set_update_version on articles;
drop trigger if exists trg_articles_update_log on articles;
drop trigger if exists trg_articles_insert_log on articles;

alter table article_logs
  drop column if exists article_name;

alter table articles
  drop column if exists article_name;

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
