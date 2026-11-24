begin;
update article_mcp_connections set permission_revision = permission_revision + 1 where read_all;
alter table article_mcp_connections drop column read_all;
commit;
