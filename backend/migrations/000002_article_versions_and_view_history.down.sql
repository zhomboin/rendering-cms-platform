drop trigger if exists trg_articles_update_log on articles;
drop trigger if exists trg_articles_insert_log on articles;
drop trigger if exists trg_articles_set_update_version on articles;
drop function if exists insert_article_log();
drop function if exists set_article_update_version();

drop table if exists site_view_history;
drop table if exists article_view_history;

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

create index idx_article_revisions_article_id on article_revisions (article_id);

insert into article_revisions (
  article_id,
  title,
  summary,
  body_mdx,
  status,
  created_by,
  created_at
)
select
  article_id,
  title,
  summary,
  body_mdx,
  status,
  author_id,
  created_at
from article_logs
order by article_id, version;

drop table if exists article_logs;

alter table articles
  drop column if exists version;
