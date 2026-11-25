-- name: CreateMCPConnection :one
insert into article_mcp_connections(owner_id,label,token_hash,oauth_issuer,oauth_client,oauth_subject,allow_publish,draft_permissions,read_all)
values ($1,$2,$3,$4,$5,$6,$7,$8,$9) returning id;

-- name: ListMCPConnections :many
select id,label,allow_publish,draft_permissions,read_all,permission_revision,revoked,created_at,client_name,last_seen_at,(token_hash is null)::boolean as oauth,
       array(select g.album_id::text from mcp_published_upload_grants g where g.connection_id=c.id order by g.album_id)::text[] as published_upload_album_ids
from article_mcp_connections c
where owner_id=$1 order by created_at desc;

-- name: ChangeMCPPublish :execrows
update article_mcp_connections set allow_publish=$3,permission_revision=permission_revision+1 where owner_id=$1 and id=$2 and not revoked;

-- name: ChangeMCPPermissions :execrows
update article_mcp_connections
set draft_permissions=coalesce(sqlc.narg(draft_permissions)::text[],draft_permissions),
    allow_publish=coalesce(sqlc.narg(allow_publish)::boolean,allow_publish),
    read_all=coalesce(sqlc.narg(read_all)::boolean,read_all),
    permission_revision=permission_revision+1
where owner_id=$1 and id=$2 and not revoked
    and (sqlc.arg(expected_revision)::integer=0 or permission_revision=sqlc.arg(expected_revision));

-- name: RevokeMCPConnection :execrows
update article_mcp_connections set revoked=true where owner_id=$1 and id=$2;

-- name: LookupMCPToken :one
select id,allow_publish,draft_permissions from article_mcp_connections where token_hash=$1 and not revoked;

-- name: LookupMCPOAuth :one
select id,allow_publish,draft_permissions from article_mcp_connections
where oauth_issuer=$1 and oauth_client=$2 and oauth_subject=$3 and not revoked;

-- name: LockMCPConnection :one
select owner_id,allow_publish,draft_permissions,read_all from article_mcp_connections where id=$1 and not revoked for update;

-- name: FindMCPRequest :one
select article_id,payload_hash from article_mcp_drafts where connection_id=$1 and request_key=$2;

-- name: BindMCPDraft :exec
insert into article_mcp_drafts(article_id,connection_id,request_key,payload_hash) values ($1,$2,$3,$4);

-- name: GetMCPArticle :one
select a.article_id,a.slug,a.article_name,a.title,a.summary,a.body_mdx,a.tags,a.version,a.status,
       a.cover_image_url,a.featured,a.featured_rank,a.featured_at,d.search_version,d.search_result,d.search_checked_at
from articles a left join article_mcp_drafts d using(article_id)
where a.article_id=sqlc.arg(article_id)
  and not coalesce(d.detached and d.connection_id=sqlc.arg(connection_id),false)
  and ((d.connection_id=sqlc.arg(connection_id) and a.status in ('draft','published'))
       or (sqlc.arg(read_all)::boolean and a.status in ('draft','published','archived')))
for update of a;

-- name: EditMCPDraft :execrows
update articles set title=$3,summary=$4,body_mdx=$5,tags=$6,
    cover_image_url=$7,featured=$8,featured_rank=$9,featured_at=$10,updated_at=now()
where article_id=$1 and version=$2 and status='draft';

-- name: MCPVerificationTarget :one
select a.slug,a.title,a.version from articles a
join article_mcp_drafts d using(article_id)
join article_mcp_connections c on c.id=d.connection_id
where c.owner_id=$1 and a.article_id=$2 and not d.detached and a.status='published';

-- name: RecordMCPSearch :execrows
update article_mcp_drafts d
set search_version=a.version,search_result=$4,search_checked_at=now()
from articles a,article_mcp_connections c
where d.article_id=a.article_id and c.id=d.connection_id and c.owner_id=$1
  and a.article_id=$2 and a.version=$3 and a.status='published' and not d.detached;
