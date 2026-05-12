create table login_attempts (
  attempt_id uuid primary key default gen_random_uuid(),
  email text not null,
  ip_hash text not null,
  success boolean not null,
  failure_reason text,
  created_at timestamptz not null default now()
);

create index login_attempts_email_created_at_idx on login_attempts (email, created_at desc);
create index login_attempts_ip_hash_created_at_idx on login_attempts (ip_hash, created_at desc);
