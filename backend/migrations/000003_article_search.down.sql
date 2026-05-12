drop index if exists articles_search_vector_idx;
alter table articles drop column if exists search_vector;
