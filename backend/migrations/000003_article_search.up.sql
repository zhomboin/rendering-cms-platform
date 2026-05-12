alter table articles
  add column search_vector tsvector generated always as (
    setweight(to_tsvector('simple', coalesce(title, '')), 'A') ||
    setweight(to_tsvector('simple', coalesce(summary, '')), 'B') ||
    setweight(to_tsvector('simple', coalesce(body_mdx, '')), 'C')
  ) stored;

create index articles_search_vector_idx on articles using gin (search_vector);
