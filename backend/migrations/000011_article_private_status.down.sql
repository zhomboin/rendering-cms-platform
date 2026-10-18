-- PostgreSQL 不支持直接删除枚举值，先清理 private 数据再重建 article_status 类型。
-- 必须先处理 article_logs 中的历史快照，否则类型重建时的 cast 会失败。
update article_logs
set status = 'draft'
where status = 'private';

update articles
set status = 'draft'
where status = 'private';

-- status 列出现在触发器的 update of 列清单里，类型重建前必须先删除触发器。
drop trigger trg_articles_set_update_version on articles;
drop trigger trg_articles_insert_log on articles;
drop trigger trg_articles_update_log on articles;

create type article_status_without_private as enum ('draft', 'published', 'archived');

-- articles.status 的默认值 'draft'::article_status 无法自动转换到新类型，先移除再恢复。
alter table articles
  alter column status drop default;

alter table articles
  alter column status type article_status_without_private using status::text::article_status_without_private;

alter table articles
  alter column status set default 'draft'::article_status_without_private;

alter table article_logs
  alter column status type article_status_without_private using status::text::article_status_without_private;

drop type article_status;

alter type article_status_without_private rename to article_status;

-- 触发器定义与 000009_article_featured_metadata 保持一致。
create trigger trg_articles_set_update_version
before update of slug, article_name, title, summary, body_mdx, status, tags, featured, featured_rank, featured_at, cover_image_url, published_at, author_id
on articles
for each row
execute function set_article_update_version();

create trigger trg_articles_insert_log
after insert on articles
for each row
execute function insert_article_log();

create trigger trg_articles_update_log
after update of slug, article_name, title, summary, body_mdx, status, tags, featured, featured_rank, featured_at, cover_image_url, published_at, author_id
on articles
for each row
execute function insert_article_log();
