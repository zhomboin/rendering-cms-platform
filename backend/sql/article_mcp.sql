-- name: CreateMCPConnection :one
insert into article_mcp_connections(owner_id,label,token_hash,oauth_issuer,oauth_client,oauth_subject,allow_publish,draft_permissions)
values ($1,$2,$3,$4,$5,$6,$7,$8) returning id;

-- name: ListMCPConnections :many
select id,label,allow_publish,draft_permissions,permission_revision,revoked,created_at,client_name,last_seen_at,(token_hash is null)::boolean as oauth from article_mcp_connections
where owner_id=$1 order by created_at desc;

-- name: ChangeMCPPublish :execrows
update article_mcp_connections set allow_publish=$3,permission_revision=permission_revision+1 where owner_id=$1 and id=$2 and not revoked;

-- name: ChangeMCPPermissions :execrows
update article_mcp_connections
set draft_permissions=$3,allow_publish=$4,permission_revision=permission_revision+1
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
select owner_id,allow_publish,draft_permissions from article_mcp_connections where id=$1 and not revoked for update;

-- name: FindMCPRequest :one
select article_id,payload_hash from article_mcp_drafts where connection_id=$1 and request_key=$2;

-- name: BindMCPDraft :exec
insert into article_mcp_drafts(article_id,connection_id,request_key,payload_hash) values ($1,$2,$3,$4);

-- name: GetMCPArticle :one
select a.article_id,a.slug,a.article_name,a.title,a.summary,a.body_mdx,a.tags,a.version,a.status,d.search_version,d.search_result,d.search_checked_at
from articles a join article_mcp_drafts d using(article_id)
where d.connection_id=$1 and a.article_id=$2 and not d.detached and a.status in ('draft','published')
for update of a;

-- name: EditMCPDraft :execrows
update articles set title=$3,summary=$4,body_mdx=$5,tags=$6,updated_at=now()
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
