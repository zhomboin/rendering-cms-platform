begin;

create table article_mcp_connections (
    id uuid primary key default gen_random_uuid(),
    owner_id uuid not null references users(user_id),
    label text not null,
    token_hash bytea unique,
    oauth_issuer text,
    oauth_client text,
    oauth_subject text,
    allow_publish boolean not null default false,
    revoked boolean not null default false,
    created_at timestamptz not null default now(),
    check ((token_hash is not null and num_nonnulls(oauth_issuer, oauth_client, oauth_subject) = 0)
        or (token_hash is null and num_nonnulls(oauth_issuer, oauth_client, oauth_subject) = 3))
);
create unique index article_mcp_active_oauth on article_mcp_connections(oauth_issuer, oauth_client, oauth_subject)
    where not revoked;

create table article_mcp_drafts (
    article_id uuid primary key references articles(article_id) on delete cascade,
    connection_id uuid not null references article_mcp_connections(id),
    request_key text not null,
    payload_hash bytea not null,
    search_version integer,
    search_result text check (search_result in ('found','not_found')),
    search_checked_at timestamptz,
    detached boolean not null default false,
    unique(connection_id, request_key)
);

-- 转私有时同时收回 AI 的访问权限，包括以后恢复到草稿的情形。
create function detach_private_mcp_draft() returns trigger language plpgsql as $$
begin
    update article_mcp_drafts set detached = true where article_id = new.article_id and not detached;
    return new;
end;
$$;
create trigger trg_detach_private_mcp_draft after update of status on articles
    for each row when (new.status = 'private') execute function detach_private_mcp_draft();

commit;
