begin;

-- Normalized data stays in place. Restore a pre-migration backup for the original values.
drop trigger trg_articles_normalize_tags on articles;
drop function set_normalized_article_tags();
drop function normalize_article_tags(text[]);
drop function normalize_article_tag(text);

commit;
