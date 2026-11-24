begin;

alter table article_mcp_connections drop constraint article_mcp_known_draft_permissions,
    add constraint article_mcp_known_permissions check (
        draft_permissions <@ array['create_draft','get_article','update_draft','article_status',
            'create_album','list_albums','get_album','update_album','publish_album','unpublish_album',
            'list_album_photos','upload_album_photo','update_album_photo','reorder_album_photos','set_album_cover','delete_album_photo']::text[]
    );

create table mcp_albums (
    album_id uuid primary key references photo_albums(album_id) on delete cascade,
    connection_id uuid not null references article_mcp_connections(id),
    request_key text not null,
    payload_hash bytea not null,
    unique (connection_id, request_key)
);
create table mcp_album_uploads (
    album_id uuid not null references mcp_albums(album_id) on delete cascade,
    request_key text not null,
    payload_hash bytea not null,
    photo_id uuid references album_photos(photo_id) on delete set null,
    primary key(album_id, request_key)
);
alter table article_mcp_operations add column album_id uuid;
commit;
