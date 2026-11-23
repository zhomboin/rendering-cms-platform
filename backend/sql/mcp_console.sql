-- name: GetMCPSettings :one
select revision, enabled, origin, admin_origin, blog_origin from article_mcp_settings where singleton;

-- name: SaveMCPSettings :execrows
update article_mcp_settings set revision=revision+1, enabled=$1, origin=$2, admin_origin=$3, blog_origin=$4
where singleton and revision=$5;

-- name: ObserveMCPClient :exec
update article_mcp_connections set last_seen_at=clock_timestamp(),
    client_name=coalesce(nullif(sqlc.arg(client_name)::text,''),client_name)
where id=$1 and not revoked;

-- name: StartMCPOperation :one
insert into article_mcp_operations(connection_id,operation) values ($1,$2) returning id;

-- name: FinishMCPOperation :exec
update article_mcp_operations set outcome=$2,article_id=$3 where id=$1;

-- name: ListMCPOperations :many
select o.id,c.label,o.operation,o.outcome,o.created_at,o.article_id
from article_mcp_operations o join article_mcp_connections c on c.id=o.connection_id
where c.owner_id=$1 and (sqlc.arg(before_id)::bigint=0 or o.id<sqlc.arg(before_id))
order by o.id desc limit 51;
