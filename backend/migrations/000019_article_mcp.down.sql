begin;
drop trigger trg_detach_private_mcp_draft on articles;
drop function detach_private_mcp_draft();
drop table article_mcp_drafts;
drop table article_mcp_connections;
commit;
