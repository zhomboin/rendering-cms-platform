drop trigger trg_postbridge_revoke_article_grants on articles;
drop function postbridge_revoke_article_grants();
drop function postbridge_enqueue_event(text, uuid, bigint, text, text, text, bytea);
drop table postbridge_registrations;
drop table postbridge_connections;
drop table postbridge_events;
drop table postbridge_attempts;
drop table postbridge_article_states;
drop sequence postbridge_state_seq;
