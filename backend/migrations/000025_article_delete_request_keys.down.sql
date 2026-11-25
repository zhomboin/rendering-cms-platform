begin;

alter table article_mcp_drafts
  drop constraint article_mcp_drafts_article_id_fkey,
  drop constraint article_mcp_drafts_article_id_key,
  drop constraint article_mcp_drafts_pkey;
delete from article_mcp_drafts where article_id is null;
alter table article_mcp_drafts
  add primary key (article_id),
  add unique (connection_id, request_key),
  add foreign key (article_id) references articles(article_id) on delete cascade;

commit;
