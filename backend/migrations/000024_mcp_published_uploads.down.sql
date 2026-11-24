begin;
update article_mcp_connections c set permission_revision = permission_revision + 1
where exists (select 1 from mcp_published_upload_grants g where g.connection_id = c.id);
drop table mcp_published_upload_grants;
drop table mcp_delegated_photo_uploads;
commit;
