begin;

create table article_mcp_settings (
    singleton boolean primary key default true check (singleton),
    revision integer not null default 0,
    enabled boolean not null default false,
    origin text not null default '',
    admin_origin text not null default '',
    blog_origin text not null default ''
);
insert into article_mcp_settings default values;

alter table article_mcp_connections
    add column client_name text not null default '',
    add column last_seen_at timestamptz;

create table article_mcp_operations (
    id bigint generated always as identity primary key,
    connection_id uuid not null references article_mcp_connections(id),
    operation text not null,
    article_id uuid,
    outcome text not null default 'pending' check (outcome in ('pending','success','failed')),
    created_at timestamptz not null default now()
);
create index article_mcp_operations_connection on article_mcp_operations(connection_id, id desc);

commit;
