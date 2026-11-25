begin;

-- 建稿请求按连接去重。文章删除后保留请求键，防止迟到的重试重新建稿。
alter table article_mcp_drafts
  drop constraint article_mcp_drafts_article_id_fkey,
  drop constraint article_mcp_drafts_pkey,
  drop constraint article_mcp_drafts_connection_id_request_key_key;
alter table article_mcp_drafts
  alter column article_id drop not null,
  add primary key (connection_id, request_key),
  add unique (article_id),
  add foreign key (article_id) references articles(article_id) on delete set null;

commit;
