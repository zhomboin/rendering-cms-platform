begin;
drop table article_mcp_operations;
alter table article_mcp_connections drop column last_seen_at, drop column client_name;
drop table article_mcp_settings;
commit;
