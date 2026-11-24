begin;
create table mcp_published_upload_grants (
    connection_id uuid not null references article_mcp_connections(id) on delete cascade,
    album_id uuid not null references photo_albums(album_id) on delete cascade,
    primary key(connection_id, album_id)
);

-- 跨连接追加的回执独立保留，重新授权后仍可识别重试。
create table mcp_delegated_photo_uploads (
    connection_id uuid not null references article_mcp_connections(id) on delete cascade,
    album_id uuid not null references photo_albums(album_id) on delete cascade,
    request_key text not null,
    payload_hash bytea not null,
    photo_id uuid references album_photos(photo_id) on delete set null,
    primary key(connection_id, album_id, request_key)
);
commit;
