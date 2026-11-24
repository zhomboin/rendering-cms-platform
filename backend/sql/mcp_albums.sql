-- name: BindMCPAlbum :exec
insert into mcp_albums (album_id, connection_id, request_key, payload_hash) values ($1,$2,$3,$4);

-- name: FindMCPAlbumRequest :one
select album_id,payload_hash from mcp_albums where connection_id=$1 and request_key=$2;

-- name: GetAccessibleMCPAlbum :one
select a.album_id,a.status from photo_albums a left join mcp_albums m using (album_id)
where (sqlc.arg(read_all)::boolean or m.connection_id=sqlc.arg(connection_id))
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
select photo_id,payload_hash from mcp_album_uploads where album_id=$1 and request_key=$2;

-- name: BindMCPUpload :exec
insert into mcp_album_uploads (album_id,request_key,payload_hash,photo_id) values ($1,$2,$3,$4);
