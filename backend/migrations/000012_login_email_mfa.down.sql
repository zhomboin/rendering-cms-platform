drop table if exists login_email_codes;

alter table users drop column if exists last_login_ip_hash;
