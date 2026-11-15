begin;

-- Keep this rule aligned with the article editor's normalizeArticleTag.
create function normalize_article_tag(tag text) returns text
language plpgsql immutable strict as $$
begin
  tag := regexp_replace(normalize(tag, NFKD) collate pg_catalog."C",
    U&'([A-Za-z])[\0300-\036f]+', '\1', 'g');
  tag := regexp_replace(normalize(tag, NFC) collate pg_catalog."C",
    '([A-Z])([A-Z][a-z])', '\1-\2', 'g');
  tag := regexp_replace(tag collate pg_catalog."C", '([a-z0-9])([A-Z])', '\1-\2', 'g');
  tag := btrim(regexp_replace(lower(tag collate pg_catalog."und-x-icu"),
    U&'[[:punct:][:space:]\0001-\002f\003a-\0040\005b-\0060\007b-\007f]+', '-', 'g'), '-');
  return case when tag collate pg_catalog."und-x-icu" ~ '[[:alnum:]]' then tag else '' end;
end;
$$;

create function normalize_article_tags(tags text[]) returns text[]
language sql immutable as $$
  select array(
    select tag
    from (
      select normalize_article_tag(value) collate pg_catalog."C" as tag, position
      from unnest(tags) with ordinality as input(value, position)
    ) normalized
    where tag <> ''
    group by tag
    order by min(position)
  );
$$;

create function set_normalized_article_tags() returns trigger
language plpgsql as $$
begin
  new.tags := normalize_article_tags(new.tags);
  return new;
end;
$$;

create trigger trg_articles_normalize_tags
before insert or update of tags on articles
for each row execute function set_normalized_article_tags();

-- Run the article revision and source-event triggers for each affected article.
update articles
set tags = normalize_article_tags(tags)
where tags is distinct from normalize_article_tags(tags);

update article_logs
set tags = normalize_article_tags(tags)
where tags is distinct from normalize_article_tags(tags);

commit;
