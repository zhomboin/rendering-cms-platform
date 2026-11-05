create sequence postbridge_state_seq;

create table postbridge_article_states (
  source_id text not null,
  article_id uuid not null,
  source_version bigint not null check (source_version > 0),
  state_seq bigint not null check (state_seq > 0),
  primary key (source_id, article_id)
);

create index idx_postbridge_article_states_article
  on postbridge_article_states (article_id);

create table postbridge_attempts (
  id text primary key,
  source_id text not null,
  article_id uuid not null,
  request_id text not null,
  source_version bigint not null check (source_version > 0),
  state_seq bigint not null check (state_seq > 0),
  target_account_id text not null,
  authorization_id text not null,
  authorization_expires_at timestamptz not null,
  status text not null,
  data jsonb not null,
  revoked_at timestamptz,
  cancel_pending boolean not null default false,
  next_sync_at timestamptz default now(),
  lease_token text not null default '',
  lease_expires_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (source_id, article_id, request_id),
  unique (source_id, authorization_id)
);

create index idx_postbridge_attempts_article
  on postbridge_attempts (article_id);

create index idx_postbridge_attempts_source_created
  on postbridge_attempts (source_id, created_at desc, id desc);

create index idx_postbridge_attempts_due
  on postbridge_attempts (source_id, next_sync_at, id)
  where next_sync_at is not null;

create table postbridge_events (
  id text primary key,
  source_id text not null,
  article_id uuid not null,
  state_seq bigint not null,
  event_type text not null,
  body bytea not null,
  snapshot bytea,
  attempts integer not null default 0,
  last_error_code text not null default '',
  next_attempt_at timestamptz not null default now(),
  delivered_at timestamptz,
  lease_token text not null default '',
  lease_expires_at timestamptz,
  created_at timestamptz not null default now()
);

create index idx_postbridge_events_due
  on postbridge_events (source_id, next_attempt_at, id)
  where delivered_at is null;

create index idx_postbridge_events_pending_article
  on postbridge_events (source_id, article_id, state_seq)
  where delivered_at is null and event_type <> 'authorization_revoked';

create table postbridge_connections (
  source_id text primary key check (source_id <> ''),
  environment text not null check (environment <> ''),
  origin text not null check (origin <> ''),
  client_id text not null check (client_id <> '')
);

create unique index idx_postbridge_connections_singleton
  on postbridge_connections ((true));

create table postbridge_registrations (
  source_id text not null,
  environment text not null,
  registration_id text not null,
  status text not null,
  updated_at timestamptz not null default now(),
  primary key (source_id, environment)
);

create function postbridge_enqueue_event(
  event_source_id text,
  event_article_id uuid,
  event_source_version bigint,
  event_type text,
  event_status text,
  event_authorization_id text,
  event_snapshot bytea
) returns void language plpgsql as $$
declare
  event_id text := gen_random_uuid()::text;
  event_seq bigint := nextval('postbridge_state_seq');
  payload jsonb;
begin
  payload := jsonb_build_object(
    'event_id', event_id,
    'source_id', event_source_id,
    'article_id', event_article_id::text,
    'state_seq', event_seq,
    'source_version', event_source_version,
    'type', event_type,
    'status', event_status,
    'authorization_valid', false
  );
  if event_authorization_id is not null then
    payload := payload || jsonb_build_object('authorization_id', event_authorization_id);
  end if;

  insert into postbridge_events (id, source_id, article_id, state_seq, event_type, body, snapshot)
  values (
    event_id, event_source_id, event_article_id, event_seq, event_type,
    convert_to(payload::text, 'UTF8'), event_snapshot
  );
end;
$$;

create function postbridge_revoke_article_grants() returns trigger language plpgsql as $$
declare
  changed_article_id uuid;
  changed_version bigint;
  changed_status text;
  changed_type text;
  changed_snapshot bytea;
  attempt record;
  tracked record;
begin
  if not exists (select 1 from postbridge_article_states where article_id = old.article_id) then
    return null;
  end if;

  if tg_op = 'DELETE' then
    changed_article_id := old.article_id;
    changed_version := old.version;
    changed_status := 'deleted';
    changed_type := 'deleted';
  else
    if new.version = old.version and new.status = old.status then
      return null;
    end if;

    changed_article_id := new.article_id;
    changed_version := new.version;
    changed_status := case when new.status = 'archived' then 'draft' else new.status::text end;
    if new.status = 'private' then
      changed_type := 'private';
    elsif new.status = 'archived' or (old.status = 'published' and new.status = 'draft') then
      changed_type := 'unpublished';
    else
      changed_type := 'updated';
      changed_snapshot := convert_to(jsonb_build_object(
        'ID', new.article_id::text,
        'Version', new.version,
        'Status', changed_status,
        'Title', new.title,
        'Summary', new.summary,
        'Body', new.body_mdx,
        'Tags', new.tags,
        'CoverImageURL', coalesce(new.cover_image_url, ''),
        'ArticleName', new.article_name
      )::text, 'UTF8');
    end if;
  end if;

  -- Preserve scoped revocations even if a newer source event is delivered first.
  for attempt in
    update postbridge_attempts
    set revoked_at = clock_timestamp(),
        updated_at = clock_timestamp(),
        cancel_pending = true,
        next_sync_at = clock_timestamp()
    where article_id = changed_article_id and revoked_at is null
    returning source_id, authorization_id
  loop
    perform postbridge_enqueue_event(
      attempt.source_id, changed_article_id, changed_version,
      'authorization_revoked', changed_status, attempt.authorization_id, null
    );
  end loop;

  for tracked in select source_id from postbridge_article_states where article_id = changed_article_id
  loop
    perform postbridge_enqueue_event(
      tracked.source_id, changed_article_id, changed_version,
      changed_type, changed_status, null, changed_snapshot
    );
  end loop;

  return null;
end;
$$;

create trigger trg_postbridge_revoke_article_grants
  after update or delete on articles
  for each row execute function postbridge_revoke_article_grants();
