drop trigger if exists trg_articles_set_update_version on articles;
drop trigger if exists trg_articles_update_log on articles;

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

create trigger trg_articles_update_log
after update of slug, article_name, title, summary, body_mdx, status, tags, featured, cover_image_url, published_at, author_id
on articles
for each row
execute function insert_article_log();

alter table article_logs
  drop column if exists featured_at,
  drop column if exists featured_rank;

alter table articles
  drop column if exists featured_at,
  drop column if exists featured_rank;
