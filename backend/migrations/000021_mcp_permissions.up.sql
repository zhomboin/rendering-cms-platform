begin;

alter table article_mcp_connections
    add column draft_permissions text[] not null
        default array['create_draft','get_article','update_draft','article_status']::text[],
    add column permission_revision integer not null default 1,
    add constraint article_mcp_known_draft_permissions check (
        draft_permissions <@ array['create_draft','get_article','update_draft','article_status']::text[]
    );

commit;
