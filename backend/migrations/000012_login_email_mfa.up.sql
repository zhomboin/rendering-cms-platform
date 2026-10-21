alter table users add column last_login_ip_hash text;

create table login_email_codes (
  code_id uuid primary key default gen_random_uuid(),
  user_id uuid not null references users(user_id),
  email text not null,
  code_hash text not null,
  expires_at timestamptz not null,
  attempts int not null default 0,
  used_at timestamptz,
  created_at timestamptz not null default now()
);

create index login_email_codes_email_created_at_idx on login_email_codes (email, created_at desc);
