begin;
update article_mcp_connections c set draft_permissions = array(
    select name from unnest(c.draft_permissions) as name
    where name = any(array['create_draft','get_article','update_draft','article_status'])
), permission_revision = permission_revision + 1
where not (draft_permissions <@ array['create_draft','get_article','update_draft','article_status']::text[]);
alter table article_mcp_connections drop constraint article_mcp_known_permissions,
    add constraint article_mcp_known_draft_permissions check (
        draft_permissions <@ array['create_draft','get_article','update_draft','article_status']::text[]
    );
alter table article_mcp_operations drop column album_id;
drop table mcp_album_uploads;
drop table mcp_albums;
commit;
