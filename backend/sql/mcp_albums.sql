-- name: BindMCPAlbum :exec
insert into mcp_albums (album_id, connection_id, request_key, payload_hash) values ($1,$2,$3,$4);

-- name: FindMCPAlbumRequest :one
select album_id,payload_hash from mcp_albums where connection_id=$1 and request_key=$2;

-- name: GetAccessibleMCPAlbum :one
select a.album_id, a.status, coalesce(m.connection_id=sqlc.arg(connection_id),false)::boolean as owned,
       (g.album_id is not null)::boolean as can_append
from photo_albums a left join mcp_albums m using (album_id)
left join mcp_published_upload_grants g on g.album_id=a.album_id and g.connection_id=sqlc.arg(connection_id)
where (sqlc.arg(read_all)::boolean or m.connection_id=sqlc.arg(connection_id)
    or (sqlc.arg(for_upload)::boolean and a.status='published' and g.album_id is not null))
    and a.album_id=sqlc.arg(album_id) for update of a;

-- name: ListMCPAlbums :many
select a.album_id,a.slug,a.title,a.status from photo_albums a left join mcp_albums m using (album_id)
where (sqlc.arg(read_all)::boolean or m.connection_id=sqlc.arg(connection_id))
order by a.created_at desc,a.album_id limit sqlc.arg('limit') offset sqlc.arg('offset');

-- name: ListMCPPhotos :many
select p.photo_id,p.alt,p.caption,p.taken_at,p.position,p.width,p.height,a.public_url
from album_photos p join assets a using(asset_id)
where p.album_id=$1 order by p.position,p.photo_id limit $2 offset $3;

-- name: FindMCPUpload :one
select u.photo_id,u.payload_hash from mcp_album_uploads u join mcp_albums m using(album_id)
where u.album_id=sqlc.arg(album_id) and u.request_key=sqlc.arg(request_key) and m.connection_id=sqlc.arg(connection_id)
union all
select photo_id,payload_hash from mcp_delegated_photo_uploads
where album_id=sqlc.arg(album_id) and request_key=sqlc.arg(request_key) and connection_id=sqlc.arg(connection_id);

-- name: BindMCPUpload :exec
insert into mcp_album_uploads (album_id,request_key,payload_hash,photo_id) values ($1,$2,$3,$4);

-- name: BindMCPDelegatedUpload :exec
insert into mcp_delegated_photo_uploads (album_id, request_key, payload_hash, photo_id, connection_id)
values ($1,$2,$3,$4,$5);

-- name: RemoveMCPUploadGrantsExcept :exec
delete from mcp_published_upload_grants where connection_id=$1 and not (album_id=any(sqlc.arg(album_ids)::uuid[]));

-- name: AddMCPUploadGrants :exec
insert into mcp_published_upload_grants(connection_id, album_id)
select sqlc.arg(connection_id), a.album_id from photo_albums a
where a.status='published' and a.album_id=any(sqlc.arg(album_ids)::uuid[])
on conflict do nothing;

-- name: CountMCPUploadGrants :one
select count(*) from mcp_published_upload_grants where connection_id=$1;
