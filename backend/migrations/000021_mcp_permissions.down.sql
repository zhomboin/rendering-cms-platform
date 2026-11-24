begin;

-- 回退后只能表示完整的草稿权限。撤销受限连接，避免回退扩大授权。
update article_mcp_connections set revoked=true
where not (draft_permissions @> array['create_draft','get_article','update_draft','article_status']::text[]);

alter table article_mcp_connections
    drop column draft_permissions,
    drop column permission_revision;

commit;
